package config

import (
	"testing"

	"github.com/gburgyan/aat/internal/yamlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironment_GRPCTLSParses(t *testing.T) {
	var env Environment
	require.NoError(t, yamlx.Decode([]byte(`
environment: prod
apiBaseUrl: grpcs://api.example.com:443
grpc:
  tls:
    caFile: ca.pem
    certFile: client.pem
    keyFile: client-key.pem
    serverName: api.internal
    insecureSkipVerify: true
`), &env))

	require.NotNil(t, env.GRPC)
	require.NotNil(t, env.GRPC.TLS)
	assert.Equal(t, "ca.pem", env.GRPC.TLS.CAFile)
	assert.Equal(t, "client.pem", env.GRPC.TLS.CertFile)
	assert.Equal(t, "client-key.pem", env.GRPC.TLS.KeyFile)
	assert.Equal(t, "api.internal", env.GRPC.TLS.ServerName)
	assert.True(t, env.GRPC.TLS.InsecureSkipVerify)
}

func TestEnvironment_GRPCTLSRejectsUnknownKeys(t *testing.T) {
	var env Environment
	err := yamlx.Decode([]byte("environment: p\ngrpc:\n  tls:\n    ca: ca.pem\n"), &env)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown key "ca"`)
}

func TestEnvironment_WithoutGRPCBlock(t *testing.T) {
	var env Environment
	require.NoError(t, yamlx.Decode([]byte("environment: p\napiBaseUrl: https://api.example.com\n"), &env))
	assert.Nil(t, env.GRPC, "an HTTP-only environment carries none")
}
