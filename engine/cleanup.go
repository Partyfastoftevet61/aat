package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/graph"
)

// CleanupEntry records a cleanup node to execute and which forward node triggered it.
type CleanupEntry struct {
	NodeName string // cleanup node (e.g., "ignoreItinerary")
	ForNode  string // forward node that registered this cleanup
}

// CleanupStack maintains cleanup entries in FILO order.
type CleanupStack struct {
	entries []CleanupEntry
}

// Push adds a cleanup entry to the stack.
func (s *CleanupStack) Push(entry CleanupEntry) {
	s.entries = append(s.entries, entry)
}

// Len returns the number of entries in the stack.
func (s *CleanupStack) Len() int {
	return len(s.entries)
}

// Filter removes entries for which keep returns false, preserving order.
func (s *CleanupStack) Filter(keep func(CleanupEntry) bool) {
	kept := s.entries[:0]
	for _, entry := range s.entries {
		if keep(entry) {
			kept = append(kept, entry)
		}
	}
	s.entries = kept
}

// ExecuteAll runs all cleanup entries in FILO order (last pushed, first executed).
// Errors are recorded in the StepResult but do not stop subsequent cleanup steps.
// The requests use ctx as given: Engine.runCleanup passes a context detached
// from the run's cancellation, with a deadline when the run was aborted.
func (s *CleanupStack) ExecuteAll(
	ctx context.Context,
	g *graph.Graph,
	registry *adapter.Registry,
	router *ExecutorRouter,
	state *RunState,
) []StepResult {
	if len(s.entries) == 0 {
		return nil
	}

	results := make([]StepResult, 0, len(s.entries))

	// Execute in reverse order (FILO)
	for i := len(s.entries) - 1; i >= 0; i-- {
		entry := s.entries[i]
		result := executeCleanupEntry(ctx, entry, g, registry, router, state)
		results = append(results, result)
	}

	return results
}

func executeCleanupEntry(
	ctx context.Context,
	entry CleanupEntry,
	g *graph.Graph,
	registry *adapter.Registry,
	router *ExecutorRouter,
	state *RunState,
) StepResult {
	start := time.Now()
	// base identifies the cleanup step and when it started, so archives place
	// it on the run's timeline like any other step.
	base := StepResult{StepID: entry.NodeName, Node: entry.NodeName, StartTime: start}
	failed := func(inputs map[string]any, err error) StepResult {
		sr := base
		sr.Inputs = inputs
		sr.Error = err
		sr.Duration = time.Since(start)
		return sr
	}

	node, ok := g.Nodes[entry.NodeName]
	if !ok {
		return failed(nil, fmt.Errorf("cleanup node %q not found in graph", entry.NodeName))
	}

	// Resolve inputs from current state using name-matching.
	// For each cleanup input, try the node that registered this cleanup
	// (entry.ForNode) first, then scan all executed steps for a matching
	// output name.
	inputs := make(map[string]any)
	for _, input := range node.Inputs {
		// First try the node that registered this cleanup
		if val, err := state.GetOutput(entry.ForNode, input.Name); err == nil {
			inputs[input.Name] = val
			continue
		}
		// Scan all executed steps for a matching output name
		for _, stepID := range state.ExecutedSteps() {
			if val, err := state.GetOutput(stepID, input.Name); err == nil {
				inputs[input.Name] = val
				break
			}
		}
	}

	adp, err := registry.Get(node.Adapter)
	if err != nil {
		return failed(inputs, fmt.Errorf("cleanup adapter: %w", err))
	}

	// Resolve executor/config/rewrite for the cleanup node
	exec, cfg, rewrite := router.Resolve(entry.NodeName)
	base.ActualBaseURL = exec.BaseURL

	req, err := adp.BuildRequest(inputs, cfg)
	if err != nil {
		return failed(inputs, fmt.Errorf("cleanup build request: %w", err))
	}

	if rewrite != nil {
		base.OriginalPath = req.Path
		req.Path = adapter.RewritePath(req.Path, rewrite)
	}
	base.Request = req

	resp, err := exec.Execute(ctx, req)
	if err != nil {
		return failed(inputs, fmt.Errorf("cleanup execute: %w", err))
	}

	result := base
	result.Inputs = inputs
	result.Response = resp
	result.StatusCode = resp.StatusCode
	result.Duration = time.Since(start)

	// Extract outputs (best-effort for cleanup)
	outputs, err := adp.ExtractOutputs(resp)
	if err == nil {
		result.Outputs = outputs
	}
	// Cleanup extraction errors are silently ignored — empty outputs is fine

	return result
}
