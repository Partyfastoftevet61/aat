package engine

import "github.com/gburgyan/aat/graph"

// echoInputOutputs sets each output the node declares with fromInput to the
// input it names, as the step resolved it, and returns outputs (made when it
// was nil and something is echoed). It runs after extraction, so an echoed
// output is there for later steps, the node's cleanup, and assertions as if
// the response had carried it. An input the step left out leaves the output
// unset, as a missing path leaves an optional extracted output.
func echoInputOutputs(outputs map[string]any, node *graph.Node, inputs map[string]any) map[string]any {
	for _, out := range node.Outputs {
		if out.FromInput == "" {
			continue
		}
		v, ok := inputs[out.FromInput]
		if !ok {
			continue
		}
		if outputs == nil {
			outputs = make(map[string]any)
		}
		outputs[out.Name] = v
	}
	return outputs
}
