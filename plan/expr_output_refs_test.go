package plan

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// outputsLookup returns an ExprContext.Outputs that reads the given steps'
// outputs.
func outputsLookup(outputs map[string]map[string]any) func(stepID, output string) (any, error) {
	return func(stepID, output string) (any, error) {
		step, ok := outputs[stepID]
		if !ok {
			return nil, errors.New("no outputs for " + stepID)
		}
		v, ok := step[output]
		if !ok {
			return nil, errors.New("no output " + output)
		}
		return v, nil
	}
}

func TestEvalExpr_OutputRefs(t *testing.T) {
	lookup := outputsLookup(map[string]map[string]any{
		"checkout": {
			"total":    json.Number("13066"),
			"subtotal": json.Number("120.5"),
			"currency": "USD",
			"paid":     true,
			"lines":    []any{"a"},
			"coupon":   nil,
			"price":    "221.78",
			"frozen":   json.Number("1789430400"),
			"shipOn":   "2026-03-01",
		},
	})
	withOutputs := ExprContext{Env: testEnv, Outputs: lookup}
	tests := []struct {
		name    string
		raw     string
		ctx     ExprContext
		want    any
		wantErr string
	}{
		{name: "a whole number keeps its type", raw: "{{checkout.total}}", ctx: withOutputs, want: int64(13066)},
		{name: "a decimal number", raw: "{{checkout.subtotal}}", ctx: withOutputs, want: 120.5},
		{name: "text", raw: "{{checkout.currency}}", ctx: withOutputs, want: "USD"},
		{name: "a boolean, with spaces", raw: "{{ checkout.paid }}", ctx: withOutputs, want: true},
		{name: "in mixed text", raw: "{{checkout.total}} {{checkout.currency}}", ctx: withOutputs, want: "13066 USD"},
		{name: "env still reads the environment", raw: "{{env.MY_VAR}}", ctx: withOutputs, want: "my-value"},
		{
			name:    "where no step outputs can be read",
			raw:     "{{checkout.total}}",
			wantErr: "{{checkout.total}} reads a step's output, which can't be read here",
		},
		{name: "a null output", raw: "{{checkout.coupon}}", ctx: withOutputs, wantErr: `step "checkout" output "coupon" is null`},
		{name: "a list output", raw: "{{checkout.lines}}", ctx: withOutputs, wantErr: `step "checkout" output "lines" is a list`},
		{name: "a lookup error", raw: "{{cart.total}}", ctx: withOutputs, wantErr: "no outputs for cart"},
		{name: "a whole number minus a whole number", raw: "{{checkout.total - 66}}", ctx: withOutputs, want: int64(13000)},
		{name: "a decimal number plus a decimal", raw: "{{checkout.subtotal + 0.25}}", ctx: withOutputs, want: 120.75},
		{name: "decimal text keeps its places", raw: "{{checkout.price - 21.78}}", ctx: withOutputs, want: "200.00"},
		{name: "decimal text plus a whole number", raw: "{{checkout.price + 1}}", ctx: withOutputs, want: "222.78"},
		{name: "Unix seconds plus days", raw: "{{checkout.frozen + 32 days}}", ctx: withOutputs, want: int64(1789430400 + 32*86400)},
		{name: "a date plus days", raw: "{{checkout.shipOn + 3 days}}", ctx: withOutputs, want: "2026-03-04"},
		{name: "an offset in mixed text", raw: "refund {{checkout.total - 13000}} of {{checkout.total}}", ctx: withOutputs, want: "refund 66 of 13066"},
		{name: "text is not a number", raw: "{{checkout.currency + 1}}", ctx: withOutputs, wantErr: `{{checkout.currency}} value "USD" is not a number`},
		{name: "a boolean is not a number", raw: "{{checkout.paid - 1}}", ctx: withOutputs, wantErr: "{{checkout.paid}} is bool, not a number"},
		{name: "a hyphenated step ID", raw: "{{pay--declined.total}}", ctx: withOutputs, wantErr: "step IDs in {{step.output}} use letters, digits, and underscores"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvalExpr(tt.raw, tt.ctx)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExprOutputRefs(t *testing.T) {
	assert.Equal(t, []OutputRef{{Step: "checkout", Output: "total"}, {Step: "getCart", Output: "subtotal"}},
		ExprOutputRefs("{{checkout.total}} of {{ getCart.subtotal }} on {{today}} from {{env.HOME}} for {{quantity}}"))
	assert.Equal(t, []OutputRef{{Step: "authorize", Output: "amount"}, {Step: "clock", Output: "frozenTime"}},
		ExprOutputRefs("{{authorize.amount - 500}} until {{ clock.frozenTime + 32 days }} for {{quantity + 1}}"),
		"a reference with an offset is a reference; an input's offset isn't")
	assert.Nil(t, ExprOutputRefs("no expressions"))
	assert.Equal(t, []OutputRef{{Step: "a", Output: "b"}}, ExprValueOutputRefs([]any{"x", map[string]any{"k": "{{a.b}}"}}))
	assert.Equal(t, "{{checkout.total}}", OutputRef{Step: "checkout", Output: "total"}.String())
	assert.Equal(t, []OutputRef{{Step: "price", Output: "totalAmount"}}, PredicateOutputRefs(`totalAmount == "{{price.totalAmount}}" && n > 0`))
}

func TestRewriteExprRefs(t *testing.T) {
	idMap := map[string]string{"price": "inc0_price", "env": "inc0_env"}

	got := RewriteExprRefs(`totalAmount == "{{price.totalAmount}}" && owner == "{{offer.owner}}" && key == "{{env.KEY}}"`, idMap)

	assert.Equal(t, `totalAmount == "{{inc0_price.totalAmount}}" && owner == "{{offer.owner}}" && key == "{{env.KEY}}"`, got,
		"a step idMap names is renamed; another step and the environment are left as they are")
	assert.Equal(t, "{{inc0_price.totalAmount - 10.50}} by {{inc0_price.dueAt + 2 days}}",
		RewriteExprRefs("{{price.totalAmount - 10.50}} by {{ price.dueAt + 2 days }}", idMap),
		"an offset stays with its reference")
}

func TestRewriteAssertionRefs(t *testing.T) {
	orig := &Assertions{Mechanical: []MechanicalAssertion{
		{Type: "predicate", Expr: `total == "{{checkout.total}}"`},
		{Type: "fieldEquals", Path: "amount", Value: "{{checkout.total}}"},
		{Type: "status", Expect: []any{200}},
	}}

	got := RewriteAssertionRefs(orig, map[string]string{"checkout": "inc0_checkout"})

	assert.Equal(t, `total == "{{inc0_checkout.total}}"`, got.Mechanical[0].Expr)
	assert.Equal(t, "{{inc0_checkout.total}}", got.Mechanical[1].Value)
	assert.Equal(t, `total == "{{checkout.total}}"`, orig.Mechanical[0].Expr, "the original assertions are left as they were")
	assert.Same(t, orig, RewriteAssertionRefs(orig, map[string]string{"other": "renamed"}), "nothing to rename returns the same assertions")
}

func TestRewriteStepRefs_AssertionsAndRepeat(t *testing.T) {
	s := Step{
		ID:         "price",
		Assertions: &Assertions{Mechanical: []MechanicalAssertion{{Type: "predicate", Expr: `n == "{{search.count}}"`}}},
		Repeat:     &RepeatConfig{Until: `n >= "{{search.count}}"`},
	}

	rewriteStepRefs(&s, map[string]string{"search": "search__declined"})

	assert.Equal(t, `n == "{{search__declined.count}}"`, s.Assertions.Mechanical[0].Expr)
	assert.Equal(t, `n >= "{{search__declined.count}}"`, s.Repeat.Until)
}

func TestRewriteStepRefs_SelectionFilters(t *testing.T) {
	shared := &SelectionConfig{Strategy: "first", Field: "id", Filter: `objectId == "{{create.customerId}}"`}
	s := Step{
		ID:         "event",
		Values:     map[string]StepValue{"eventId": {From: "events.events", Select: shared}},
		Selections: map[string]StepSelection{"about": {From: "events.events", Filter: `objectId == "{{create.customerId}}"`}},
	}

	rewriteStepRefs(&s, map[string]string{"create": "inc0_create", "events": "inc0_events"})

	assert.Equal(t, "inc0_events.events", s.Values["eventId"].From)
	assert.Equal(t, `objectId == "{{inc0_create.customerId}}"`, s.Values["eventId"].Select.Filter)
	assert.Equal(t, `objectId == "{{inc0_create.customerId}}"`, s.Selections["about"].Filter)
	assert.Equal(t, `objectId == "{{create.customerId}}"`, shared.Filter, "a select block another step shares is left as it was")
}
