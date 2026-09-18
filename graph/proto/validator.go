package proto

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/internal/gjsonpath"
	"github.com/gburgyan/aat/internal/protoreg"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Validator implements graph.SpecValidator for protobuf descriptor sets.
type Validator struct {
	// registries holds one registry per descriptor set, by the path the graph
	// refers to it by.
	registries map[string]*protoreg.Registry
	// outputPaths maps node name → output name → the GJSON path the node's
	// template extracts it from, so the check follows the template rather than
	// assuming a top-level field named after the output.
	outputPaths OutputPaths
	// inputPaths maps node name → input name → where the template's message
	// places the input, so the check follows the template rather than
	// expecting a top-level field named after the input.
	inputPaths InputPaths
	// projectRefs names descriptor sets that apply to the whole project, for a
	// node whose graph names none.
	projectRefs []string
}

// OutputPaths maps node name → output name → extract path. It mirrors
// oas.OutputPaths, and engine.OutputExtractPaths builds both.
type OutputPaths map[string]map[string]string

// InputPaths maps node name → input name → the GJSON paths at which the
// node's template places the input in its request message. An input with no
// paths reaches the request without a field to check, such as in metadata.
// engine.TemplateMessageInputPaths builds it.
type InputPaths map[string]map[string][]string

// NewValidator creates a validator with no descriptor sets loaded.
func NewValidator() *Validator {
	return &Validator{registries: make(map[string]*protoreg.Registry)}
}

// WithOutputPaths makes the output check look for each output at its template
// extract path instead of at a field named after the output.
func (v *Validator) WithOutputPaths(paths OutputPaths) *Validator {
	v.outputPaths = paths
	return v
}

// WithInputPaths makes the input check look for each input where the
// template's message places it. A node absent from paths, or an input absent
// from its node's entry, is checked by name.
func (v *Validator) WithInputPaths(paths InputPaths) *Validator {
	v.inputPaths = paths
	return v
}

// WithProjectDescriptors names the descriptor sets aat-project.yaml declares.
// They stand in for a graph that names none of its own.
//
// The refs are the paths as graph.ResolveSpecPaths yields them for a project
// entry — already resolved against the manifest's directory — because that is
// the key LoadSpec stores them under. They are deliberately absent from
// CollectSpecPaths, which returns only what the graph writes: a caller merges
// the two with graph.ResolveSpecPaths, which knows that a project path must
// never be joined onto the graph's directory.
func (v *Validator) WithProjectDescriptors(refs []string) *Validator {
	v.projectRefs = refs
	return v
}

// CountNodes returns how many of a graph's nodes name a gRPC method.
func CountNodes(g *graph.Graph) int {
	n := 0
	for _, node := range g.Nodes {
		if node != nil && node.Proto != nil {
			n++
		}
	}
	return n
}

// CollectSpecPaths returns the descriptor sets the graph refers to: its own,
// and any a node names for itself. A project's own descriptor sets are not
// here — see WithProjectDescriptors.
func (v *Validator) CollectSpecPaths(g *graph.Graph) []string {
	seen := make(map[string]bool)
	var paths []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}

	add(g.Proto)
	for _, node := range g.Nodes {
		if node.Proto != nil && node.Proto.Descriptor != "" {
			add(node.Proto.Descriptor)
		}
	}
	return paths
}

// LoadSpec loads a descriptor set and stores it under the path the graph refers
// to it by. refPath is the key written in YAML; fsPath is where it resolved to.
func (v *Validator) LoadSpec(refPath, fsPath string) error {
	reg, err := protoreg.LoadDescriptorSets(fsPath)
	if err != nil {
		return err
	}
	v.registries[refPath] = reg
	return nil
}

// ResolveNodeDescriptor returns the descriptor set a node is checked against:
// its own when it names one, otherwise the graph's.
func ResolveNodeDescriptor(node *graph.Node, graphProto string) string {
	if node.Proto != nil && node.Proto.Descriptor != "" {
		return node.Proto.Descriptor
	}
	return graphProto
}

// descriptorRefs lists the descriptor sets a node is checked against, in the
// order they are tried: its own when it names one, otherwise the graph's,
// otherwise the project's. A node naming its own descriptor set means it, so
// the project's are not also tried.
func (v *Validator) descriptorRefs(node *graph.Node, graphProto string) []string {
	if ref := ResolveNodeDescriptor(node, graphProto); ref != "" {
		return []string{ref}
	}
	return v.projectRefs
}

// Validate cross-references every node carrying a proto ref against the loaded
// descriptors.
func (v *Validator) Validate(g *graph.Graph) *graph.SpecValidationResult {
	result := &graph.SpecValidationResult{}

	names := make([]string, 0, len(g.Nodes))
	for name := range g.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		node := g.Nodes[name]
		if node.Proto == nil {
			continue
		}
		if node.OAS != nil {
			result.Issues = append(result.Issues, graph.SpecValidationIssue{
				Severity: graph.SpecError, Node: name,
				Message: "node has both oas and proto; an operation is described by one contract",
			})
			continue
		}
		v.validateNode(result, name, node, g.Proto)
	}
	return result
}

func (v *Validator) validateNode(result *graph.SpecValidationResult, name string, node *graph.Node, graphProto string) {
	fail := func(format string, args ...any) {
		result.Issues = append(result.Issues, graph.SpecValidationIssue{
			Severity: graph.SpecError, Node: name, Message: fmt.Sprintf(format, args...),
		})
	}
	warn := func(format string, args ...any) {
		result.Issues = append(result.Issues, graph.SpecValidationIssue{
			Severity: graph.SpecWarning, Node: name, Message: fmt.Sprintf(format, args...),
		})
	}

	refs := v.descriptorRefs(node, graphProto)
	if len(refs) == 0 {
		fail("proto %s needs a descriptor set: name one on the node, on the graph, or with proto: in aat-project.yaml", node.Proto.String())
		return
	}

	// With one candidate the error is whatever protoreg.Method says, including
	// its "did you mean"; with several, no single set's suggestion is the
	// answer, so the sets are named instead.
	var md protoreflect.MethodDescriptor
	var firstErr error
	var loaded []string
	for _, ref := range refs {
		reg, ok := v.registries[ref]
		if !ok {
			// The path was collected but did not load; LoadSpec reported why.
			continue
		}
		loaded = append(loaded, ref)
		found, err := reg.Method(node.Proto.Service, node.Proto.Method)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		md = found
		break
	}
	switch {
	case md != nil:
	case len(loaded) == 0:
		return
	case len(loaded) == 1:
		fail("%s", firstErr)
		return
	default:
		fail("%s is in none of the project's descriptor sets (%s)", node.Proto.String(), strings.Join(loaded, ", "))
		return
	}

	if md.IsStreamingClient() || md.IsStreamingServer() {
		fail("%s is %s; aat runs unary methods, where one request has one response", node.Proto.String(), streamKind(md))
		return
	}

	v.checkInputs(fail, warn, name, node, md.Input())
	v.checkOutputs(fail, name, node, md.Output())
}

// checkInputs reports inputs the request message does not declare. An input
// the template places in its message is checked where it goes; one it doesn't
// place must name a top-level field.
func (v *Validator) checkInputs(fail, warn func(string, ...any), name string, node *graph.Node, msg protoreflect.MessageDescriptor) {
	placements := v.inputPaths[name]
	for _, in := range node.Inputs {
		if paths, placed := placements[in.Name]; placed {
			// The codec rejects a field the message doesn't declare, so a
			// placement that doesn't resolve fails every request.
			for _, path := range paths {
				if res := walkPath(msg, path); res.problem != "" {
					fail("input %q is sent at %q: %s", in.Name, path, res.problem)
				}
			}
			continue
		}
		if fieldByAnyName(msg, in.Name) != nil {
			continue
		}
		if suggestion := suggestField(msg, in.Name); suggestion != "" {
			fail("input %q is not a field of %s%s", in.Name, msg.FullName(), suggestion)
			continue
		}
		// An input the message does not declare may still be legitimate: a
		// template can send it as metadata, or use it to build another field.
		warn("input %q is not a field of %s; it must reach the request another way, such as metadata", in.Name, msg.FullName())
	}
}

// checkOutputs reports outputs the response message does not declare, following
// the template's extract path where there is one.
func (v *Validator) checkOutputs(fail func(string, ...any), name string, node *graph.Node, msg protoreflect.MessageDescriptor) {
	paths := v.outputPaths[name]
	for _, out := range node.Outputs {
		path, hasPath := paths[out.Name]
		if hasPath && path == "" {
			continue // computed by a response transform; nothing to locate
		}
		if !hasPath {
			path = out.Name
		}
		trimmed := strings.TrimPrefix(path, "$.")
		res := walkPath(msg, trimmed)
		if res.problem != "" {
			fail("output %q reads %q: %s", out.Name, path, res.problem)
			continue
		}
		checkJSONNames(fail, out.Name, path, trimmed, res.protoNames)
	}
}

// checkJSONNames reports an extract path that spells any field by its proto
// name where the response encodes it under its JSON name. Both spellings
// resolve against the descriptor, so the path would silently read nothing at
// run time. The report gives the path to read instead.
func checkJSONNames(fail func(string, ...any), output, path, trimmed string, uses []protoNameUse) {
	if len(uses) == 0 {
		return
	}
	segs := gjsonpath.Split(trimmed)
	for _, u := range uses {
		segs[u.segment] = gjsonpath.KeySegment(u.field.JSONName())
	}
	corrected := gjsonpath.Join(segs)
	if len(uses) == 1 {
		fail("output %q reads %q, but the response encodes that field as %q: read %q", output, path, uses[0].field.JSONName(), corrected)
		return
	}
	fail("output %q reads %q, but the response encodes fields by their JSON names: read %q", output, path, corrected)
}

// fieldByAnyName finds a field by its proto name or its JSON name, because a
// template may be written either way.
func fieldByAnyName(msg protoreflect.MessageDescriptor, name string) protoreflect.FieldDescriptor {
	fields := msg.Fields()
	if fd := fields.ByName(protoreflect.Name(name)); fd != nil {
		return fd
	}
	return fields.ByJSONName(name)
}

// suggestField names the field a misspelling most likely meant: one that
// differs only in case or in underscores.
func suggestField(msg protoreflect.MessageDescriptor, name string) string {
	if name == "" {
		return ""
	}
	want := normalize(name)
	fields := msg.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if normalize(string(fd.Name())) == want || normalize(fd.JSONName()) == want {
			return fmt.Sprintf(" (did you mean %q?)", fd.JSONName())
		}
	}
	return ""
}

// normalize folds case and drops underscores, so "order_id", "orderId", and
// "OrderID" compare equal.
func normalize(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "_", ""))
}

func streamKind(md protoreflect.MethodDescriptor) string {
	switch {
	case md.IsStreamingClient() && md.IsStreamingServer():
		return "a bidirectional streaming method"
	case md.IsStreamingClient():
		return "a client-streaming method"
	default:
		return "a server-streaming method"
	}
}
