package mcp

import (
	"encoding/json"
	"testing"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/archive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestLine(t *testing.T) {
	assert.Equal(t, "POST https://api.example.com/carts",
		requestLine(&archive.RequestRecord{Method: "POST", URL: "https://api.example.com/carts"}))
	assert.Equal(t, "gRPC grpc://host:9090/shop.v1.Carts/CreateCart",
		requestLine(&archive.RequestRecord{Method: "GRPC", Protocol: "grpc", URL: "grpc://host:9090/shop.v1.Carts/CreateCart"}))
}

func TestStatusText(t *testing.T) {
	assert.Equal(t, "404", statusText(&archive.ResponseRecord{Status: 404}))
	assert.Equal(t, "NOT_FOUND", statusText(&archive.ResponseRecord{Status: 404, GRPCCode: "NOT_FOUND"}))
	assert.Equal(t, "NOT_FOUND (no such cart)",
		statusText(&archive.ResponseRecord{Status: 404, GRPCCode: "NOT_FOUND", GRPCMessage: "no such cart"}))
}

func TestFormatTemplate_GRPC(t *testing.T) {
	tmpl, err := adapter.ParseTemplate([]byte(`adapter: createCart
protocol: grpc
request:
  rpc: shop.v1.Carts/CreateCart
  metadata:
    x-tenant: "{{tenant}}"
  message: |
    {"customerId": "{{customerId}}"}
response:
  extract:
    cartId: cartId
`))
	require.NoError(t, err)

	out := formatTemplate(tmpl)
	assert.Contains(t, out, "**Protocol:** grpc")
	assert.Contains(t, out, "**Service:** shop.v1.Carts")
	assert.Contains(t, out, "**Method:** CreateCart")
	assert.Contains(t, out, "## Metadata")
	assert.Contains(t, out, "x-tenant")
	assert.Contains(t, out, "## Message")
	assert.Contains(t, out, "customerId")
	assert.NotContains(t, out, "**Path:**", "a gRPC call has no path")
	assert.NotContains(t, out, "## Headers", "a gRPC call sends metadata")
}

func TestFormatTemplate_HTTPIsUnchanged(t *testing.T) {
	tmpl, err := adapter.ParseTemplate([]byte("adapter: t\nrequest:\n  method: POST\n  path: /carts\n  headers:\n    Accept: application/json\n  body: \"{}\"\n"))
	require.NoError(t, err)

	out := formatTemplate(tmpl)
	assert.Contains(t, out, "**Method:** POST")
	assert.Contains(t, out, "**Path:** `/carts`")
	assert.Contains(t, out, "## Headers")
	assert.Contains(t, out, "## Body")
	assert.NotContains(t, out, "## Metadata")
}

func TestFormatStep_GRPCNamesTheCode(t *testing.T) {
	out := formatStepRecord(&archive.StepRecord{
		Node:       "createCart",
		DurationMs: 3,
		Request:    &archive.RequestRecord{Method: "GRPC", Protocol: "grpc", URL: "grpc://host:9090/shop.v1.Carts/CreateCart"},
		Response: &archive.ResponseRecord{
			Status: 404, GRPCCode: "NOT_FOUND", GRPCMessage: "no such cart",
			Body: json.RawMessage(`{"code":"NOT_FOUND"}`),
		},
	}, 1, 1)
	assert.Contains(t, out, "gRPC grpc://host:9090/shop.v1.Carts/CreateCart")
	assert.Contains(t, out, "NOT_FOUND (no such cart)")
	assert.NotContains(t, out, "→ 404", "the HTTP status it maps to is AAT's business, not the reader's")
}
