package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gburgyan/aat/graph/oas"
)

func TestFormatOperationDetail_IncludesPathItemParameters(t *testing.T) {
	spec := `openapi: 3.0.3
info:
  title: carts
  version: 1.0.0
paths:
  /carts/{cartId}:
    parameters:
      - name: cartId
        in: path
        required: true
        schema:
          type: string
    get:
      operationId: getCart
      responses:
        '200':
          description: The cart
`
	path := filepath.Join(t.TempDir(), "carts.yaml")
	require.NoError(t, os.WriteFile(path, []byte(spec), 0o644))
	doc, err := oas.LoadSpec(path)
	require.NoError(t, err)
	method, specPath, _, op, err := oas.FindOperation(doc, "getCart")
	require.NoError(t, err)

	out := formatOperationDetail(method, specPath, op, doc)

	assert.Contains(t, out, "## Parameters")
	assert.Contains(t, out, "| cartId | path | string | yes |")
}
