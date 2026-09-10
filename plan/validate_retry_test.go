package plan

import (
	"testing"

	"github.com/gburgyan/aat/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidRetryRule(t *testing.T) {
	for _, ok := range []string{"transient", "Server", "response_error", "503", " 429 ", "100", "599"} {
		assert.True(t, ValidRetryRule(ok), ok)
	}
	for _, bad := range []string{"", "bogus", "5xx", "99", "600", "-1", "1.5"} {
		assert.False(t, ValidRetryRule(bad), bad)
	}
}

func TestValidate_RetryRules(t *testing.T) {
	g := &graph.Graph{Version: "1.0.0", Nodes: map[string]*graph.Node{"a": {Name: "a", Adapter: "a"}}}
	mk := func(rc *RetryConfig) *Plan {
		return &Plan{
			Metadata:  Metadata{GraphVersion: "1.0.0"},
			Execution: Execution{Steps: []Step{{Node: "a", Retry: rc}}},
		}
	}

	require.NoError(t, Validate(mk(&RetryConfig{Max: 2, On: []string{"503", "transient"}, FailOn: []string{"401"}}), g))
	require.NoError(t, Validate(mk(nil), g))

	err := Validate(mk(&RetryConfig{Max: 2, On: []string{"bogus", "999"}}), g)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `retry.on has unknown rule "bogus"`)
	assert.Contains(t, err.Error(), `retry.on has unknown rule "999"`)

	err = Validate(mk(&RetryConfig{Max: -1, FailOn: []string{"5xx"}}), g)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry.max must be >= 0")
	assert.Contains(t, err.Error(), `retry.failOn has unknown rule "5xx"`)
}
