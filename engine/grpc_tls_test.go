package engine

import (
	"path/filepath"
	"testing"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/config"
	"github.com/stretchr/testify/assert"
)

func TestGRPCTLS(t *testing.T) {
	t.Run("no gRPC block", func(t *testing.T) {
		assert.Equal(t, adapter.TLSConfig{}, GRPCTLS(&config.Environment{}, "/project"))
		assert.Equal(t, adapter.TLSConfig{}, GRPCTLS(nil, "/project"))
	})

	t.Run("relative paths resolve beside the environment file", func(t *testing.T) {
		env := &config.Environment{GRPC: &config.GRPCConfig{TLS: &config.GRPCTLSConfig{
			CAFile:     "certs/ca.pem",
			CertFile:   "certs/client.pem",
			KeyFile:    "certs/client-key.pem",
			ServerName: "api.internal",
		}}}
		got := GRPCTLS(env, "/project")
		assert.Equal(t, filepath.Join("/project", "certs/ca.pem"), got.CAFile)
		assert.Equal(t, filepath.Join("/project", "certs/client.pem"), got.CertFile)
		assert.Equal(t, filepath.Join("/project", "certs/client-key.pem"), got.KeyFile)
		assert.Equal(t, "api.internal", got.ServerName)
		assert.False(t, got.Insecure)
	})

	t.Run("absolute paths are left alone", func(t *testing.T) {
		env := &config.Environment{GRPC: &config.GRPCConfig{TLS: &config.GRPCTLSConfig{
			CAFile:             "/etc/ssl/ca.pem",
			InsecureSkipVerify: true,
		}}}
		got := GRPCTLS(env, "/project")
		assert.Equal(t, "/etc/ssl/ca.pem", got.CAFile)
		assert.True(t, got.Insecure)
	})
}
