package oas

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate_PathItemParameters(t *testing.T) {
	model, err := LoadSpec("testdata/path_item_params.yaml")
	require.NoError(t, err)

	result, err := Generate(model, "path_item_params.yaml")
	require.NoError(t, err)

	node := result.Graph.Nodes["getCart"]
	require.NotNil(t, node)
	optional := map[string]bool{}
	for _, in := range node.Inputs {
		optional[in.Name] = in.Optional
	}
	assert.Equal(t, map[string]bool{"cartId": false, "X-Trace": false}, optional,
		"cartId is inherited from the path item; the operation makes X-Trace required")

	require.Len(t, result.Templates, 1)
	assert.Equal(t, "/carts/{{cartId}}", result.Templates[0].Request.Path)
	assert.Equal(t, "{{X-Trace}}", result.Templates[0].Request.Headers["X-Trace"])
}
