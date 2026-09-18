package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A gRPC target with a path, or a plaintext one with no port, is reported by
// aat validate. Left to a run, grpc-go answers either with UNAVAILABLE, which
// names neither problem.
func TestValidateEnvironmentFile_GRPCTargets(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		wantErr []string
	}{
		{
			name: "a host and a port",
			env:  "environment: local\napiBaseUrl: grpc://localhost:8767\n",
		},
		{
			name:    "a path after a port",
			env:     "environment: local\napiBaseUrl: grpc://localhost:8767/shop.v1.Payments\n",
			wantErr: []string{"apiBaseUrl", "has a path"},
		},
		{
			name: "an override with no port",
			env: "environment: local\napiBaseUrl: http://localhost:8765\n" +
				"overrides:\n  - match: \"payment*\"\n    baseUrl: grpc://localhost\n",
			wantErr: []string{"overrides[0] (payment*)", "names no port"},
		},
		{
			name: "every environment of a multi-environment file",
			env: "environments:\n  us:\n    apiBaseUrl: grpc://localhost:8767\n" +
				"  eu:\n    apiBaseUrl: grpc://localhost\n",
			wantErr: []string{"eu: apiBaseUrl", "names no port"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "env.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tt.env), 0o644))

			section, _ := validateEnvironmentFile(path, "", nil)

			if tt.wantErr == nil {
				assert.Equal(t, "OK", section.Status, "errors: %v", section.Errors)
				return
			}
			require.Equal(t, "FAILED", section.Status)
			joined := strings.Join(section.Errors, "\n")
			for _, want := range tt.wantErr {
				assert.Contains(t, joined, want)
			}
		})
	}
}
