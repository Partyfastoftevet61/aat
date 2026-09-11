package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/gburgyan/aat/config"
	"github.com/gburgyan/aat/engine"
	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteRunArchive_UnredactableWritesNothing checks that a run whose archive
// cannot be redacted leaves no archive behind instead of an unredacted one.
func TestWriteRunArchive_UnredactableWritesNothing(t *testing.T) {
	dir := t.TempDir()
	env := &config.Environment{
		Name: "test",
		Auth: config.AuthConfig{Type: "apikey", HeaderName: "X-API-Key", Credentials: map[string]config.SecretRef{
			"key": {Source: "literal", Value: "sk-test-0123456789"},
		}},
	}
	result := &engine.RunResult{
		Outcome: engine.OutcomePassed,
		Steps: []engine.StepResult{{
			StepID:  "getQuote",
			Node:    "getQuote",
			Inputs:  map[string]any{"key": "sk-test-0123456789"},
			Outputs: map[string]any{"rate": math.NaN()},
		}},
	}

	path, err := writeRunArchive(result, &plan.Plan{}, env, &graph.Graph{Version: "1"}, dir, nil)
	require.Error(t, err)
	assert.Empty(t, path)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "no run directory is created")
}

func writeOverlay(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0644))
	return path
}

func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	orig, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(orig) })
	require.NoError(t, os.Chdir(resolved))
	return resolved
}

func TestResolveOverlayEnvName_ExplicitOnly(t *testing.T) {
	chdirTemp(t)
	explicit := writeOverlay(t, t.TempDir(), "overlay.yaml", "environment: qa\n")

	env, src, err := resolveOverlayEnvName(explicit, false)
	require.NoError(t, err)
	assert.Equal(t, "qa", env)
	assert.Equal(t, explicit, src)
}

func TestResolveOverlayEnvName_AutoOnly(t *testing.T) {
	dir := chdirTemp(t)
	autoPath := filepath.Join(dir, config.AutoOverridesFile)
	require.NoError(t, os.WriteFile(autoPath, []byte("environment: dev\n"), 0644))

	env, src, err := resolveOverlayEnvName("", false)
	require.NoError(t, err)
	assert.Equal(t, "dev", env)
	assert.Equal(t, autoPath, src)
}

func TestResolveOverlayEnvName_ExplicitBeatsAuto(t *testing.T) {
	dir := chdirTemp(t)
	autoPath := filepath.Join(dir, config.AutoOverridesFile)
	require.NoError(t, os.WriteFile(autoPath, []byte("environment: dev\n"), 0644))

	explicit := writeOverlay(t, t.TempDir(), "overlay.yaml", "environment: qa\n")

	env, src, err := resolveOverlayEnvName(explicit, false)
	require.NoError(t, err)
	assert.Equal(t, "qa", env)
	assert.Equal(t, explicit, src)
}

func TestResolveOverlayEnvName_ExplicitWithoutEnvFallsThroughToAuto(t *testing.T) {
	// An explicit overlay that doesn't set `environment:` should not mask the
	// auto-discovered overlay's environment.
	dir := chdirTemp(t)
	autoPath := filepath.Join(dir, config.AutoOverridesFile)
	require.NoError(t, os.WriteFile(autoPath, []byte("environment: dev\n"), 0644))

	explicit := writeOverlay(t, t.TempDir(), "overlay.yaml", "overrides:\n  - match: '*'\n    baseUrl: http://localhost\n")

	env, src, err := resolveOverlayEnvName(explicit, false)
	require.NoError(t, err)
	assert.Equal(t, "dev", env)
	assert.Equal(t, autoPath, src)
}

func TestResolveOverlayEnvName_NoAutoOverrides(t *testing.T) {
	dir := chdirTemp(t)
	autoPath := filepath.Join(dir, config.AutoOverridesFile)
	require.NoError(t, os.WriteFile(autoPath, []byte("environment: dev\n"), 0644))

	env, src, err := resolveOverlayEnvName("", true)
	require.NoError(t, err)
	assert.Empty(t, env)
	assert.Empty(t, src)
}

func TestResolveOverlayEnvName_NeitherSet(t *testing.T) {
	chdirTemp(t)

	env, src, err := resolveOverlayEnvName("", false)
	require.NoError(t, err)
	assert.Empty(t, env)
	assert.Empty(t, src)
}

func TestResolveOverlayEnvName_ExplicitFilePropagatesError(t *testing.T) {
	chdirTemp(t)

	_, _, err := resolveOverlayEnvName("/nonexistent/overlay.yaml", true)
	assert.Error(t, err)
}
