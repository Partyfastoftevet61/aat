package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
)

// runValueRefs runs a checkout that totals 13066, with an optional coupon its
// response leaves out, then a refund whose amount is the given value.
func runValueRefs(t *testing.T, amount string) *RunResult {
	t.Helper()
	g := &graph.Graph{Version: "1.0.0", Nodes: map[string]*graph.Node{
		"checkoutCart": {Name: "checkoutCart", Adapter: "values.checkout", Outputs: []graph.Output{
			{Name: "total", Type: "integer"},
			{Name: "coupon", Type: "integer", Optional: true},
		}},
		"paymentRefund": {Name: "paymentRefund", Adapter: "values.refund",
			Inputs:  []graph.Input{{Name: "amount", Type: "integer"}},
			Outputs: []graph.Output{{Name: "status", Type: "string"}},
		},
	}}
	srv := newChainServer(t, nil)
	registry := adapter.NewRegistry()
	require.NoError(t, registry.Register("values.checkout", &stubAdapter{method: "POST", path: "/checkout", response: map[string]any{"total": 13066}}))
	require.NoError(t, registry.Register("values.refund", &stubAdapter{method: "POST", path: "/refunds", response: map[string]any{"status": "refunded"}}))
	eng := NewEngine(g, registry, NewExecutorRouter(adapter.NewHTTPExecutor(srv.URL), &adapter.EnvironmentConfig{}))

	return eng.Run(context.Background(), &plan.Plan{
		Metadata: plan.Metadata{GraphVersion: "1.0.0"},
		Execution: plan.Execution{Steps: []plan.Step{
			{ID: "checkout", Node: "checkoutCart"},
			{ID: "refund", Node: "paymentRefund", DependsOn: []string{"checkout"}, Values: map[string]plan.StepValue{"amount": {Default: amount}}},
		}},
	})
}

func TestStepValues_ReadEarlierStepOutputs(t *testing.T) {
	t.Run("with an offset", func(t *testing.T) {
		result := runValueRefs(t, "{{checkout.total - 66}}")

		require.Equal(t, OutcomePassed, result.Outcome, "error: %v", result.Error)
		assert.EqualValues(t, 13000, result.Steps[1].Inputs["amount"])
	})

	t.Run("an output the node doesn't declare fails validation", func(t *testing.T) {
		result := runValueRefs(t, "{{checkout.refunded - 66}}")

		require.NotEqual(t, OutcomePassed, result.Outcome)
		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), `reads {{checkout.refunded}}`)
		assert.Empty(t, result.Steps, "nothing is sent")
	})

	t.Run("an optional output the response left out fails the step", func(t *testing.T) {
		result := runValueRefs(t, "{{checkout.coupon - 66}}")

		require.NotEqual(t, OutcomePassed, result.Outcome)
		require.Len(t, result.Steps, 2)
		require.Error(t, result.Steps[1].Error)
		assert.Contains(t, result.Steps[1].Error.Error(), `step "checkout" produced no output "coupon"`)
	})
}
