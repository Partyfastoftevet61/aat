package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/graph/oas"
	"github.com/gburgyan/aat/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const strictTestSpec = `openapi: "3.0.3"
info: {title: strict, version: "1"}
paths:
  /pets:
    post:
      operationId: createPet
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name: {type: string}
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                type: object
                required: [id, name]
                properties:
                  id: {type: integer}
                  name: {type: string}
`

// strictEngine builds an engine whose only node is OAS-bound to createPet and
// whose server answers with the given status and body.
func strictEngine(t *testing.T, strict bool, status int, body string) (*Engine, *plan.Plan) {
	t.Helper()
	specPath := filepath.Join(t.TempDir(), "spec.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte(strictTestSpec), 0o644))
	cache := oas.NewSpecCache()
	require.NoError(t, cache.Load("spec.yaml", specPath))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	g := &graph.Graph{
		Version: "1.0.0",
		Nodes: map[string]*graph.Node{
			"createPet": {
				Name: "createPet", Adapter: "test.createPet",
				OAS:     &graph.OASRef{OperationID: "createPet", Spec: "spec.yaml"},
				Inputs:  []graph.Input{{Name: "name", Type: "string"}},
				Outputs: []graph.Output{{Name: "petName", Type: "string"}},
			},
		},
	}
	registry := adapter.NewRegistry()
	require.NoError(t, registry.Register("test.createPet", &stubAdapter{method: "POST", path: "/pets", response: map[string]any{"petName": "Fido"}}))
	eng := NewEngine(g, registry, NewExecutorRouter(adapter.NewHTTPExecutor(server.URL), &adapter.EnvironmentConfig{})).
		WithOASSpecs(cache, "", strict)

	p := &plan.Plan{
		Metadata: plan.Metadata{GraphVersion: "1.0.0"},
		Execution: plan.Execution{Steps: []plan.Step{{
			Node:   "createPet",
			Values: map[string]plan.StepValue{"name": {Default: "Fido"}},
		}}},
	}
	return eng, p
}

func TestOASStrict_ViolationFailsStep(t *testing.T) {
	// 201 body is missing the required "id" field.
	eng, p := strictEngine(t, true, http.StatusCreated, `{"name": "Fido"}`)
	result := eng.Run(context.Background(), p)

	assert.Equal(t, OutcomeFailed, result.Outcome)
	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "strict mode")
	require.Len(t, result.Steps, 1)
	require.NotNil(t, result.Steps[0].OASValidation)
	assert.True(t, result.Steps[0].OASValidation.HasErrors())
}

func TestOASStrict_WarnModeOnlyReports(t *testing.T) {
	eng, p := strictEngine(t, false, http.StatusCreated, `{"name": "Fido"}`)
	result := eng.Run(context.Background(), p)

	assert.Equal(t, OutcomePassed, result.Outcome, "error: %v", result.Error)
	require.Len(t, result.Steps, 1)
	require.NotNil(t, result.Steps[0].OASValidation)
	assert.True(t, result.Steps[0].OASValidation.HasErrors(), "violation is still recorded")
}

func TestOASStrict_ExpectFailureStepsAreExempt(t *testing.T) {
	// A 400 with a body the spec does not describe is an OAS violation, but the
	// step expects that failure, so strict mode must not turn it into a red run.
	eng, p := strictEngine(t, true, http.StatusBadRequest, `{"oops": true}`)
	p.Execution.Steps[0].ExpectFailure = &plan.ExpectFailure{Status: []int{400}}
	result := eng.Run(context.Background(), p)

	assert.Equal(t, OutcomePassed, result.Outcome, "error: %v", result.Error)
}
