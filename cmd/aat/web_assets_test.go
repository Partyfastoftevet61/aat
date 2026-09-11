package main

import (
	"errors"
	"testing"

	"github.com/gburgyan/aat/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipWithoutWebAssets skips a test that serves the web UI when the frontend
// bundle is not embedded, as in a plain `go test ./...` without `make frontend`.
func skipWithoutWebAssets(t *testing.T) {
	t.Helper()
	if !server.HasWebAssets() {
		t.Skip("frontend bundle not embedded; run `make frontend` (make test does) to include this test")
	}
}

func TestRequireWebAssets(t *testing.T) {
	assert.NoError(t, requireWebAssets(true), "dev mode proxies to Vite and needs no bundle")

	err := requireWebAssets(false)
	if server.HasWebAssets() {
		assert.NoError(t, err)
		return
	}
	require.Error(t, err)
	var exitErr *exitError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, 2, exitErr.Code)
	assert.Contains(t, err.Error(), "make build")
}
