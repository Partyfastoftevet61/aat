package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/config"
	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutorRouter_AddResolvedOverride_ValueOnlyKeepsRoute(t *testing.T) {
	router := NewExecutorRouter(adapter.NewHTTPExecutor("http://shop"), &adapter.EnvironmentConfig{BaseURL: "http://shop"})
	router.AddResolvedOverride(config.ResolvedOverride{
		Pattern:   "payment*",
		Routes:    true,
		APIConfig: config.APIConfig{BaseURL: "http://payments", Headers: map[string]string{"X-API-Key": "k"}},
	})
	router.AddResolvedOverride(config.ResolvedOverride{
		Pattern:       "paymentCharge",
		APIConfig:     config.APIConfig{BaseURL: "http://shop"},
		Values:        map[string]any{"cardNumber": "4000000000000002"},
		ExpectFailure: &config.OverrideExpectFailure{Status: []int{402}},
	})

	exec, cfg, _ := router.Resolve("paymentCharge")
	assert.Equal(t, "http://payments", exec.BaseURL, "a value-only override must not replace the glob route")
	assert.Equal(t, "k", cfg.Headers["X-API-Key"])

	values, ef := router.ResolveValueOverride("paymentCharge")
	assert.Equal(t, "4000000000000002", values["cardNumber"])
	require.NotNil(t, ef)
	assert.Equal(t, []int{402}, ef.Status)
}

func TestExecutorRouter_InheritedEnvOverride_ChildWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
environments:
  _base:
    apiBaseUrl: https://api.example.com
    auth:
      type: none
    overrides:
      - match: "payment*"
        baseUrl: https://base.example.com
  child:
    extends: _base
    overrides:
      - match: "payment*"
        baseUrl: https://child.example.com
`), 0o644))

	env, err := config.LoadNamedEnvironment(path, "child")
	require.NoError(t, err)
	resolved, err := env.BuildOverrideConfigs(context.Background(), nil)
	require.NoError(t, err)

	router := NewExecutorRouter(adapter.NewHTTPExecutor(env.APIBaseURL), &adapter.EnvironmentConfig{BaseURL: env.APIBaseURL})
	for _, ov := range resolved {
		router.AddResolvedOverride(ov)
	}

	exec, _, _ := router.Resolve("paymentCharge")
	assert.Equal(t, "https://child.example.com", exec.BaseURL)
}

func TestExecutorRouter_DefaultRoute(t *testing.T) {
	defaultExec := adapter.NewHTTPExecutor("https://default.example.com")
	defaultCfg := &adapter.EnvironmentConfig{BaseURL: "https://default.example.com"}

	router := NewExecutorRouter(defaultExec, defaultCfg)

	exec, cfg, rewrite := router.Resolve("anyNode")
	assert.Equal(t, defaultExec, exec)
	assert.Equal(t, defaultCfg, cfg)
	assert.Nil(t, rewrite)
}

func TestExecutorRouter_ExactMatch(t *testing.T) {
	defaultExec := adapter.NewHTTPExecutor("https://default.example.com")
	overrideExec := adapter.NewHTTPExecutor("http://localhost:8080")
	overrideCfg := &adapter.EnvironmentConfig{BaseURL: "http://localhost:8080"}

	router := NewExecutorRouter(defaultExec, &adapter.EnvironmentConfig{})
	router.AddOverride("searchFlights", overrideExec, overrideCfg, nil)

	exec, cfg, rewrite := router.Resolve("searchFlights")
	assert.Equal(t, overrideExec, exec)
	assert.Equal(t, overrideCfg, cfg)
	assert.Nil(t, rewrite)

	// Non-matching node gets default
	exec, _, _ = router.Resolve("bookFlight")
	assert.Equal(t, defaultExec, exec)
}

func TestExecutorRouter_GlobMatch(t *testing.T) {
	defaultExec := adapter.NewHTTPExecutor("https://default.example.com")
	priceExec := adapter.NewHTTPExecutor("https://price.example.com")
	priceCfg := &adapter.EnvironmentConfig{BaseURL: "https://price.example.com"}

	router := NewExecutorRouter(defaultExec, &adapter.EnvironmentConfig{})
	router.AddOverride("price*", priceExec, priceCfg, nil)

	exec, cfg, _ := router.Resolve("priceOffer")
	assert.Equal(t, priceExec, exec)
	assert.Equal(t, priceCfg, cfg)

	exec, cfg, _ = router.Resolve("priceTicket")
	assert.Equal(t, priceExec, exec)
	assert.Equal(t, priceCfg, cfg)

	// Non-matching
	exec, _, _ = router.Resolve("searchFlights")
	assert.Equal(t, defaultExec, exec)
}

func TestExecutorRouter_ExactBeforeGlob(t *testing.T) {
	defaultExec := adapter.NewHTTPExecutor("https://default.example.com")
	globExec := adapter.NewHTTPExecutor("https://glob.example.com")
	exactExec := adapter.NewHTTPExecutor("https://exact.example.com")

	router := NewExecutorRouter(defaultExec, &adapter.EnvironmentConfig{})
	router.AddOverride("price*", globExec, &adapter.EnvironmentConfig{}, nil)
	router.AddOverride("priceOffer", exactExec, &adapter.EnvironmentConfig{}, nil)

	// Exact match wins over glob
	exec, _, _ := router.Resolve("priceOffer")
	assert.Equal(t, exactExec, exec)

	// Glob still works for non-exact
	exec, _, _ = router.Resolve("priceTicket")
	assert.Equal(t, globExec, exec)
}

func TestExecutorRouter_WithPathRewrite(t *testing.T) {
	defaultExec := adapter.NewHTTPExecutor("https://default.example.com")
	overrideExec := adapter.NewHTTPExecutor("http://localhost:8080")
	rewrite := &adapter.PathRewrite{Strip: "/11", Prefix: "/api/v2"}

	router := NewExecutorRouter(defaultExec, &adapter.EnvironmentConfig{})
	router.AddOverride("searchFlights", overrideExec, &adapter.EnvironmentConfig{}, rewrite)

	_, _, rw := router.Resolve("searchFlights")
	assert.NotNil(t, rw)
	assert.Equal(t, "/11", rw.Strip)
	assert.Equal(t, "/api/v2", rw.Prefix)
}

func TestExecutorRouter_LastGlobWins(t *testing.T) {
	defaultExec := adapter.NewHTTPExecutor("https://default.example.com")
	firstExec := adapter.NewHTTPExecutor("https://first.example.com")
	secondExec := adapter.NewHTTPExecutor("https://second.example.com")

	router := NewExecutorRouter(defaultExec, &adapter.EnvironmentConfig{})
	router.AddOverride("search*", firstExec, &adapter.EnvironmentConfig{}, nil)
	router.AddOverride("search*", secondExec, &adapter.EnvironmentConfig{}, nil)

	// Later registrations (.aat-overrides.yaml, --overlay, --override) beat
	// earlier ones (env.yaml overrides).
	exec, _, _ := router.Resolve("searchFlights")
	assert.Equal(t, secondExec, exec)
}

func TestExecutorRouter_HasOverrides(t *testing.T) {
	router := NewExecutorRouter(adapter.NewHTTPExecutor("https://default.example.com"), &adapter.EnvironmentConfig{})
	assert.False(t, router.HasOverrides())

	router.AddOverride("node1", adapter.NewHTTPExecutor("http://localhost"), &adapter.EnvironmentConfig{}, nil)
	assert.True(t, router.HasOverrides())
}

func TestExecutorRouter_OverridePatterns(t *testing.T) {
	router := NewExecutorRouter(adapter.NewHTTPExecutor("https://default.example.com"), &adapter.EnvironmentConfig{})
	router.AddOverride("searchFlights", adapter.NewHTTPExecutor("http://localhost:8080"), &adapter.EnvironmentConfig{}, nil)
	router.AddOverride("price*", adapter.NewHTTPExecutor("http://localhost:8081"), &adapter.EnvironmentConfig{}, nil)

	patterns := router.OverridePatterns()
	assert.Equal(t, []string{"searchFlights", "price*"}, patterns)
}

func TestIsGlobPattern(t *testing.T) {
	assert.True(t, isGlobPattern("price*"))
	assert.True(t, isGlobPattern("node?"))
	assert.True(t, isGlobPattern("[abc]"))
	assert.False(t, isGlobPattern("exactMatch"))
	assert.False(t, isGlobPattern("searchFlights"))
}

func TestExecutorRouter_QuestionMarkGlob(t *testing.T) {
	defaultExec := adapter.NewHTTPExecutor("https://default.example.com")
	overrideExec := adapter.NewHTTPExecutor("http://localhost:8080")

	router := NewExecutorRouter(defaultExec, &adapter.EnvironmentConfig{})
	router.AddOverride("step?", overrideExec, &adapter.EnvironmentConfig{}, nil)

	exec, _, _ := router.Resolve("step1")
	assert.Equal(t, overrideExec, exec)

	exec, _, _ = router.Resolve("step2")
	assert.Equal(t, overrideExec, exec)

	exec, _, _ = router.Resolve("step12") // two chars after "step" — no match
	assert.Equal(t, defaultExec, exec)
}

// TestEngine_Run_RoutingToMultipleServers verifies that a 2-step plan routes
// each step to a different HTTP server based on override configuration.
func TestEngine_Run_RoutingToMultipleServers(t *testing.T) {
	g := &graph.Graph{
		Version: "1.0.0",
		Nodes: map[string]*graph.Node{
			"search": {
				Name:    "search",
				Adapter: "test.search",
				Inputs:  []graph.Input{{Name: "query", Type: "string"}},
				Outputs: []graph.Output{{Name: "resultId", Type: "string"}},
			},
			"book": {
				Name:    "book",
				Adapter: "test.book",
				Inputs:  []graph.Input{{Name: "resultId", Type: "string"}},
				Outputs: []graph.Output{{Name: "bookingId", Type: "string"}},
			},
		},
	}

	// Server 1: search service (local override)
	var searchHost string
	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"resultId": "found-123"})
	}))
	defer searchServer.Close()

	// Server 2: booking service (default)
	var bookHost string
	bookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bookHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"bookingId": "book-456"})
	}))
	defer bookServer.Close()

	registry := adapter.NewRegistry()
	require.NoError(t, registry.Register("test.search", &stubAdapter{
		method:   "POST",
		path:     "/search",
		response: map[string]any{"resultId": "found-123"},
	}))
	require.NoError(t, registry.Register("test.book", &stubAdapter{
		method:   "POST",
		path:     "/book",
		response: map[string]any{"bookingId": "book-456"},
	}))

	// Default executor points to booking server
	defaultExec := adapter.NewHTTPExecutor(bookServer.URL)
	defaultCfg := &adapter.EnvironmentConfig{BaseURL: bookServer.URL}

	// Override: search goes to search server
	searchExec := adapter.NewHTTPExecutor(searchServer.URL)
	searchCfg := &adapter.EnvironmentConfig{BaseURL: searchServer.URL}

	router := NewExecutorRouter(defaultExec, defaultCfg)
	router.AddOverride("search", searchExec, searchCfg, nil)

	eng := NewEngine(g, registry, router)

	p := &plan.Plan{
		Metadata: plan.Metadata{GraphVersion: "1.0.0"},
		Execution: plan.Execution{
			Steps: []plan.Step{
				{Node: "search", Values: map[string]plan.StepValue{"query": {Default: "test"}}},
				{Node: "book", DependsOn: []string{"search"}, Values: map[string]plan.StepValue{
					"resultId": {From: "search.resultId"},
				}},
			},
		},
	}

	result := eng.Run(context.Background(), p)

	assert.Equal(t, OutcomePassed, result.Outcome)
	assert.Nil(t, result.Error)
	require.Len(t, result.Steps, 2)

	// Verify the two steps hit different servers
	assert.NotEmpty(t, searchHost)
	assert.NotEmpty(t, bookHost)
	assert.NotEqual(t, searchHost, bookHost, "search and book should hit different servers")
}

// TestEngine_Run_PathRewriteIntegration verifies that path rewriting works end-to-end.
func TestEngine_Run_PathRewriteIntegration(t *testing.T) {
	g := &graph.Graph{
		Version: "1.0.0",
		Nodes: map[string]*graph.Node{
			"search": {
				Name:    "search",
				Adapter: "test.search",
				Inputs:  []graph.Input{{Name: "query", Type: "string"}},
				Outputs: []graph.Output{{Name: "id", Type: "string"}},
			},
		},
	}

	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	registry := adapter.NewRegistry()
	require.NoError(t, registry.Register("test.search", &stubAdapter{
		method:   "POST",
		path:     "/11/search/flights", // template path includes /11 prefix
		response: map[string]any{"id": "x"},
	}))

	exec := adapter.NewHTTPExecutor(server.URL)
	router := NewExecutorRouter(exec, &adapter.EnvironmentConfig{})
	router.AddOverride("search", exec, &adapter.EnvironmentConfig{},
		&adapter.PathRewrite{Strip: "/11", Prefix: "/api/v2"})

	eng := NewEngine(g, registry, router)

	p := &plan.Plan{
		Metadata: plan.Metadata{GraphVersion: "1.0.0"},
		Execution: plan.Execution{
			Steps: []plan.Step{
				{Node: "search", Values: map[string]plan.StepValue{"query": {Default: "test"}}},
			},
		},
	}

	result := eng.Run(context.Background(), p)
	assert.Equal(t, OutcomePassed, result.Outcome)

	// Path should be rewritten: /11/search/flights → /api/v2/search/flights
	assert.Equal(t, "/api/v2/search/flights", receivedPath)
}

func TestExecutorRouter_LastRegisteredOverrideWins(t *testing.T) {
	defaultExec := adapter.NewHTTPExecutor("https://default.example.com")
	envGlob := adapter.NewHTTPExecutor("https://env.example.com")
	overlayGlob := adapter.NewHTTPExecutor("http://localhost:9091")
	exact := adapter.NewHTTPExecutor("http://localhost:9092")
	laterExact := adapter.NewHTTPExecutor("http://localhost:9093")

	router := NewExecutorRouter(defaultExec, &adapter.EnvironmentConfig{})
	router.AddOverride("payment*", envGlob, &adapter.EnvironmentConfig{}, nil)
	router.AddOverride("payment*", overlayGlob, &adapter.EnvironmentConfig{}, nil)

	exec, _, _ := router.Resolve("paymentCharge")
	assert.Equal(t, overlayGlob, exec, "a later glob beats an earlier glob")

	router.AddOverride("paymentCharge", exact, &adapter.EnvironmentConfig{}, nil)
	router.AddOverride("payment*", envGlob, &adapter.EnvironmentConfig{}, nil)
	exec, _, _ = router.Resolve("paymentCharge")
	assert.Equal(t, exact, exec, "an exact match beats any glob, even a later one")

	router.AddOverride("paymentCharge", laterExact, &adapter.EnvironmentConfig{}, nil)
	exec, _, _ = router.Resolve("paymentCharge")
	assert.Equal(t, laterExact, exec, "a later exact match beats an earlier exact match")

	exec, _, _ = router.Resolve("paymentRefund")
	assert.Equal(t, envGlob, exec, "glob still applies to other nodes")
}

func TestExecutorRouter_ValueOverrideLastExpectFailureWins(t *testing.T) {
	router := NewExecutorRouter(adapter.NewHTTPExecutor("https://default.example.com"), &adapter.EnvironmentConfig{})
	router.AddValueOverride("payment*", map[string]any{"cardNumber": "1"}, &plan.ExpectFailure{Status: []int{402}})
	router.AddValueOverride("payment*", map[string]any{"cardNumber": "2"}, &plan.ExpectFailure{Status: []int{409}})

	values, ef := router.ResolveValueOverride("paymentCharge")
	assert.Equal(t, "2", values["cardNumber"])
	require.NotNil(t, ef)
	assert.Equal(t, []int{409}, ef.Status, "later glob wins")

	router.AddValueOverride("paymentCharge", nil, &plan.ExpectFailure{Status: []int{422}})
	_, ef = router.ResolveValueOverride("paymentCharge")
	assert.Equal(t, []int{422}, ef.Status, "exact beats glob")
}
