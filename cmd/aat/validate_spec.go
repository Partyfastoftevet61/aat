package main

import (
	"fmt"
	"path/filepath"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/engine"
	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/graph/proto"
)

// specCheck runs one contract validator over a graph: it loads every spec the
// project and the graph refer to, then cross-references the nodes. It returns
// nil when neither refers to a spec of that kind, so a project without one gets
// no section at all.
//
// projectPaths are the specs aat-project.yaml names, already resolved against
// the manifest's directory; graph.ResolveSpecPaths knows not to join those onto
// the graph's directory a second time. The OpenAPI caller passes nil on
// purpose: a manifest's oas: is read by the MCP server alone, while its proto:
// describes the project.
//
// Both contract kinds go through this: OpenAPI documents for HTTP nodes and
// protobuf descriptor sets for gRPC ones.
func specCheck(name string, v graph.SpecValidator, g *graph.Graph, graphPath string, projectPaths []string, strict bool) *sectionResult {
	specPaths := graph.ResolveSpecPaths(v.CollectSpecPaths(g), filepath.Dir(graphPath), projectPaths)
	if len(specPaths) == 0 {
		return nil
	}

	var loadErrors []string
	for _, sp := range specPaths {
		if err := v.LoadSpec(sp.Ref, sp.Path); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("loading spec %q: %s", sp.Ref, err))
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

// protoSpecCheck runs the protobuf section: the graph's gRPC nodes against the
// descriptor sets the project and the graph name. registry may be nil when the
// templates did not load, in which case outputs are checked by name rather than
// at their extract paths.
//
// It also reports the two cases specCheck alone renders as silence: a project
// that names a descriptor set but has no gRPC node to check against it, and a
// graph full of gRPC nodes that names no descriptor set at all.
func protoSpecCheck(g *graph.Graph, graphPath string, projectProto []string, registry *adapter.Registry, strict bool) *sectionResult {
	nodes := proto.CountNodes(g)

	validator := proto.NewValidator().WithProjectDescriptors(projectProto)
	if registry != nil {
		validator.WithOutputPaths(proto.OutputPaths(engine.OutputExtractPaths(g, registry)))
	}

	section := specCheck("Protobuf validation", validator, g, graphPath, projectProto, strict)
	if section == nil {
		if nodes == 0 {
			return nil
		}
		// Nodes name gRPC methods and nothing names a descriptor set, so there
		// was no spec to load and nothing was checked.
		return &sectionResult{
			Name:   "Protobuf validation",
			Status: "FAILED",
			Errors: []string{fmt.Sprintf("%s but the project names no descriptor set; add proto: <file>.protoset to aat-project.yaml or the graph",
				pluralize(nodes, "node")+" name a gRPC method")},
		}
	}

	if nodes == 0 {
		section.Detail = "(" + pluralize(len(projectProto), "descriptor set") + ", no gRPC nodes)"
		section.Notes = append(section.Notes,
			"aat-project.yaml names a descriptor set, but no node has proto:; a node is checked against it once it names a method")
	} else {
		section.Detail = "(" + pluralize(nodes, "gRPC node") + ")"
	}
	return section
}

// nodeProtocolSection reports the node-vs-template protocol check. It returns
// nil for a project with no gRPC surface and nothing to say, so an HTTP-only
// project gains no line.
func nodeProtocolSection(g *graph.Graph, registry *adapter.Registry, strict bool) *sectionResult {
	if registry == nil {
		return nil
	}
	report := engine.ValidateNodeProtocols(g, registry)
	if report.GRPCNodes == 0 && !report.Result.HasIssues() {
		return nil
	}
	if !report.Result.HasIssues() {
		return &sectionResult{
			Name:   "Node protocols",
			Status: "OK",
			Detail: fmt.Sprintf("(%d gRPC, %d HTTP)", report.GRPCNodes, report.HTTPNodes),
		}
	}
	return &sectionResult{
		Name:   "Node protocols",
		Status: issueStatus(report.Result.HasErrors(), strict),
		Detail: fmt.Sprintf("(%d gRPC, %d HTTP)", report.GRPCNodes, report.HTTPNodes),
		Errors: []string{report.Result.Format()},
	}
}
