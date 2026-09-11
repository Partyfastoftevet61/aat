package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectTerminal_ChecksTheGivenFile checks that a redirected writer is not
// treated as a terminal, whatever stdout is.
func TestDetectTerminal_ChecksTheGivenFile(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "progress.log"))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	assert.Equal(t, TerminalInfo{IsTTY: false, Width: 80, Height: 24}, DetectTerminal(f))
}

func TestDetectTerminal_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	assert.Equal(t, TerminalInfo{IsTTY: false, Width: 80, Height: 24}, DetectTerminal(os.Stdout))
}
