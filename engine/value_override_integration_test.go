package engine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOverlayValueOverride_InjectsMalformedValue verifies that overlay values
// replace plan-supplied values in the outgoing request, and that an overlay
// expectFailure flips the step's pass/fail logic — the primitive for authoring
// negative tests without editing the plan.
func TestOverlayValueOverride_InjectsMalformedValue(t *testing.T) {
	g := &graph.Graph{
		Version: "1.0.0",
		Nodes: map[string]*graph.Node{
			"search": {
				Name:    "search",
				Adapter: "test.search",
				Inputs: []graph.Input{
					{Name: "query", Type: "string"},
				},
			},
		},
	}

	var seenBodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		seenBodies = append(seenBodies, parsed)

		w.Header().Set("Content-Type", "application/json")
		if q, _ := parsed["query"].(string); q == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"query is required"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	registry := adapter.NewRegistry()
	require.NoError(t, registry.Register("test.search", &stubAdapter{
		method:   "POST",
		path:     "/search",
		response: map[string]any{"ok": true},
	}))

	executor := adapter.NewHTTPExecutor(server.URL)
	router := NewExecutorRouter(executor, &adapter.EnvironmentConfig{})

	// Overlay: rewrite query to empty, declare expected 400.
	router.AddValueOverride("search", map[string]any{"query": ""}, &plan.ExpectFailure{Status: plan.HTTPStatuses([]int{400})})

	engine := NewEngine(g, registry, router)

	p := &plan.Plan{
		Metadata: plan.Metadata{GraphVersion: "1.0.0"},
		Execution: plan.Execution{
			Steps: []plan.Step{
				{
					Node: "search",
					Values: map[string]plan.StepValue{
						"query": {Default: "this should be overwritten"},
					},
				},
			},
		},
	}

	result := engine.Run(context.Background(), p)

	assert.Equal(t, OutcomePassed, result.Outcome, "expected failure matched — run passes")
	require.Len(t, result.Steps, 1)
	sr := result.Steps[0]
	assert.Equal(t, 400, sr.StatusCode)
	require.NotNil(t, sr.ExpectFailure)
	assert.True(t, sr.ExpectFailure.Passed, "overlay expectFailure should be consulted when step declares none")

	require.Len(t, seenBodies, 1)
	assert.Equal(t, "", seenBodies[0]["query"], "overlay value must replace plan default in outgoing request")
}

// TestOverlayExpectFailure_SkipsStatusAssertion verifies that a step whose
// status assertion expects success (the recipe default "2xx") still passes when
// an overlay turns it into a negative test: expectFailure owns the status check,
// so the status assertion is reported as skipped while other assertions run.
func TestOverlayExpectFailure_SkipsStatusAssertion(t *testing.T) {
	g := &graph.Graph{
		Version: "1.0.0",
		Nodes: map[string]*graph.Node{
			"charge": {
				Name:    "charge",
				Adapter: "test.charge",
				Inputs:  []graph.Input{{Name: "card", Type: "string"}},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":{"code":"CARD_DECLINED"}}`))
	}))
	defer server.Close()

	registry := adapter.NewRegistry()
	require.NoError(t, registry.Register("test.charge", &stubAdapter{method: "POST", path: "/charges"}))
	router := NewExecutorRouter(adapter.NewHTTPExecutor(server.URL), &adapter.EnvironmentConfig{})
	router.AddValueOverride("charge", map[string]any{"card": "4000000000000002"}, &plan.ExpectFailure{Status: plan.HTTPStatuses([]int{402})})

	p := &plan.Plan{
		Metadata: plan.Metadata{GraphVersion: "1.0.0"},
		Execution: plan.Execution{
			Steps: []plan.Step{{
				Node:   "charge",
				Values: map[string]plan.StepValue{"card": {Default: "4242424242424242"}},
				Assertions: &plan.Assertions{Mechanical: []plan.MechanicalAssertion{
					{Type: "status", Expect: "2xx"},
					{Type: "fieldExists", Path: "error.code", Raw: true},
				}},
			}},
		},
	}

	result := NewEngine(g, registry, router).Run(context.Background(), p)

	assert.Equal(t, OutcomePassed, result.Outcome, "error: %v", result.Error)
	require.Len(t, result.Steps, 1)
	require.NotNil(t, result.Steps[0].Validation)
	results := result.Steps[0].Validation.Results
	require.Len(t, results, 2)
	assert.True(t, results[0].Skipped, "the status assertion is skipped under expectFailure")
	assert.Contains(t, results[0].Message, "expectFailure")
	assert.True(t, results[1].Passed, "other assertions still run")
	assert.False(t, results[1].Skipped)
}

// TestExpectFailure_StatusAssertionRule verifies that under expectFailure only
// status assertions that expect success are skipped; assertions that agree with
// the expected failure are evaluated and can fail the step.
func TestExpectFailure_StatusAssertionRule(t *testing.T) {
	g := &graph.Graph{
		Version: "1.0.0",
		Nodes: map[string]*graph.Node{
			"charge": {Name: "charge", Adapter: "test.charge"},
		},
	}

	tests := []struct {
		name        string
		status      int
		stepFailure *plan.ExpectFailure // declared on the step; nil means the overlay supplies [402]
		expect      any
		wantSkipped bool
		wantOutcome Outcome
	}{
		{name: "overlay, success class is skipped", status: 402, expect: "2xx", wantSkipped: true, wantOutcome: OutcomePassed},
		{name: "overlay, success code is skipped", status: 402, expect: 200, wantSkipped: true, wantOutcome: OutcomePassed},
		{name: "overlay, matching code is evaluated", status: 402, expect: 402, wantOutcome: OutcomePassed},
		{name: "overlay, matching class is evaluated", status: 402, expect: "4xx", wantOutcome: OutcomePassed},
		{name: "overlay, different failure code fails", status: 402, expect: 404, wantOutcome: OutcomeFailed},
		{
			name: "plan, narrower code within the expected list", status: 409,
			stepFailure: &plan.ExpectFailure{Status: plan.HTTPStatuses([]int{404, 409})},
			expect:      409, wantOutcome: OutcomePassed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"error":{"code":"REJECTED"}}`))
			}))
			defer server.Close()

			registry := adapter.NewRegistry()
			require.NoError(t, registry.Register("test.charge", &stubAdapter{method: "POST", path: "/charges"}))
			router := NewExecutorRouter(adapter.NewHTTPExecutor(server.URL), &adapter.EnvironmentConfig{})
			if tt.stepFailure == nil {
				router.AddValueOverride("charge", nil, &plan.ExpectFailure{Status: plan.HTTPStatuses([]int{402})})
			}

			p := &plan.Plan{
				Metadata: plan.Metadata{GraphVersion: "1.0.0"},
				Execution: plan.Execution{
					Steps: []plan.Step{{
						Node:          "charge",
						ExpectFailure: tt.stepFailure,
						Assertions: &plan.Assertions{Mechanical: []plan.MechanicalAssertion{
							{Type: "status", Expect: tt.expect},
						}},
					}},
				},
			}

			result := NewEngine(g, registry, router).Run(context.Background(), p)

			assert.Equal(t, tt.wantOutcome, result.Outcome, "error: %v", result.Error)
			require.Len(t, result.Steps, 1)
			require.NotNil(t, result.Steps[0].Validation)
			require.Len(t, result.Steps[0].Validation.Results, 1)
			assert.Equal(t, tt.wantSkipped, result.Steps[0].Validation.Results[0].Skipped)
		})
	}
}

// TestOverlayValueOverride_StepExpectFailureWins verifies that a plan-declared
// expectFailure is not overwritten by an overlay expectFailure.
func TestOverlayValueOverride_StepExpectFailureWins(t *testing.T) {
	g := &graph.Graph{
		Version: "1.0.0",
		Nodes: map[string]*graph.Node{
			"search": {
				Name:    "search",
				Adapter: "test.search",
				Inputs:  []graph.Input{{Name: "query", Type: "string"}},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"nf"}`))
	}))
	defer server.Close()

	registry := adapter.NewRegistry()
	require.NoError(t, registry.Register("test.search", &stubAdapter{
		method: "POST", path: "/search",
		response: map[string]any{},
	}))

	router := NewExecutorRouter(adapter.NewHTTPExecutor(server.URL), &adapter.EnvironmentConfig{})
	// Overlay declares 400 expected; plan step declares 404 expected — plan wins.
	router.AddValueOverride("search", nil, &plan.ExpectFailure{Status: plan.HTTPStatuses([]int{400})})

	engine := NewEngine(g, registry, router)

	p := &plan.Plan{
		Metadata: plan.Metadata{GraphVersion: "1.0.0"},
		Execution: plan.Execution{
			Steps: []plan.Step{
				{
					Node:          "search",
					Values:        map[string]plan.StepValue{"query": {Default: "x"}},
					ExpectFailure: &plan.ExpectFailure{Status: plan.HTTPStatuses([]int{404})},
				},
			},
		},
	}

	result := engine.Run(context.Background(), p)
	require.Len(t, result.Steps, 1)
	sr := result.Steps[0]
	require.NotNil(t, sr.ExpectFailure)
	assert.Equal(t, []int{404}, sr.ExpectFailure.ExpectedStatuses.Codes(), "plan step expectFailure takes precedence over overlay")
	assert.True(t, sr.ExpectFailure.Passed)
}
