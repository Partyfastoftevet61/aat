package main

import (
	"fmt"
	"path/filepath"

	"github.com/gburgyan/aat/graph"
)

// specCheck runs one contract validator over a graph: it loads every spec the
// graph refers to, resolving relative paths against the graph's directory, then
// cross-references the nodes. It returns nil when the graph refers to no spec
// of that kind, so a project without one gets no section at all.
//
// Both contract kinds go through this: OpenAPI documents for HTTP nodes and
// protobuf descriptor sets for gRPC ones.
func specCheck(name string, v graph.SpecValidator, g *graph.Graph, graphPath string, strict bool) *sectionResult {
	specPaths := v.CollectSpecPaths(g)
	if len(specPaths) == 0 {
		return nil
	}

	graphDir := filepath.Dir(graphPath)
	var loadErrors []string
	for _, sp := range specPaths {
		resolved := sp
		if !filepath.IsAbs(sp) {
			resolved = filepath.Join(graphDir, sp)
		}
		if err := v.LoadSpec(sp, resolved); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("loading spec %q: %s", sp, err))
		}
	}
	if len(loadErrors) > 0 {
		return &sectionResult{Name: name, Status: "FAILED", Errors: loadErrors}
	}

	result := v.Validate(g)
	if !result.HasIssues() {
		return &sectionResult{Name: name, Status: "OK"}
	}
	return &sectionResult{
		Name:   name,
		Status: issueStatus(result.HasErrors(), strict),
		Errors: []string{result.Format()},
	}
}
