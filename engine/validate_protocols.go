package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
)

// ProtocolValidationError collects node-vs-template protocol mismatches.
type ProtocolValidationError struct {
	Errors []string
}

func (e *ProtocolValidationError) Error() string {
	return fmt.Sprintf("node protocol validation failed:\n  - %s", strings.Join(e.Errors, "\n  - "))
}

// ProtocolReport is the outcome of the node-vs-template protocol check: what a
// node's proto: says measured against what its template's protocol: and rpc:
// say.
type ProtocolReport struct {
	// Result holds every issue, including the warnings --strict decides on.
	// It is never nil.
	Result *graph.SpecValidationResult
	// GRPCNodes and HTTPNodes count the templated nodes of each protocol.
	GRPCNodes int
	HTTPNodes int
}

// Err returns the error-severity issues as one error, or nil. Warnings are
// left out: they are for aat validate, where --strict decides whether they
// fail, and must not stop a run.
func (r *ProtocolReport) Err() error {
	var errs []string
	for _, issue := range r.Result.Issues {
		if issue.Severity == graph.SpecError {
			errs = append(errs, fmt.Sprintf("node %q: %s", issue.Node, issue.Message))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return &ProtocolValidationError{Errors: errs}
}

// ValidateNodeProtocols checks every node in the graph against its template:
// that a node naming a gRPC method has a gRPC template, that a gRPC template's
// node names the method it sends, and that the two name the same method.
// Nodes whose adapter is not template-based are skipped.
//
// Nothing at run time reads a node's proto: — a gRPC call is built entirely
// from the template's protocol: and rpc:. So a mismatch does not change what
// goes over the wire; it means the contract aat validate checked is not the
// call the run makes, which is worse than a wrong call, because it passes.
func ValidateNodeProtocols(g *graph.Graph, registry *adapter.Registry) *ProtocolReport {
	return validateNodeProtocolsForNodes(g, registry, nil)
}

// ValidateNodeProtocolsForPlan checks only the nodes the plan's steps and
// cleanup reference, as ValidateAdapterOutputsForPlan does.
func ValidateNodeProtocolsForPlan(g *graph.Graph, registry *adapter.Registry, p *plan.Plan) *ProtocolReport {
	return validateNodeProtocolsForNodes(g, registry, planNodeSet(p))
}

// planNodeSet returns the nodes a plan's steps and cleanup reference.
func planNodeSet(p *plan.Plan) map[string]bool {
	nodeSet := make(map[string]bool)
	for _, step := range p.Execution.Steps {
		nodeSet[step.Node] = true
	}
	for _, cs := range p.Execution.Cleanup {
		nodeSet[cs.Node] = true
	}
	return nodeSet
}

func validateNodeProtocolsForNodes(g *graph.Graph, registry *adapter.Registry, nodeSet map[string]bool) *ProtocolReport {
	report := &ProtocolReport{Result: &graph.SpecValidationResult{}}
	if g == nil || registry == nil {
		return report
	}

	names := make([]string, 0, len(g.Nodes))
	for name := range g.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		node := g.Nodes[name]
		if node == nil || (nodeSet != nil && !nodeSet[name]) {
			continue
		}
		tmpl, ok := registry.GetTemplate(node.Adapter)
		if !ok {
			// A custom adapter implementation; it has no declared protocol.
			continue
		}

		fail := func(format string, args ...any) {
			report.Result.Issues = append(report.Result.Issues, graph.SpecValidationIssue{
				Severity: graph.SpecError, Node: name, Message: fmt.Sprintf(format, args...),
			})
		}
		warn := func(format string, args ...any) {
			report.Result.Issues = append(report.Result.Issues, graph.SpecValidationIssue{
				Severity: graph.SpecWarning, Node: name, Message: fmt.Sprintf(format, args...),
			})
		}

		isGRPC := tmpl.Protocol == adapter.ProtocolGRPC
		if isGRPC {
			report.GRPCNodes++
		} else {
			report.HTTPNodes++
		}

		switch {
		case node.Proto != nil && !isGRPC:
			fail("declares proto %s but its template %q is protocol: %s; set protocol: grpc on the template, or drop proto: from the node",
				node.Proto.String(), node.Adapter, tmpl.Protocol)
		case node.Proto == nil && isGRPC:
			warn("template %q is protocol: grpc but the node names no method; add proto: %s to the node so aat validate checks it against the descriptors",
				node.Adapter, tmpl.Request.RPC)
		case node.Proto != nil && isGRPC:
			if !sameMethod(node.Proto, tmpl) {
				fail("declares proto %s but its template %q sends %s/%s; the node and its template name the same method",
					node.Proto.String(), node.Adapter, tmpl.Request.Service(), tmpl.Request.MethodName())
			}
		}
	}
	return report
}

// sameMethod reports whether a node's proto ref and its template's rpc name the
// same method. Both sides are already split, so the dotted and slashed
// spellings of a full method compare equal.
func sameMethod(ref *graph.ProtoRef, tmpl *adapter.Template) bool {
	return ref.Service == tmpl.Request.Service() && ref.Method == tmpl.Request.MethodName()
}
