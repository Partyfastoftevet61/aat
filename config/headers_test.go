package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteHeaders_Protected checks which headers templates cannot replace: the
// credential and overlay headers on the default route, and on a routed node
// also the override's own headers, with overlay headers applied last on both.
func TestRouteHeaders_Protected(t *testing.T) {
	env := &Environment{
		APIBaseURL: "https://api.example.com",
		Headers:    map[string]string{"Accept": "application/json", "authorization": "static"},
		Auth: AuthConfig{
			Type:        "bearer",
			Credentials: map[string]SecretRef{"token": {Source: "literal", Value: "main"}},
		},
		Overrides: []HostOverride{
			{Match: "inherits", BaseURL: "http://inherit.local", Headers: map[string]string{"X-Route": "inherit"}},
			{
				Match:   "payments",
				BaseURL: "http://pay.local",
				Auth: &AuthConfig{
					Type:        "apikey",
					HeaderName:  "X-API-Key",
					Credentials: map[string]SecretRef{"key": {Source: "literal", Value: "pay-key"}},
				},
			},
		},
	}

	base := env.BuildAPIConfigFromToken(&OAuthToken{AccessToken: "main"}, env.Auth, map[string]string{"X-Plan": "plan"})
	assert.Equal(t, map[string]string{"Authorization": "Bearer main"}, base.Protected)
	assert.Equal(t, "Bearer main", base.Headers["Authorization"], "the credential replaces a static header of the same name")
	assert.NotContains(t, base.Headers, "authorization")

	base.AddOverlayHeaders(map[string]string{"X-Access-Group": "blue", "AUTHORIZATION": "Bearer local-stub"})
	assert.Equal(t, map[string]string{"X-Access-Group": "blue", "AUTHORIZATION": "Bearer local-stub"}, base.Protected, "an overlay header replaces the credential")
	assert.Equal(t, base.Protected, base.Overlay)

	resolved, err := env.BuildOverrideConfigs(context.Background(), base)
	require.NoError(t, err)
	require.Len(t, resolved, 2)

	inherit := resolved[0].APIConfig
	assert.Equal(t, "http://inherit.local", inherit.BaseURL)
	assert.Equal(t, "plan", inherit.Headers["X-Plan"])
	assert.Equal(t, map[string]string{"X-Route": "inherit", "X-Access-Group": "blue", "AUTHORIZATION": "Bearer local-stub"}, inherit.Protected,
		"overlay headers apply after the inherited credential")

	payments := resolved[1].APIConfig
	assert.Equal(t, "pay-key", payments.Headers["X-API-Key"])
	assert.Equal(t, "Bearer local-stub", payments.Headers["AUTHORIZATION"], "the overlay header still reaches the routed node")
	assert.Equal(t, map[string]string{"X-API-Key": "pay-key", "X-Access-Group": "blue", "AUTHORIZATION": "Bearer local-stub"}, payments.Protected)
}

// TestCredentialHeader_ValuePrefix checks that an apikey's valuePrefix comes
// before the key, so an API with an authentication scheme of its own is written
// without the scheme becoming part of the secret. Shippo's own OpenAPI spec
// declares this shape as x-token-format: "ShippoToken {token}".
func TestCredentialHeader_ValuePrefix(t *testing.T) {
	env := &Environment{
		APIBaseURL: "https://api.goshippo.com",
		Auth: AuthConfig{
			Type:        "apikey",
			HeaderName:  "Authorization",
			ValuePrefix: "ShippoToken ",
			Credentials: map[string]SecretRef{"key": {Source: "literal", Value: "shippo_test_abc123"}},
		},
	}

	cfg := env.BuildAPIConfigFromToken(&OAuthToken{AccessToken: "shippo_test_abc123"}, env.Auth, nil)
	assert.Equal(t, "ShippoToken shippo_test_abc123", cfg.Headers["Authorization"])
	assert.Equal(t, map[string]string{"Authorization": "ShippoToken shippo_test_abc123"}, cfg.Protected,
		"the prefixed credential is still protected from template headers")
}

// TestCredentialHeader_NoValuePrefix checks that the header is unchanged when no
// prefix is set, and that bearer keeps writing its own "Bearer ".
func TestCredentialHeader_NoValuePrefix(t *testing.T) {
	apikey := AuthConfig{Type: "apikey", HeaderName: "X-API-Key"}
	name, value, ok := credentialHeader(apikey, &OAuthToken{AccessToken: "plain"})
	assert.True(t, ok)
	assert.Equal(t, "X-API-Key", name)
	assert.Equal(t, "plain", value)

	bearer := AuthConfig{Type: "bearer"}
	name, value, ok = credentialHeader(bearer, &OAuthToken{AccessToken: "tok"})
	assert.True(t, ok)
	assert.Equal(t, "Authorization", name)
	assert.Equal(t, "Bearer tok", value)
}
