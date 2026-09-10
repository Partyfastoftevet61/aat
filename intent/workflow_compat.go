package intent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
)

// WorkflowCompatResult captures the results of addon-to-base workflow
// compatibility checking.
//   - Warnings: structural AUTOWIRE inputs that the graph CAN produce but a
//     specific base workflow doesn't satisfy.
//   - NonProducible: AUTOWIRE inputs whose name doesn't match any output or
//     elementField in the graph. These indicate either misuse of AUTOWIRE
//     (should be LLM-filled) or a missing graph elementField.
//   - Errors: template loading failures (non-fatal).
type WorkflowCompatResult struct {
	Warnings      []WorkflowCompatWarning
	NonProducible []WorkflowNonProducible
	Errors        []WorkflowCompatError
}

// WorkflowNonProducible records an addon with AUTOWIRE inputs that no node
// in the graph can produce. These inputs either need a graph elementField
// or shouldn't be marked AUTOWIRE.
type WorkflowNonProducible struct {
	Addon  string
	Inputs []string
}

// WorkflowCompatWarning records an addon+base pair where some AUTOWIRE
// inputs in the addon cannot be auto-wired from the base workflow's outputs.
type WorkflowCompatWarning struct {
	Addon        string
	BaseWorkflow string
	UnfedInputs  []string
}

// WorkflowCompatError records a template loading failure for a workflow.
type WorkflowCompatError struct {
	Workflow string
	Err      error
}

// HasWarnings returns true if any compatibility warnings were found.
func (r *WorkflowCompatResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// HasErrors returns true if any template loading errors occurred.
func (r *WorkflowCompatResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasNonProducible returns true if any AUTOWIRE inputs reference names
// that no graph node can produce.
func (r *WorkflowCompatResult) HasNonProducible() bool {
	return len(r.NonProducible) > 0
}

// HasIssues returns true if any warnings, non-producible entries, or errors were found.
func (r *WorkflowCompatResult) HasIssues() bool {
	return r.HasWarnings() || r.HasNonProducible() || r.HasErrors()
}

// Format returns a human-readable summary of compatibility issues.
func (r *WorkflowCompatResult) Format() string {
	if !r.HasWarnings() && !r.HasNonProducible() {
		return ""
	}

	var sb strings.Builder
	if r.HasWarnings() {
		sb.WriteString("Workflow compatibility warnings:\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(&sb, "  addon %q + base %q: unfed AUTOWIRE inputs: %s\n",
				w.Addon, w.BaseWorkflow, strings.Join(w.UnfedInputs, ", "))
		}
	}
	if r.HasNonProducible() {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Non-producible AUTOWIRE inputs (no graph output or elementField matches):\n")
		for _, np := range r.NonProducible {
			fmt.Fprintf(&sb, "  addon %q: %s\n", np.Addon, strings.Join(np.Inputs, ", "))
		}
	}
	return sb.String()
}

// ValidateWorkflowCompat checks that all addon workflows can have their
// AUTOWIRE inputs satisfied when composed into compatible base workflows.
// Only AUTOWIRE inputs that are "structural" (the graph produces a matching
// output name or elementField name somewhere) are checked. Value inputs
// that no node can produce (e.g., email, commentText) are assumed to be
// LLM-filled and are not flagged.
//
// Slots count the way composition uses them: a base's slots are filled before
// addons are spliced, so an input that every option of a slot produces is fed,
// and an addon may attach after a node that only a slot option contributes.
func ValidateWorkflowCompat(g *graph.Graph, graphDir string) *WorkflowCompatResult {
	bases, addons := partitionWorkflows(g)
	if len(addons) == 0 || len(bases) == 0 {
		return &WorkflowCompatResult{}
	}

	// Load all templates, caching by template path.
	templates := make(map[string]*plan.Plan)
	var loadErrs []WorkflowCompatError
	for _, wf := range g.Workflows {
		if wf.Template == "" {
			continue
		}
		if _, cached := templates[wf.Template]; cached {
			continue
		}
		p, err := LoadWorkflowTemplate(wf.Template, graphDir, g)
		if err != nil {
			loadErrs = append(loadErrs, WorkflowCompatError{
				Workflow: wf.Name,
				Err:      err,
			})
			continue
		}
		templates[wf.Template] = p
	}

	result := checkWorkflowCompat(g, templates)
	result.Errors = loadErrs
	return result
}

// checkWorkflowCompat runs the compatibility checks against templates that are
// already loaded, keyed by workflow template path. A base whose template, or
// any of whose slot option templates, is missing from the map is skipped.
func checkWorkflowCompat(g *graph.Graph, templates map[string]*plan.Plan) *WorkflowCompatResult {
	result := &WorkflowCompatResult{}

	bases, addons := partitionWorkflows(g)
	if len(addons) == 0 || len(bases) == 0 {
		return result
	}

	// Build the set of names that the graph can produce (outputs + elementFields).
	producible := buildProducibleNames(g)

	// For each addon, check compatibility with each base workflow.
	for _, addon := range addons {
		addonPlan := templates[addon.Template]
		if addonPlan == nil {
			continue // template failed to load
		}

		// Collect AUTOWIRE inputs from the addon plan.
		autowireInputs := collectAutowireInputs(addonPlan)
		if len(autowireInputs) == 0 {
			continue // nothing to check
		}

		// Remove inputs covered by explicit Wire entries (including MANUAL).
		for inputName := range addon.Wire {
			delete(autowireInputs, inputName)
		}

		// Separate structural vs non-producible AUTOWIRE inputs.
		// Non-producible: name doesn't match any output or elementField in the graph.
		var nonProducibleInputs []string
		for inputName := range autowireInputs {
			if !producible[inputName] {
				nonProducibleInputs = append(nonProducibleInputs, inputName)
				delete(autowireInputs, inputName)
			}
		}
		if len(nonProducibleInputs) > 0 {
			sort.Strings(nonProducibleInputs)
			result.NonProducible = append(result.NonProducible, WorkflowNonProducible{
				Addon:  addon.Name,
				Inputs: nonProducibleInputs,
			})
		}

		if len(autowireInputs) == 0 {
			continue
		}

		// Check each base workflow for compatibility.
		for _, base := range bases {
			shape, ok := composedShape(g, base, templates)
			if !ok {
				continue // base or slot option template failed to load
			}

			// The addon only composes into bases that contain one of its After nodes.
			if addon.After.IsSet() && !shape.hasAnyNode(addon.After) {
				continue
			}

			// Check which structural AUTOWIRE inputs are not satisfied.
			var unfed []string
			for inputName := range autowireInputs {
				if !shape.feeds(inputName) {
					unfed = append(unfed, inputName)
				}
			}

			if len(unfed) > 0 {
				sort.Strings(unfed)
				result.Warnings = append(result.Warnings, WorkflowCompatWarning{
					Addon:        addon.Name,
					BaseWorkflow: base.Name,
					UnfedInputs:  unfed,
				})
			}
		}
	}

	return result
}

// partitionWorkflows splits the graph's workflows into base workflows and
// addons. Slot options are neither: they are checked through the bases whose
// slots offer them.
func partitionWorkflows(g *graph.Graph) (bases, addons []graph.Workflow) {
	for _, wf := range g.Workflows {
		switch {
		case wf.IsAddon():
			addons = append(addons, wf)
		case wf.IsSlot():
			// checked through its base
		default:
			bases = append(bases, wf)
		}
	}
	return bases, addons
}

// baseShape describes what composing a base workflow can contain before addons
// are spliced in: the base template's own steps, plus the steps each option of
// each slot would add.
type baseShape struct {
	base        *plan.Plan
	baseOutputs map[string]string
	slots       [][]*plan.Plan        // per slot, one template per option
	slotOutputs [][]map[string]string // per slot, one output map per option
}

// composedShape gathers the base's template and the templates of every option
// of its slots. It reports false when any of them is unavailable.
func composedShape(g *graph.Graph, base graph.Workflow, templates map[string]*plan.Plan) (*baseShape, bool) {
	basePlan := templates[base.Template]
	if basePlan == nil {
		return nil, false
	}
	shape := &baseShape{base: basePlan, baseOutputs: buildOutputMap(basePlan, g)}
	for _, sd := range base.Slots {
		var options []*plan.Plan
		var outputs []map[string]string
		for _, optionName := range sd.Options {
			option, found := findWorkflowByName(g, optionName)
			if !found || templates[option.Template] == nil {
				return nil, false
			}
			options = append(options, templates[option.Template])
			outputs = append(outputs, buildOutputMap(templates[option.Template], g))
		}
		shape.slots = append(shape.slots, options)
		shape.slotOutputs = append(shape.slotOutputs, outputs)
	}
	return shape, true
}

// hasAnyNode reports whether any of the nodes appears in the base template or
// in some slot option, i.e. whether an addon can find its insertion point in
// at least one composition of the base.
func (s *baseShape) hasAnyNode(nodes graph.AfterSpec) bool {
	for _, node := range nodes {
		if findStepByNode(s.base, node) != "" {
			return true
		}
		for _, options := range s.slots {
			for _, option := range options {
				if findStepByNode(option, node) != "" {
					return true
				}
			}
		}
	}
	return false
}

// feeds reports whether every composition of the base produces an output named
// input: the base template produces it, or every option of some slot does.
func (s *baseShape) feeds(input string) bool {
	if _, ok := s.baseOutputs[input]; ok {
		return true
	}
	for _, options := range s.slotOutputs {
		if len(options) == 0 {
			continue
		}
		everyOption := true
		for _, outputs := range options {
			if _, ok := outputs[input]; !ok {
				everyOption = false
				break
			}
		}
		if everyOption {
			return true
		}
	}
	return false
}

// buildProducibleNames returns the set of all names that the graph can
// produce: output names and elementField names across all nodes. An
// AUTOWIRE input is considered "structural" (expected to come from upstream)
// only if its name appears in this set.
func buildProducibleNames(g *graph.Graph) map[string]bool {
	names := make(map[string]bool)
	for _, node := range g.Nodes {
		for _, out := range node.Outputs {
			names[out.Name] = true
			for _, ef := range out.ElementFields {
				names[ef.Name] = true
			}
		}
	}
	return names
}

// collectAutowireInputs scans all steps in a plan and returns a set of
// input names that have AUTOWIRE placeholder values.
func collectAutowireInputs(p *plan.Plan) map[string]bool {
	inputs := make(map[string]bool)
	for _, step := range p.Execution.Steps {
		for inputName, sv := range step.Values {
			if isPlaceholder(sv) {
				inputs[inputName] = true
			}
		}
	}
	return inputs
}
