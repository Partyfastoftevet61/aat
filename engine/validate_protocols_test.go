package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// protocolRegistry registers one template per name, gRPC ones carrying an rpc.
func protocolRegistry(t *testing.T, templates ...adapter.Template) *adapter.Registry {
	t.Helper()
	registry := adapter.NewRegistry()
	for _, tmpl := range templates {
		require.NoError(t, registry.Register(tmpl.Adapter, adapter.NewTemplateAdapter(tmpl)))
	}
	return registry
}

func grpcTemplate(name, rpc string) adapter.Template {
	return adapter.Template{Adapter: name, Protocol: adapter.ProtocolGRPC,
		Request: adapter.TemplateRequest{RPC: rpc}}
}

func httpTemplate(name string) adapter.Template {
	return adapter.Template{Adapter: name, Protocol: adapter.ProtocolHTTP,
		Request: adapter.TemplateRequest{Method: "POST", Path: "/things"}}
}

func protoNode(adapterName, service, method string) *graph.Node {
	return &graph.Node{Name: adapterName, Adapter: adapterName,
		Proto: &graph.ProtoRef{Service: service, Method: method}}
}

func TestValidateNodeProtocols(t *testing.T) {
	tests := []struct {
		name      string
		node      *graph.Node
		template  adapter.Template
		errors    int
		warnings  int
		grpcNodes int
		contains  string
	}{
		{
			name:      "matched grpc pair",
			node:      protoNode("charge", "shop.v1.Payments", "Charge"),
			template:  grpcTemplate("charge", "shop.v1.Payments/Charge"),
			grpcNodes: 1,
		},
		{
			name:      "dotted rpc spelling matches a slashed proto ref",
			node:      protoNode("charge", "shop.v1.Payments", "Charge"),
			template:  grpcTemplate("charge", "shop.v1.Payments.Charge"),
			grpcNodes: 1,
		},
		{
			name:     "proto on an http template",
			node:     protoNode("charge", "shop.v1.Payments", "Charge"),
			template: httpTemplate("charge"),
			errors:   1,
			contains: "set protocol: grpc on the template",
		},
		{
			name:      "grpc template without proto",
			node:      &graph.Node{Name: "charge", Adapter: "charge"},
			template:  grpcTemplate("charge", "shop.v1.Payments/Charge"),
			warnings:  1,
			grpcNodes: 1,
			contains:  "add proto: shop.v1.Payments/Charge to the node",
		},
		{
			name:      "rpc method mismatch",
			node:      protoNode("charge", "shop.v1.Payments", "Charge"),
			template:  grpcTemplate("charge", "shop.v1.Payments/Refund"),
			errors:    1,
			grpcNodes: 1,
			contains:  "name the same method",
		},
		{
			name:      "rpc service mismatch",
			node:      protoNode("charge", "shop.v1.Payments", "Charge"),
			template:  grpcTemplate("charge", "other.v1.Payments/Charge"),
			errors:    1,
			grpcNodes: 1,
			contains:  "name the same method",
		},
		{
			name:     "plain http node",
			node:     &graph.Node{Name: "charge", Adapter: "charge"},
			template: httpTemplate("charge"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &graph.Graph{Nodes: map[string]*graph.Node{tt.node.Name: tt.node}}
			report := ValidateNodeProtocols(g, protocolRegistry(t, tt.template))

			var errs, warns int
			for _, issue := range report.Result.Issues {
				if issue.Severity == graph.SpecError {
					errs++
				} else {
					warns++
				}
			}
			assert.Equal(t, tt.errors, errs, "errors: %s", report.Result.Format())
			assert.Equal(t, tt.warnings, warns, "warnings: %s", report.Result.Format())
			assert.Equal(t, tt.grpcNodes, report.GRPCNodes)
			if tt.contains != "" {
				assert.Contains(t, report.Result.Format(), tt.contains)
			}
			if tt.errors == 0 {
				assert.NoError(t, report.Err(), "warnings must not stop a run")
			} else {
				assert.Error(t, report.Err())
			}
		})
	}
}

// A cleanup node declares no outputs, so the adapter-output check skips it.
// The protocol check must not.
func TestValidateNodeProtocols_VoidNodeStillChecked(t *testing.T) {
	g := &graph.Graph{Nodes: map[string]*graph.Node{
		"deleteCart": protoNode("deleteCart", "shop.v1.Carts", "DeleteCart"),
	}}
	report := ValidateNodeProtocols(g, protocolRegistry(t, httpTemplate("deleteCart")))

	require.Error(t, report.Err())
	assert.Contains(t, report.Err().Error(), "deleteCart")
}

func TestValidateNodeProtocols_CustomAdapterSkipped(t *testing.T) {
	g := &graph.Graph{Nodes: map[string]*graph.Node{
		"custom": protoNode("custom", "shop.v1.Payments", "Charge"),
	}}
	report := ValidateNodeProtocols(g, adapter.NewRegistry())

	assert.False(t, report.Result.HasIssues(), "a node with no template declares no protocol")
	assert.Equal(t, 0, report.GRPCNodes)
	assert.Equal(t, 0, report.HTTPNodes)
}

func TestValidateNodeProtocols_CountsBothProtocols(t *testing.T) {
	g := &graph.Graph{Nodes: map[string]*graph.Node{
		"charge": protoNode("charge", "shop.v1.Payments", "Charge"),
		"cart":   {Name: "cart", Adapter: "cart"},
		"item":   {Name: "item", Adapter: "item"},
	}}
	report := ValidateNodeProtocols(g, protocolRegistry(t,
		grpcTemplate("charge", "shop.v1.Payments/Charge"), httpTemplate("cart"), httpTemplate("item")))

	assert.False(t, report.Result.HasIssues(), report.Result.Format())
	assert.Equal(t, 1, report.GRPCNodes)
	assert.Equal(t, 2, report.HTTPNodes)
}

func TestValidateNodeProtocolsForPlan_OnlyPlanNodes(t *testing.T) {
	g := &graph.Graph{Nodes: map[string]*graph.Node{
		"charge":  protoNode("charge", "shop.v1.Payments", "Charge"),
		"unused":  protoNode("unused", "shop.v1.Payments", "Refund"),
		"cleanup": protoNode("cleanup", "shop.v1.Carts", "DeleteCart"),
	}}
	// Every template is HTTP, so every node mismatches; only the plan's count.
	registry := protocolRegistry(t, httpTemplate("charge"), httpTemplate("unused"), httpTemplate("cleanup"))

	p := &plan.Plan{Execution: plan.Execution{
		Steps:   []plan.Step{{ID: "s1", Node: "charge"}},
		Cleanup: []plan.CleanupStep{{Node: "cleanup"}},
	}}
	report := ValidateNodeProtocolsForPlan(g, registry, p)

	require.Error(t, report.Err())
	msg := report.Err().Error()
	assert.Contains(t, msg, "charge")
	assert.Contains(t, msg, "cleanup")
	assert.NotContains(t, msg, "unused", "a node the plan never touches is not the run's problem")
}

func TestProtocolReport_ErrOmitsWarnings(t *testing.T) {
	g := &graph.Graph{Nodes: map[string]*graph.Node{
		"charge": {Name: "charge", Adapter: "charge"},
	}}
	report := ValidateNodeProtocols(g, protocolRegistry(t, grpcTemplate("charge", "shop.v1.Payments/Charge")))

	assert.True(t, report.Result.HasIssues())
	assert.False(t, report.Result.HasErrors())
	assert.NoError(t, report.Err())
}

// The point of checking at run start: a mismatched project stops before the
// first step creates anything.
func TestRun_FailsBeforeFirstStepOnProtocolMismatch(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"c1"}`))
	}))
	defer srv.Close()

	g := &graph.Graph{Nodes: map[string]*graph.Node{
		"charge": protoNode("charge", "shop.v1.Payments", "Charge"),
	}}
	registry := protocolRegistry(t, httpTemplate("charge"))
	eng := NewEngine(g, registry, NewExecutorRouter(adapter.NewHTTPExecutor(srv.URL), &adapter.EnvironmentConfig{}))

	result := eng.Run(context.Background(), &plan.Plan{Execution: plan.Execution{
		Steps: []plan.Step{{ID: "s1", Node: "charge"}},
	}})

	require.Equal(t, OutcomeError, result.Outcome)
	assert.Contains(t, result.Error.Error(), "node protocol validation failed")
	assert.Zero(t, calls, "no request is sent when the project's protocols disagree")
	assert.Empty(t, result.Steps)
}

func TestValidateNodeProtocols_NilRegistry(t *testing.T) {
	g := &graph.Graph{Nodes: map[string]*graph.Node{"a": {Name: "a", Adapter: "a"}}}
	report := ValidateNodeProtocols(g, nil)
	assert.NotNil(t, report.Result)
	assert.NoError(t, report.Err())
}
