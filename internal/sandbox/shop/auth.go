package shop

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Demo credentials. They are printed by the startup banner and used verbatim
// by examples/shop/env.yaml.
const (
	DemoUsername     = "demo"
	DemoPassword     = "demo"
	DemoClientID     = "aat-shop"
	DemoClientSecret = "aat-shop-secret"
	DemoAPIKey       = "pay-demo-key"
	APIKeyHeader     = "X-API-Key"
	tokenTTLSeconds  = 3600
)

// anonymousToken is the token identity used when auth is disabled.
const anonymousToken = "anonymous"

type tokenStore struct {
	mu     sync.Mutex
	tokens map[string]time.Time // token -> expiry
	seq    int
	rng    *rand.Rand
	now    func() time.Time
}

func newTokenStore(seed int64, now func() time.Time) *tokenStore {
	return &tokenStore{
		tokens: map[string]time.Time{},
		rng:    rand.New(rand.NewSource(seed)), //nolint:gosec // demo tokens, not security
		now:    now,
	}
}

func (t *tokenStore) issue() (string, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	tok := fmt.Sprintf("shop-%04d-%08x", t.seq, t.rng.Uint32())
	t.tokens[tok] = t.now().Add(tokenTTLSeconds * time.Second)
	return tok, tokenTTLSeconds
}

func (t *tokenStore) valid(tok string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	exp, ok := t.tokens[tok]
	return ok && t.now().Before(exp)
}

type tokenKey struct{}

func tokenFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(tokenKey{}).(string); ok {
		return v
	}
	return anonymousToken
}

type oauthError struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

func writeOAuthError(w http.ResponseWriter, status int, code, desc string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, oauthError{Error: code, Description: desc})
}

// handleToken implements POST /oauth/token for the password and
// client_credentials grants (RFC 6749 form encoding; client credentials may
// also arrive as HTTP Basic auth).
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "malformed form body")
		return
	}
	clientID, clientSecret := r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	if id, secret, ok := r.BasicAuth(); ok {
		clientID, clientSecret = id, secret
	}
	if clientID != DemoClientID || clientSecret != DemoClientSecret {
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client",
			fmt.Sprintf("unknown client; use client_id=%s client_secret=%s", DemoClientID, DemoClientSecret))
		return
	}
	switch grant := r.PostForm.Get("grant_type"); grant {
	case "password":
		if r.PostForm.Get("username") != DemoUsername || r.PostForm.Get("password") != DemoPassword {
			writeOAuthError(w, http.StatusBadRequest, "invalid_grant",
				fmt.Sprintf("bad username or password; use %s / %s", DemoUsername, DemoPassword))
			return
		}
	case "client_credentials":
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type",
			fmt.Sprintf("grant_type %q is not supported; use password or client_credentials", grant))
		return
	}
	tok, ttl := s.tokens.issue()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": tok,
		"token_type":   "Bearer",
		"expires_in":   ttl,
		"scope":        "shop",
	})
}

// requireBearer enforces "Authorization: Bearer <token>" under /{region}/v1
// and stores the token identity in the request context (chaos counters are
// keyed by it).
func (s *Server) requireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := anonymousToken
		if !s.opts.NoAuth {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") || !s.tokens.valid(strings.TrimPrefix(auth, "Bearer ")) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="shop"`)
				writeError(w, newError(http.StatusUnauthorized, CodeUnauthorized,
					"missing or invalid bearer token; obtain one from POST /oauth/token"))
				return
			}
			tok = strings.TrimPrefix(auth, "Bearer ")
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), tokenKey{}, tok)))
	})
}

// requireAPIKey enforces the payments listener's X-API-Key header.
func (s *Server) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.opts.NoAuth && r.Header.Get(APIKeyHeader) != DemoAPIKey {
			writeError(w, newError(http.StatusUnauthorized, CodeUnauthorized,
				"missing or invalid %s header; the payments API expects %s", APIKeyHeader, DemoAPIKey))
			return
		}
		next.ServeHTTP(w, r)
	})
}
