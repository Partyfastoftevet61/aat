package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanListCommand_SummarizesRecipesAndPlans(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "negative"), 0o755))
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	write("checkout.yaml", `kind: recipe
selection:
  workflow: Checkout
  layers: [shipping-express]
  choices:
    payment: PayPal
    customer: Registered
  addons: [Apply Coupon]
`)
	write("smoke.yaml", `kind: recipe
selection:
  workflow: Quick Purchase
`)
	write("negative/state-machine.yaml", `intent:
  goal: ship
execution:
  steps:
    - id: cart
      node: createCart
    - id: ship
      node: shipOrder
`)
	write("broken.yaml", "execution: [")

	var out bytes.Buffer
	require.NoError(t, planListCommand([]string{dir}, &out))
	got := out.String()

	assert.Contains(t, got, "Found 4 plan(s)")
	assert.Contains(t, got, "recipe    Checkout (customer=Registered, payment=PayPal; +Apply Coupon; layers: shipping-express)")
	assert.Contains(t, got, "recipe    Quick Purchase\n")
	assert.Contains(t, got, "2 steps  ship")
	assert.Regexp(t, `broken\.yaml\s+\(parse error\)`, got)
}
