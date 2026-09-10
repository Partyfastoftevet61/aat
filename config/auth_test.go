package config

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticate_VerboseRedactsTokens(t *testing.T) {
	const accessToken = "shop-0001-deadbeefcafebabe"
	const refreshToken = "refresh-0123456789abcdef"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	var log bytes.Buffer
	token, err := Authenticate(WithAuthVerbose(context.Background(), &log), AuthConfig{
		Type:     "oauth2",
		TokenURL: server.URL,
		Credentials: map[string]SecretRef{
			"username":     {Source: "literal", Value: "demo"},
			"password":     {Source: "literal", Value: "demo-password"},
			"clientId":     {Source: "literal", Value: "client"},
			"clientSecret": {Source: "literal", Value: "client-secret"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, accessToken, token.AccessToken, "the returned token is not redacted")

	out := log.String()
	assert.NotContains(t, out, accessToken)
	assert.NotContains(t, out, refreshToken)
	assert.Contains(t, out, "shop-000...")
	assert.Contains(t, out, "token_type")
}

func TestRedactTokenResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "not JSON", body: "upstream error", want: "upstream error"},
		{name: "no credential fields", body: `{"error":"invalid_grant"}`, want: `{"error":"invalid_grant"}`},
		{name: "short token shows at most half", body: `{"access_token":"abcd"}`, want: `{"access_token":"ab..."}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, redactTokenResponse([]byte(tt.body)))
		})
	}
}
