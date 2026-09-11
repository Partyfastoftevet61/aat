package intent

import (
	"testing"

	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildCompatTestGraph creates a graph with nodes that match the compat test fixtures.
// "specialProvider" produces "specialInput" as an output, making it a producible
// name — so AUTOWIRE for "specialInput" is structural and will be validated.
func buildCompatTestGraph() *graph.Graph {
	return &graph.Graph{
		Version: "1.0",
		Nodes: map[string]*graph.Node{
			"search": {
				Name: "search", Adapter: "search",
				Outputs: []graph.Output{
					{Name: "results", Type: "string[]"},
					{Name: "token", Type: "string"},
				},
			},
			"book": {
				Name: "book", Adapter: "book",
				Inputs:  []graph.Input{{Name: "itemId", Type: "string"}},
				Outputs: []graph.Output{{Name: "itineraryId", Type: "string"}, {Name: "confirmationCode", Type: "string"}},
			},
			"commit": {
				Name: "commit", Adapter: "commit",
				Inputs:  []graph.Input{{Name: "itineraryId", Type: "string"}},
				Outputs: []graph.Output{{Name: "locator", Type: "string"}},
			},
			"addonNode": {
				Name: "addonNode", Adapter: "addonNode",
				Inputs: []graph.Input{
					{Name: "itineraryId", Type: "string"},
					{Name: "specialInput", Type: "string"},
				},
				Outputs: []graph.Output{{Name: "addonResult", Type: "string"}},
			},
			"specialProvider": {
				Name: "specialProvider", Adapter: "specialProvider",
				Outputs: []graph.Output{{Name: "specialInput", Type: "string"}},
			},
		},
	}
}

func TestValidateWorkflowCompat_AllSatisfied(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "Booking", Template: "testdata/compat/base_full.yaml"},
		{
			Name:     "AddonWired",
			Kind:     "addon",
			Template: "testdata/compat/addon_wired.yaml",
			After:    graph.AfterSpec{"book"},
		},
	}

	result := ValidateWorkflowCompat(g, ".")
	assert.False(t, result.HasWarnings(), "expected no warnings, got: %v", result.Warnings)
	assert.False(t, result.HasErrors())
}

func TestValidateWorkflowCompat_UnfedStructuralInput(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "Booking", Template: "testdata/compat/base_full.yaml"},
		{
			Name:     "AddonUnfed",
			Kind:     "addon",
			Template: "testdata/compat/addon_unfed.yaml",
			After:    graph.AfterSpec{"book"},
		},
	}

	result := ValidateWorkflowCompat(g, ".")
	require.True(t, result.HasWarnings())
	require.Len(t, result.Warnings, 1)
	w := result.Warnings[0]
	assert.Equal(t, "AddonUnfed", w.Addon)
	assert.Equal(t, "Booking", w.BaseWorkflow)
	// specialInput is producible (specialProvider outputs it) but base_full
	// doesn't include specialProvider → unfed warning.
	// itineraryId is produced by book → satisfied.
	assert.Equal(t, []string{"specialInput"}, w.UnfedInputs)
}

func TestValidateWorkflowCompat_ValueInputNotFlagged(t *testing.T) {
	// An AUTOWIRE input whose name doesn't match any graph output or
	// elementField is a value input (LLM-filled) and should not generate
	// a compatibility warning. It IS recorded as non-producible.
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "Booking", Template: "testdata/compat/base_full.yaml"},
		{
			Name:     "AddonValue",
			Kind:     "addon",
			Template: "testdata/compat/addon_value_input.yaml",
			After:    graph.AfterSpec{"book"},
		},
	}

	result := ValidateWorkflowCompat(g, ".")
	assert.False(t, result.HasWarnings(), "value inputs should not be compatibility warnings")
	require.True(t, result.HasNonProducible())
	np := result.NonProducible[0]
	assert.Equal(t, "AddonValue", np.Addon)
	assert.Contains(t, np.Inputs, "email")
	assert.Contains(t, np.Inputs, "phoneNumber")
}

func TestValidateWorkflowCompat_ElementFieldProducible(t *testing.T) {
	// AUTOWIRE input matching an elementField name (not just output name)
	// is considered structural and should be flagged when unfed.
	g := buildCompatTestGraph()
	// Add an elementField "itemCode" to search.results
	g.Nodes["search"].Outputs[0].ElementFields = []graph.Field{
		{Name: "itemCode", Type: "string"},
	}
	g.Workflows = []graph.Workflow{
		{Name: "Booking", Template: "testdata/compat/base_full.yaml"},
		{
			Name:     "AddonEF",
			Kind:     "addon",
			Template: "testdata/compat/addon_elementfield.yaml",
			After:    graph.AfterSpec{"book"},
		},
	}

	result := ValidateWorkflowCompat(g, ".")
	require.True(t, result.HasWarnings())
	w := result.Warnings[0]
	// itemCode is producible (elementField) but not in base output map → unfed
	assert.Equal(t, []string{"itemCode"}, w.UnfedInputs)
}

func TestValidateWorkflowCompat_WireOverrideSatisfied(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "Booking", Template: "testdata/compat/base_full.yaml"},
		{
			Name:     "AddonWired",
			Kind:     "addon",
			Template: "testdata/compat/addon_unfed.yaml",
			After:    graph.AfterSpec{"book"},
			Wire: map[string]string{
				"specialInput": "search.token",
			},
		},
	}

	result := ValidateWorkflowCompat(g, ".")
	assert.False(t, result.HasWarnings(), "wire override should satisfy specialInput")
}

func TestValidateWorkflowCompat_ManualWireNotFlagged(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "Booking", Template: "testdata/compat/base_full.yaml"},
		{
			Name:     "AddonManual",
			Kind:     "addon",
			Template: "testdata/compat/addon_unfed.yaml",
			After:    graph.AfterSpec{"book"},
			Wire: map[string]string{
				"specialInput": "MANUAL",
			},
		},
	}

	result := ValidateWorkflowCompat(g, ".")
	assert.False(t, result.HasWarnings(), "MANUAL wire should not be flagged")
}

func TestValidateWorkflowCompat_AddonNotInBase(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "SearchOnly", Template: "testdata/compat/base_partial.yaml"},
		{
			Name:     "AddonWired",
			Kind:     "addon",
			Template: "testdata/compat/addon_wired.yaml",
			After:    graph.AfterSpec{"book"}, // book is NOT in base_partial
		},
	}

	result := ValidateWorkflowCompat(g, ".")
	assert.False(t, result.HasWarnings(), "addon After node not in base → no compatibility check")
}

func TestValidateWorkflowCompat_MultipleBasesPartialCompat(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "FullBase", Template: "testdata/compat/base_full.yaml"},
		{Name: "PartialBase", Template: "testdata/compat/base_partial.yaml"},
		{
			Name:     "AddonWired",
			Kind:     "addon",
			Template: "testdata/compat/addon_wired.yaml",
			After:    graph.AfterSpec{"book"},
		},
	}

	result := ValidateWorkflowCompat(g, ".")
	// FullBase has book (After node present) and produces itineraryId → OK
	// PartialBase doesn't have book → addon is not compatible → no check
	assert.False(t, result.HasWarnings())
}

func TestValidateWorkflowCompat_TemplateLoadError(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "BadBase", Template: "testdata/compat/nonexistent.yaml"},
		{
			Name:     "AddonWired",
			Kind:     "addon",
			Template: "testdata/compat/addon_wired.yaml",
			After:    graph.AfterSpec{"book"},
		},
	}

	result := ValidateWorkflowCompat(g, ".")
	require.True(t, result.HasErrors())
	assert.Equal(t, "BadBase", result.Errors[0].Workflow)
	assert.False(t, result.HasWarnings(), "should not warn when base template fails to load")
}

func TestValidateWorkflowCompat_NoWorkflows(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = nil

	result := ValidateWorkflowCompat(g, ".")
	assert.False(t, result.HasIssues())
}

func TestValidateWorkflowCompat_NoAddons(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "Base", Template: "testdata/compat/base_full.yaml"},
	}

	result := ValidateWorkflowCompat(g, ".")
	assert.False(t, result.HasIssues())
}

func TestValidateWorkflowCompat_NoBases(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "Addon", Kind: "addon", Template: "testdata/compat/addon_wired.yaml", After: graph.AfterSpec{"book"}},
	}

	result := ValidateWorkflowCompat(g, ".")
	assert.False(t, result.HasIssues())
}

func TestWorkflowCompatResult_Format(t *testing.T) {
	result := &WorkflowCompatResult{
		Warnings: []WorkflowCompatWarning{
			{
				Addon:        "SeatAddon",
				BaseWorkflow: "SimpleBooking",
				UnfedInputs:  []string{"seatMapId", "segmentRef"},
			},
		},
	}

	formatted := result.Format()
	assert.Contains(t, formatted, "Workflow compatibility warnings:")
	assert.Contains(t, formatted, `addon "SeatAddon"`)
	assert.Contains(t, formatted, `base "SimpleBooking"`)
	assert.Contains(t, formatted, "seatMapId, segmentRef")
}

func TestWorkflowCompatResult_FormatNonProducible(t *testing.T) {
	result := &WorkflowCompatResult{
		NonProducible: []WorkflowNonProducible{
			{Addon: "ContactAddon", Inputs: []string{"email", "phoneNumber"}},
		},
	}

	formatted := result.Format()
	assert.Contains(t, formatted, "Non-producible AUTOWIRE inputs")
	assert.Contains(t, formatted, `addon "ContactAddon"`)
	assert.Contains(t, formatted, "email, phoneNumber")
}

func TestWorkflowCompatResult_FormatBoth(t *testing.T) {
	result := &WorkflowCompatResult{
		Warnings: []WorkflowCompatWarning{
			{Addon: "SeatAddon", BaseWorkflow: "Booking", UnfedInputs: []string{"seatMapId"}},
		},
		NonProducible: []WorkflowNonProducible{
			{Addon: "ContactAddon", Inputs: []string{"email"}},
		},
	}

	formatted := result.Format()
	assert.Contains(t, formatted, "Workflow compatibility warnings:")
	assert.Contains(t, formatted, "Non-producible AUTOWIRE inputs")
}

func TestWorkflowCompatResult_FormatEmpty(t *testing.T) {
	result := &WorkflowCompatResult{}
	assert.Equal(t, "", result.Format())
}

// --- buildProducibleNames ---

func TestBuildProducibleNames(t *testing.T) {
	g := &graph.Graph{
		Nodes: map[string]*graph.Node{
			"a": {
				Name: "a",
				Outputs: []graph.Output{
					{Name: "alpha", Type: "string"},
					{
						Name: "items", Type: "item[]",
						ElementFields: []graph.Field{
							{Name: "itemId", Type: "string"},
							{Name: "itemName", Type: "string"},
						},
					},
				},
			},
			"b": {
				Name: "b",
				Outputs: []graph.Output{
					{Name: "beta", Type: "string"},
				},
			},
		},
	}

	names := buildProducibleNames(g)
	assert.True(t, names["alpha"])
	assert.True(t, names["items"])
	assert.True(t, names["itemId"])
	assert.True(t, names["itemName"])
	assert.True(t, names["beta"])
	assert.False(t, names["nonexistent"])
	assert.False(t, names["email"]) // no node produces email
}

// --- collectAutowireInputs ---

func TestCollectAutowireInputs(t *testing.T) {
	p := &plan.Plan{
		Execution: plan.Execution{
			Steps: []plan.Step{
				{
					Node: "step1",
					Values: map[string]plan.StepValue{
						"a": {Default: "AUTOWIRE"},
						"b": {Default: "literal"},
						"c": {From: "other.output"},
					},
				},
				{
					Node: "step2",
					Values: map[string]plan.StepValue{
						"d": {Default: "AUTOWIRE"},
						"a": {Default: "AUTOWIRE"}, // duplicate name from different step
					},
				},
			},
		},
	}

	inputs := collectAutowireInputs(p)
	assert.True(t, inputs["a"])
	assert.True(t, inputs["d"])
	assert.False(t, inputs["b"])
	assert.False(t, inputs["c"])
}

// --- In-memory checks (checkWorkflowCompat) ---

func TestCheckWorkflowCompat_AllSatisfied(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "Base", Template: "base"},
		{Name: "Addon", Kind: "addon", Template: "addon", After: graph.AfterSpec{"book"}},
	}

	plans := map[string]*plan.Plan{
		"base": {
			Execution: plan.Execution{
				Steps: []plan.Step{
					{Node: "search"},
					{Node: "book"},
				},
			},
		},
		"addon": {
			Execution: plan.Execution{
				Steps: []plan.Step{
					{Node: "addonNode", Values: map[string]plan.StepValue{
						"itineraryId": {Default: "AUTOWIRE"},
					}},
				},
			},
		},
	}

	result := checkWorkflowCompat(g, plans)
	assert.False(t, result.HasWarnings())
}

func TestCheckWorkflowCompat_UnfedStructural(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "Base", Template: "base"},
		{Name: "Addon", Kind: "addon", Template: "addon", After: graph.AfterSpec{"search"}},
	}

	plans := map[string]*plan.Plan{
		"base": {
			Execution: plan.Execution{
				Steps: []plan.Step{
					{Node: "search"},
				},
			},
		},
		"addon": {
			Execution: plan.Execution{
				Steps: []plan.Step{
					{Node: "addonNode", Values: map[string]plan.StepValue{
						"itineraryId":  {Default: "AUTOWIRE"},
						"specialInput": {Default: "AUTOWIRE"},
					}},
				},
			},
		},
	}

	result := checkWorkflowCompat(g, plans)
	require.True(t, result.HasWarnings())
	w := result.Warnings[0]
	// search only produces "results" and "token".
	// itineraryId is producible (book outputs it) but not in search-only base → unfed.
	// specialInput is producible (specialProvider outputs it) but not in base → unfed.
	assert.Equal(t, []string{"itineraryId", "specialInput"}, w.UnfedInputs)
}

func TestCheckWorkflowCompat_ValueInputFiltered(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "Base", Template: "base"},
		{Name: "Addon", Kind: "addon", Template: "addon", After: graph.AfterSpec{"book"}},
	}

	plans := map[string]*plan.Plan{
		"base": {
			Execution: plan.Execution{
				Steps: []plan.Step{
					{Node: "search"},
					{Node: "book"},
				},
			},
		},
		"addon": {
			Execution: plan.Execution{
				Steps: []plan.Step{
					{Node: "addonNode", Values: map[string]plan.StepValue{
						"itineraryId": {Default: "AUTOWIRE"}, // producible, satisfied
						"email":       {Default: "AUTOWIRE"}, // NOT producible → non-producible entry
					}},
				},
			},
		},
	}

	result := checkWorkflowCompat(g, plans)
	// email is not produced by any node → no compatibility warning
	assert.False(t, result.HasWarnings())
	// email shows up as non-producible
	require.True(t, result.HasNonProducible())
	assert.Contains(t, result.NonProducible[0].Inputs, "email")
}

// TestCheckWorkflowCompat_Slots: composition fills a base's slots before it
// splices addons, so slot options count toward what the base produces and
// toward where an addon can attach.
func TestCheckWorkflowCompat_Slots(t *testing.T) {
	tests := []struct {
		name        string
		optionB     []plan.Step
		after       graph.AfterSpec
		addonValues map[string]plan.StepValue
		wantUnfed   []string // nil: no warning expected
	}{
		{
			name:        "every slot option produces the input",
			optionB:     []plan.Step{{Node: "book"}},
			after:       graph.AfterSpec{"commit"},
			addonValues: map[string]plan.StepValue{"itineraryId": {Default: "AUTOWIRE"}},
		},
		{
			name:        "one slot option does not produce the input",
			optionB:     []plan.Step{{Node: "search"}},
			after:       graph.AfterSpec{"commit"},
			addonValues: map[string]plan.StepValue{"itineraryId": {Default: "AUTOWIRE"}},
			wantUnfed:   []string{"itineraryId"},
		},
		{
			name:        "after node contributed only by a slot option",
			optionB:     []plan.Step{{Node: "book"}},
			after:       graph.AfterSpec{"book"},
			addonValues: map[string]plan.StepValue{"specialInput": {Default: "AUTOWIRE"}},
			wantUnfed:   []string{"specialInput"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := buildCompatTestGraph()
			g.Workflows = []graph.Workflow{
				{Name: "Base", Template: "base", Slots: []graph.SlotDef{
					{Name: "cart", Options: []string{"OptA", "OptB"}, Default: "OptA"},
				}},
				{Name: "OptA", Kind: "slot", Template: "optA"},
				{Name: "OptB", Kind: "slot", Template: "optB"},
				{Name: "Addon", Kind: "addon", Template: "addon", After: tt.after},
			}
			plans := map[string]*plan.Plan{
				"base": {Execution: plan.Execution{Steps: []plan.Step{
					{Node: "search"}, {Slot: "cart"}, {Node: "commit"},
				}}},
				"optA":  {Execution: plan.Execution{Steps: []plan.Step{{Node: "book"}}}},
				"optB":  {Execution: plan.Execution{Steps: tt.optionB}},
				"addon": {Execution: plan.Execution{Steps: []plan.Step{{Node: "addonNode", Values: tt.addonValues}}}},
			}

			result := checkWorkflowCompat(g, plans)
			if tt.wantUnfed == nil {
				assert.False(t, result.HasWarnings(), "unexpected warnings: %v", result.Warnings)
				return
			}
			// Slot options are checked through their base, never as bases of their own.
			require.Len(t, result.Warnings, 1)
			assert.Equal(t, "Base", result.Warnings[0].BaseWorkflow)
			assert.Equal(t, tt.wantUnfed, result.Warnings[0].UnfedInputs)
		})
	}
}

func TestCheckWorkflowCompat_MissingSlotOptionTemplateSkipsBase(t *testing.T) {
	g := buildCompatTestGraph()
	g.Workflows = []graph.Workflow{
		{Name: "Base", Template: "base", Slots: []graph.SlotDef{
			{Name: "cart", Options: []string{"OptA", "OptB"}},
		}},
		{Name: "OptA", Kind: "slot", Template: "optA"},
		{Name: "OptB", Kind: "slot", Template: "optB"}, // failed to load: absent from the map
		{Name: "Addon", Kind: "addon", Template: "addon", After: graph.AfterSpec{"commit"}},
	}
	plans := map[string]*plan.Plan{
		"base":  {Execution: plan.Execution{Steps: []plan.Step{{Node: "search"}, {Slot: "cart"}, {Node: "commit"}}}},
		"optA":  {Execution: plan.Execution{Steps: []plan.Step{{Node: "search"}}}},
		"addon": {Execution: plan.Execution{Steps: []plan.Step{{Node: "addonNode", Values: map[string]plan.StepValue{"itineraryId": {Default: "AUTOWIRE"}}}}}},
	}

	result := checkWorkflowCompat(g, plans)
	assert.False(t, result.HasWarnings(), "a base with an unloadable slot option is not checked")
}
