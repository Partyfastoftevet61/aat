package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEchoInputOutputs(t *testing.T) {
	node := &graph.Node{Outputs: []graph.Output{
		{Name: "created", Type: "boolean"},
		{Name: "collectionName", Type: "string", FromInput: "collectionName"},
		{Name: "shardKey", Type: "string", FromInput: "shardKey"},
	}}

	tests := []struct {
		name    string
		outputs map[string]any
		inputs  map[string]any
		want    map[string]any
	}{
		{
			name:    "an echoed output joins the extracted ones",
			outputs: map[string]any{"created": true},
			inputs:  map[string]any{"collectionName": "aat-qdrant-1a2b", "shardKey": "eu"},
			want:    map[string]any{"created": true, "collectionName": "aat-qdrant-1a2b", "shardKey": "eu"},
		},
		{
			name:    "an input the step left out leaves its output unset",
			outputs: map[string]any{"created": true},
			inputs:  map[string]any{"collectionName": "aat-qdrant-1a2b"},
			want:    map[string]any{"created": true, "collectionName": "aat-qdrant-1a2b"},
		},
		{
			name:    "the echo wins over an output of the same name",
			outputs: map[string]any{"collectionName": "from the response"},
			inputs:  map[string]any{"collectionName": "as sent"},
			want:    map[string]any{"collectionName": "as sent"},
		},
		{
			name:    "nil outputs are made when something is echoed",
			outputs: nil,
			inputs:  map[string]any{"collectionName": "aat-qdrant-1a2b"},
			want:    map[string]any{"collectionName": "aat-qdrant-1a2b"},
		},
		{
			name:    "nil outputs stay nil when nothing is echoed",
			outputs: nil,
			inputs:  map[string]any{},
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, echoInputOutputs(tt.outputs, node, tt.inputs))
		})
	}
}

// A create whose reply doesn't name what it created: the name the step sent is
// what a later step and the cleanup read, through the echoed output.
func TestEngine_Run_EchoedOutputReachesLaterStepsAndCleanup(t *testing.T) {
	g := &graph.Graph{
		Version: "1.0.0",
		Nodes: map[string]*graph.Node{
			"createCollection": {
				Name:    "createCollection",
				Adapter: "createCollection",
				Inputs:  []graph.Input{{Name: "collectionName", Type: "string"}},
				Outputs: []graph.Output{
					{Name: "created", Type: "boolean"},
					{Name: "collectionName", Type: "string", FromInput: "collectionName"},
				},
				Cleanup: graph.CleanupPairing{Node: "deleteCollection"},
			},
			"getCollection": {
				Name:    "getCollection",
				Adapter: "getCollection",
				Inputs: []graph.Input{{
					Name: "collectionName", Type: "string",
					Default: &graph.InputDefault{From: "createCollection.collectionName"},
				}},
				Outputs: []graph.Output{{Name: "status", Type: "string"}},
			},
			"deleteCollection": {
				Name:    "deleteCollection",
				Adapter: "deleteCollection",
				Inputs:  []graph.Input{{Name: "collectionName", Type: "string"}},
			},
		},
	}

	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path+" "+string(body))
		mu.Unlock()
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"result":{"status":"green"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":true}`))
	}))
	defer server.Close()

	registry := adapter.NewRegistry()
	require.NoError(t, registry.Register("createCollection", adapter.NewTemplateAdapter(adapter.Template{
		Adapter:  "createCollection",
		Request:  adapter.TemplateRequest{Method: "PUT", Path: "/collections/{{collectionName}}", Body: `{"size": 4}`},
		Response: adapter.TemplateResponse{Extract: map[string]adapter.ExtractRule{"created": {Path: "result"}}},
	})))
	require.NoError(t, registry.Register("getCollection", adapter.NewTemplateAdapter(adapter.Template{
		Adapter:  "getCollection",
		Request:  adapter.TemplateRequest{Method: "GET", Path: "/collections/{{collectionName}}"},
		Response: adapter.TemplateResponse{Extract: map[string]adapter.ExtractRule{"status": {Path: "result.status"}}},
	})))
	require.NoError(t, registry.Register("deleteCollection", adapter.NewTemplateAdapter(adapter.Template{
		Adapter: "deleteCollection",
		Request: adapter.TemplateRequest{Method: "DELETE", Path: "/collections/{{collectionName}}"},
	})))

	eng := NewEngine(g, registry, NewExecutorRouter(adapter.NewHTTPExecutor(server.URL), &adapter.EnvironmentConfig{}))
	p := &plan.Plan{
		Metadata: plan.Metadata{GraphVersion: "1.0.0"},
		Execution: plan.Execution{Steps: []plan.Step{
			{ID: "create", Node: "createCollection", Values: map[string]plan.StepValue{
				"collectionName": {Default: "aat-qdrant-1a2b"},
			}},
			{ID: "read", Node: "getCollection", DependsOn: []string{"create"}},
		}},
	}

	result := eng.Run(context.Background(), p)

	require.Equal(t, OutcomePassed, result.Outcome, "run error: %v", result.Error)
	require.Len(t, result.Steps, 2)
	assert.Equal(t, map[string]any{"created": true, "collectionName": "aat-qdrant-1a2b"}, result.Steps[0].Outputs)
	require.Len(t, result.CleanupResults, 1)
	assert.NoError(t, result.CleanupResults[0].Error)
	assert.Equal(t, []string{
		`PUT /collections/aat-qdrant-1a2b {"size": 4}`,
		`GET /collections/aat-qdrant-1a2b `,
		`DELETE /collections/aat-qdrant-1a2b `,
	}, requests)
}

func TestOutputExtractPaths_EchoedOutput(t *testing.T) {
	g := &graph.Graph{Nodes: map[string]*graph.Node{
		"createCollection": {Name: "createCollection", Adapter: "createCollection", Outputs: []graph.Output{
			{Name: "created"}, {Name: "collectionName", FromInput: "collectionName"},
		}},
	}}
	registry := adapter.NewRegistry()
	require.NoError(t, registry.Register("createCollection", adapter.NewTemplateAdapter(adapter.Template{
		Adapter:  "createCollection",
		Response: adapter.TemplateResponse{Extract: map[string]adapter.ExtractRule{"created": {Path: "result"}}},
	})))

	want := map[string]map[string]string{"createCollection": {"created": "result", "collectionName": ""}}
	assert.Equal(t, want, map[string]map[string]string(OutputExtractPaths(g, registry)), "an echoed output has no response location to check")
}

func TestValidateAdapterOutputs_EchoedOutput(t *testing.T) {
	node := func() *graph.Node {
		return &graph.Node{Name: "createCollection", Adapter: "createCollection", Outputs: []graph.Output{
			{Name: "created", Type: "boolean"},
			{Name: "collectionName", Type: "string", FromInput: "collectionName"},
		}}
	}
	register := func(t *testing.T, extract map[string]adapter.ExtractRule) *adapter.Registry {
		t.Helper()
		registry := adapter.NewRegistry()
		require.NoError(t, registry.Register("createCollection", adapter.NewTemplateAdapter(adapter.Template{
			Adapter:  "createCollection",
			Response: adapter.TemplateResponse{Extract: extract},
		})))
		return registry
	}

	t.Run("needs no extract rule", func(t *testing.T) {
		g := &graph.Graph{Nodes: map[string]*graph.Node{"createCollection": node()}}
		assert.NoError(t, ValidateAdapterOutputs(g, register(t, map[string]adapter.ExtractRule{"created": {Path: "result"}})))
	})

	t.Run("an extract rule for it too is an error", func(t *testing.T) {
		g := &graph.Graph{Nodes: map[string]*graph.Node{"createCollection": node()}}
		err := ValidateAdapterOutputs(g, register(t, map[string]adapter.ExtractRule{
			"created":        {Path: "result"},
			"collectionName": {Path: "name"},
		}))
		require.Error(t, err)
		assert.Equal(t, "adapter output validation failed:\n"+
			`  - node "createCollection": output "collectionName" is echoed from input "collectionName", so the template must not also extract it`, err.Error())
	})
}
