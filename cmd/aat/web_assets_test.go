package main

import (
	"errors"
	"testing"

	"github.com/gburgyan/aat/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
