package intent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tpGraphPathCompose = "../airline/graph.yaml"
const tpGraphDirCompose = "../airline"

func loadAirlineForCompose(t *testing.T) *graph.Graph {
	t.Helper()
	g, err := graph.ParseFile(tpGraphPathCompose)
	require.NoError(t, err)
	return g
}

// TestCompose_FullWithSeatSelection verifies that composing the
// Full Booking with the Seat Selection addon produces a valid plan
// with the seat steps spliced in at the right place.
func TestCompose_FullWithSeatSelection(t *testing.T) {
	g := loadAirlineForCompose(t)

	// Find the base workflow.
	var base graph.Workflow
	for _, w := range g.Workflows {
		if w.Name == "Full Booking" {
			base = w
			break
		}
	}
	require.NotEmpty(t, base.Name, "base workflow not found in graph")

	// Compose with Seat Selection addon.
	p, err := ComposeWithAddons(base, []string{"Seat Selection"}, g, tpGraphDirCompose)
	require.NoError(t, err)

	// Validate the composed plan against the graph.
	// Seat Selection addon has intentionally unfed value inputs (offerId,
	// productId on searchSeatMap) that the LLM provides at runtime.
	err = plan.Validate(p, g)
	if err != nil {
		errStr := err.Error()
		assert.Contains(t, errStr, "offerId", "unexpected validation error")
		assert.Contains(t, errStr, "productId", "unexpected validation error")
	}

	// Check step count: parent has 8 steps, addon has 2 = 10 total.
	require.Len(t, p.Execution.Steps, 10, "expected 8 parent + 2 addon steps")

	// Verify step order: seat steps should appear after priceOfferFull
	// (that's the After node for Seat Selection).
	stepIDs := make([]string, len(p.Execution.Steps))
	for i, s := range p.Execution.Steps {
		stepIDs[i] = s.StepID()
	}
	t.Logf("Step order: %s", strings.Join(stepIDs, " → "))

	// Find insertion point and seat step indices.
	priceIdx := indexOf(stepIDs, "priceOfferFull")
	seatSearchIdx := indexOf(stepIDs, "inc0_searchSeatMap")
	seatAddIdx := indexOf(stepIDs, "inc0_addSeatOffer")
	require.Greater(t, priceIdx, -1, "priceOfferFull not found")
	require.Greater(t, seatSearchIdx, -1, "inc0_searchSeatMap not found")
	require.Greater(t, seatAddIdx, -1, "inc0_addSeatOffer not found")

	assert.Greater(t, seatSearchIdx, priceIdx, "seat search should be after priceOfferFull")
	assert.Greater(t, seatAddIdx, seatSearchIdx, "addSeat should be after seat search")

	// Verify seat steps depend on priceOfferFull.
	seatSearch := p.Execution.Steps[seatSearchIdx]
	assert.Contains(t, seatSearch.DependsOn, "priceOfferFull", "seat search should depend on priceOfferFull")

	// Verify explicit Wire override: offerListIdentifier should be wired
	// from priceOfferFull.offerRef via the addon's Wire config.
	oliValue, hasOLI := seatSearch.Values["offerListIdentifier"]
	require.True(t, hasOLI, "offerListIdentifier should exist in seat search values")
	assert.Equal(t, "priceOfferFull.offerRef", oliValue.From,
		"offerListIdentifier should be wired via addon Wire config")

	// Cleanup should still have ignoreItinerary.
	require.NotEmpty(t, p.Execution.Cleanup)
	assert.Equal(t, "ignoreItinerary", p.Execution.Cleanup[0].Node)
}

// TestCompose_FullWithAncillaries verifies ancillary addon composition.
func TestCompose_FullWithAncillaries(t *testing.T) {
	g := loadAirlineForCompose(t)

	var base graph.Workflow
	for _, w := range g.Workflows {
		if w.Name == "Full Booking" {
			base = w
			break
		}
	}
	require.NotEmpty(t, base.Name)

	p, err := ComposeWithAddons(base, []string{"Ancillary Booking"}, g, tpGraphDirCompose)
	require.NoError(t, err)

	err = plan.Validate(p, g)
	assert.NoError(t, err, "composed plan should validate")

	require.Len(t, p.Execution.Steps, 10, "expected 8 parent + 2 ancillary steps")

	// Verify ancillary steps present.
	stepIDs := collectStepIDs(p)
	t.Logf("Step order: %s", strings.Join(stepIDs, " → "))

	assert.Contains(t, stepIDs, "inc0_searchAncillaries")
	assert.Contains(t, stepIDs, "inc0_addAncillaryOffer")

	// Verify auto-wiring of itineraryId.
	ancSearch := findStep(p, "inc0_searchAncillaries")
	require.NotNil(t, ancSearch)
	wbValue, hasWB := ancSearch.Values["itineraryId"]
	require.True(t, hasWB)
	assert.NotEmpty(t, wbValue.From, "itineraryId should be auto-wired")
	t.Logf("ancillary itineraryId wired to: %s", wbValue.From)
}

// TestCompose_FullWithSeatAndAncillaries verifies double-addon composition.
func TestCompose_FullWithSeatAndAncillaries(t *testing.T) {
	g := loadAirlineForCompose(t)

	var base graph.Workflow
	for _, w := range g.Workflows {
		if w.Name == "Full Booking" {
			base = w
			break
		}
	}
	require.NotEmpty(t, base.Name)

	p, err := ComposeWithAddons(base, []string{"Seat Selection", "Ancillary Booking"}, g, tpGraphDirCompose)
	require.NoError(t, err)

	// Seat Selection addon has intentionally unfed value inputs.
	err = plan.Validate(p, g)
	if err != nil {
		errStr := err.Error()
		assert.Contains(t, errStr, "offerId", "unexpected validation error")
		assert.Contains(t, errStr, "productId", "unexpected validation error")
	}

	// 8 parent + 2 seat + 2 ancillary = 12 steps.
	require.Len(t, p.Execution.Steps, 12, "expected 8 parent + 2 seat + 2 ancillary steps")

	stepIDs := collectStepIDs(p)
	t.Logf("Step order: %s", strings.Join(stepIDs, " → "))

	// Both addon sets should be present.
	assert.Contains(t, stepIDs, "inc0_searchSeatMap")
	assert.Contains(t, stepIDs, "inc0_addSeatOffer")
	assert.Contains(t, stepIDs, "inc1_searchAncillaries")
	assert.Contains(t, stepIDs, "inc1_addAncillaryOffer")
}

// TestCompose_DynamicWithTravelerModification verifies dynamic composition.
func TestCompose_DynamicWithTravelerModification(t *testing.T) {
	g := loadAirlineForCompose(t)

	// Find base workflow.
	var base graph.Workflow
	for _, w := range g.Workflows {
		if w.Name == "Full Booking" {
			base = w
			break
		}
	}
	require.NotEmpty(t, base.Name)

	// Compose with Traveler Modification addon.
	p, err := ComposeWithAddons(base, []string{"Traveler Modification"}, g, tpGraphDirCompose)
	require.NoError(t, err)

	// Traveler Modification addon has intentionally unfed updateValue
	// (LLM provides the new value at runtime).
	err = plan.Validate(p, g)
	if err != nil {
		assert.Contains(t, err.Error(), "updateValue", "unexpected validation error")
	}

	stepIDs := collectStepIDs(p)
	t.Logf("Step order: %s", strings.Join(stepIDs, " → "))

	// 8 parent + 2 traveler mod = 10 steps.
	require.Len(t, p.Execution.Steps, 10)
	assert.Contains(t, stepIDs, "inc0_getUpdatableTravelerItems")
	assert.Contains(t, stepIDs, "inc0_updateTraveler")
}

// TestCompose_UnfedInputsAfterComposition checks what's still unfed
// after auto-wiring.
func TestCompose_UnfedInputsAfterComposition(t *testing.T) {
	g := loadAirlineForCompose(t)

	var base graph.Workflow
	for _, w := range g.Workflows {
		if w.Name == "Full Booking" {
			base = w
			break
		}
	}
	require.NotEmpty(t, base.Name)

	p, err := ComposeWithAddons(base, []string{"Seat Selection"}, g, tpGraphDirCompose)
	require.NoError(t, err)

	unfed := UnfedInputsFromTemplate(p, g)
	t.Logf("Unfed inputs after composition (%d):", len(unfed))
	for _, u := range unfed {
		t.Logf("  - %s", u)
	}

	// The parent template provides most values. After auto-wiring,
	// remaining unfed inputs are those the LLM/user must fill.
	// Just verify it's a reasonable number (not all inputs unfed).
	assert.Less(t, len(unfed), 20, "too many unfed inputs — auto-wiring may not be working")
}

// TestCompose_MarshalRoundTrip verifies the composed plan can be marshalled
// and re-parsed.
func TestCompose_MarshalRoundTrip(t *testing.T) {
	g := loadAirlineForCompose(t)

	var base graph.Workflow
	for _, w := range g.Workflows {
		if w.Name == "Full Booking" {
			base = w
			break
		}
	}
	require.NotEmpty(t, base.Name)

	p, err := ComposeWithAddons(base, []string{"Seat Selection"}, g, tpGraphDirCompose)
	require.NoError(t, err)

	// Marshal to YAML.
	yamlBytes, err := plan.Marshal(p)
	require.NoError(t, err)
	t.Logf("Composed plan YAML (%d bytes):\n%s", len(yamlBytes), string(yamlBytes))

	// Parse back.
	p2, err := plan.Parse(yamlBytes)
	require.NoError(t, err)

	// Validate the round-tripped plan (Seat Selection has intentionally
	// unfed offerId/productId that the LLM fills).
	err = plan.Validate(p2, g)
	if err != nil {
		errStr := err.Error()
		assert.Contains(t, errStr, "offerId", "unexpected validation error")
		assert.Contains(t, errStr, "productId", "unexpected validation error")
	}

	assert.Equal(t, len(p.Execution.Steps), len(p2.Execution.Steps))
}

// --- helpers ---

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

func collectStepIDs(p *plan.Plan) []string {
	ids := make([]string, len(p.Execution.Steps))
	for i, s := range p.Execution.Steps {
		ids[i] = s.StepID()
	}
	return ids
}

func findStep(p *plan.Plan, id string) *plan.Step {
	for i := range p.Execution.Steps {
		if p.Execution.Steps[i].StepID() == id {
			return &p.Execution.Steps[i]
		}
	}
	return nil
}

// Prevent unused import warning.
var _ = fmt.Sprintf
