package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/archive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// grpcStepRecord is an archived gRPC step that failed with NOT_FOUND.
func grpcStepRecord() *archive.StepRecord {
	return &archive.StepRecord{
		StepID: "open",
		Node:   "createCart",
		Request: &archive.RequestRecord{
			Method:   "GRPC",
			Protocol: "grpc",
			URL:      "grpc://127.0.0.1:9455/shop.v1.Carts/CreateCart",
		},
		Response: &archive.ResponseRecord{
			Status:      404,
			GRPCCode:    "NOT_FOUND",
			GRPCMessage: "no such customer",
			Body:        json.RawMessage(`{"code":"NOT_FOUND","message":"no such customer"}`),
		},
	}
}

func TestShowStep_GRPCNamesTheCode(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, showStep(&out, grpcStepRecord(), "open", false, showText))

	text := out.String()
	assert.Contains(t, text, "GRPC grpc://127.0.0.1:9455/shop.v1.Carts/CreateCart")
	assert.Contains(t, text, "status NOT_FOUND (no such customer)")
	assert.NotContains(t, text, "status 404", "the mapped status is AAT's business, not the reader's")
}

func TestShowStep_HTTPIsUnchanged(t *testing.T) {
	var out bytes.Buffer
	step := &archive.StepRecord{
		StepID:   "get",
		Node:     "getCart",
		Request:  &archive.RequestRecord{Method: "GET", URL: "https://api.example.com/carts/1"},
		Response: &archive.ResponseRecord{Status: 404},
	}
	require.NoError(t, showStep(&out, step, "get", false, showText))
	assert.Contains(t, out.String(), "status 404")
}

func TestShowStep_GRPCJSONCarriesBothForms(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, showStep(&out, grpcStepRecord(), "open", false, showJSON))

	var view map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &view))
	assert.Equal(t, float64(404), view["status"], "a tool comparing numbers still can")
	assert.Equal(t, "NOT_FOUND", view["grpc_code"])
	assert.Equal(t, "no such customer", view["grpc_message"])
}

func TestColorStatusNamed(t *testing.T) {
	// A gRPC step shows its own code, colored by the status it maps to.
	assert.Equal(t, "NOT_FOUND", colorStatusNamed(404, "NOT_FOUND", false))
	assert.Equal(t, "404", colorStatusNamed(404, "", false))
	assert.Contains(t, colorStatusNamed(404, "NOT_FOUND", true), "NOT_FOUND")
	assert.Contains(t, colorStatusNamed(404, "NOT_FOUND", true), colorYellow, "4xx is yellow whatever names it")
	assert.Contains(t, colorStatusNamed(503, "UNAVAILABLE", true), colorRed)
	assert.Contains(t, colorStatusNamed(200, "OK", true), colorGreen)
}

func TestGRPCCodeName(t *testing.T) {
	assert.Equal(t, "", grpcCodeName(nil))
	assert.Equal(t, "", grpcCodeName(&adapter.Response{StatusCode: 404}))
	assert.Equal(t, "NOT_FOUND", grpcCodeName(&adapter.Response{
		StatusCode: 404, GRPC: &adapter.GRPCStatus{Name: "NOT_FOUND"},
	}))
}
