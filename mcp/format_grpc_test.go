package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/archive"
	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
	"github.com/mark3labs/mcp-go/mcp"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
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

func TestFormatStep_ExpectFailureNamesTheGRPCCode(t *testing.T) {
	step := &archive.StepRecord{
		StepID: "charge", Node: "paymentCharge",
		Request:  &archive.RequestRecord{Protocol: "grpc", URL: "grpc://localhost:8767/shop.v1.Payments/Charge"},
		Response: &archive.ResponseRecord{Status: 400, GRPCCode: "INVALID_ARGUMENT"},
		ExpectFailure: &archive.ExpectFailureRecord{
			Expected:   plan.ExpectedStatuses{{Code: 400, Name: "INVALID_ARGUMENT"}},
			Actual:     400,
			ActualName: "INVALID_ARGUMENT",
			Passed:     true,
		},
	}
	out := formatStepRecord(step, 1, 1)
	assert.Contains(t, out, "got INVALID_ARGUMENT")
	assert.NotContains(t, out, "got 400", "the HTTP status it maps to is not what the plan wrote")
}

func TestFormatStep_ExpectFailureHTTPIsUnchanged(t *testing.T) {
	step := &archive.StepRecord{
		StepID: "get", Node: "getCart",
		Request:  &archive.RequestRecord{Method: "GET", URL: "http://x/carts/1"},
		Response: &archive.ResponseRecord{Status: 404},
		ExpectFailure: &archive.ExpectFailureRecord{
			Expected: plan.HTTPStatuses([]int{404}),
			Actual:   404,
			Passed:   true,
		},
	}
	assert.Contains(t, formatStepRecord(step, 1, 1), "got 404")
}

// grpcNodeContext is a project with one gRPC node beside the HTTP ones.
func grpcNodeContext(t *testing.T) *ServerContext {
	t.Helper()
	ctx := buildPersonaTestContext()
	ctx.Graph.Nodes["charge"] = &graph.Node{
		Name:        "charge",
		Description: "Capture payment",
		Adapter:     "chargeTmpl",
		Proto:       &graph.ProtoRef{Service: "shop.v1.Payments", Method: "Charge"},
	}
	require.NoError(t, ctx.Registry.Register("chargeTmpl", adapter.NewTemplateAdapter(adapter.Template{
		Adapter: "chargeTmpl", Protocol: adapter.ProtocolGRPC,
		Request: adapter.TemplateRequest{RPC: "shop.v1.Payments/Charge"},
	})))
	return ctx
}

// A gRPC node has no verb and no path, and used to be listed with neither.
func TestHandleAPIOverviewResource_NamesAGRPCMethod(t *testing.T) {
	srv := NewIntegrationServer(grpcNodeContext(t))

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "aat://api/overview"
	contents, err := srv.handleAPIOverviewResource(context.Background(), req)
	require.NoError(t, err)

	text := contents[0].(mcp.TextResourceContents).Text
	assert.Contains(t, text, "**charge** gRPC `shop.v1.Payments/Charge` — Capture payment")
	assert.Contains(t, text, "**search** POST `/api/search`", "an HTTP node reads as before")
}

// Asked for a gRPC node's OpenAPI operation, the server says what the node is
// rather than that something is missing.
func TestHandleGetOASOperation_GRPCNode(t *testing.T) {
	ctx := grpcNodeContext(t)
	ctx.OASSpecs = map[string]*v3high.Document{"test.yaml": {}}
	srv := NewServer(ctx)

	result := callTool(t, srv.handleGetOASOperation, map[string]any{"node": "charge"})

	assert.True(t, result.IsError)
	text := resultText(t, result)
	assert.Contains(t, text, "gRPC method shop.v1.Payments/Charge")
	assert.Contains(t, text, "inspect_template")
	assert.NotContains(t, text, "HTTP")
}
