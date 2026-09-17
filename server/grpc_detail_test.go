package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gburgyan/aat/archive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToRequestDetail_GRPC(t *testing.T) {
	d := toRequestDetail(&archive.RequestRecord{
		Method:   "GRPC",
		URL:      "grpc://127.0.0.1:9455/shop.v1.Carts/CreateCart",
		Protocol: "grpc",
		Headers:  map[string]string{"x-tenant": "acme"},
		Body:     json.RawMessage(`{"customerId":"c-1"}`),
	})
	require.NotNil(t, d)
	assert.Equal(t, "grpc", d.Protocol)
	assert.Equal(t, "grpc://127.0.0.1:9455", d.Target, "the service it went to")
	assert.Equal(t, "shop.v1.Carts/CreateCart", d.RPC, "the method it called")
}

func TestToRequestDetail_HTTPIsUnchanged(t *testing.T) {
	d := toRequestDetail(&archive.RequestRecord{Method: "POST", URL: "https://api.example.com/carts"})
	require.NotNil(t, d)
	assert.Empty(t, d.Protocol)
	assert.Empty(t, d.RPC)
	assert.Empty(t, d.Target)
	assert.Equal(t, "POST", d.Method)
}

func TestSplitGRPCURL(t *testing.T) {
	tests := []struct {
		url    string
		target string
		rpc    string
	}{
		{"grpc://host:9090/pkg.Svc/Method", "grpc://host:9090", "pkg.Svc/Method"},
		{"grpcs://api.example.com:443/pkg.Svc/Method", "grpcs://api.example.com:443", "pkg.Svc/Method"},
		// Not a gRPC URL, or one without a method: show it whole rather than nothing.
		{"grpc://host:9090", "grpc://host:9090", ""},
		{"https://api.example.com/carts", "https://api.example.com/carts", ""},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			target, rpc := splitGRPCURL(tt.url)
			assert.Equal(t, tt.target, target)
			assert.Equal(t, tt.rpc, rpc)
		})
	}
}

func TestToResponseDetail_GRPC(t *testing.T) {
	d := toResponseDetail(&archive.ResponseRecord{
		Status:      404,
		Headers:     map[string]string{"x-request-id": "req-1"},
		Trailers:    map[string]string{"x-cost": "3"},
		GRPCCode:    "NOT_FOUND",
		GRPCMessage: "cart 7 not found",
		GRPCDetails: []json.RawMessage{json.RawMessage(`{"reason":"CART_MISSING"}`)},
		Body:        json.RawMessage(`{"code":"NOT_FOUND"}`),
	})
	require.NotNil(t, d)
	assert.Equal(t, 404, d.Status, "the mapped status, for readers that compare numbers")
	assert.Equal(t, "NOT_FOUND", d.GRPCCode, "the code the server actually sent")
	assert.Equal(t, "cart 7 not found", d.GRPCMessage)
	require.Len(t, d.Trailers, 1)
	assert.Equal(t, "x-cost", d.Trailers[0].Name)
	require.Len(t, d.Headers, 1)
	assert.Equal(t, "x-request-id", d.Headers[0].Name, "trailers stay out of the headers")
	assert.Len(t, d.GRPCDetails, 1)
}

func TestToStepSummary_CarriesGRPCCode(t *testing.T) {
	s := toStepSummary(archive.StepRecord{
		Node:     "createCart",
		Response: &archive.ResponseRecord{Status: 404, GRPCCode: "NOT_FOUND"},
	}, false, time.Time{})
	assert.Equal(t, 404, s.Status)
	assert.Equal(t, "NOT_FOUND", s.GRPCCode)
}
