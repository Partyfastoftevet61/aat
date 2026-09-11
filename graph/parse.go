package graph

import (
	"fmt"
	"os"
	"strings"

	"github.com/gburgyan/aat/internal/yamlx"
)

// Parse unmarshals YAML bytes into a Graph, populates node names
// from map keys, and validates the result. Keys that no graph field accepts
// are errors.
func Parse(data []byte) (*Graph, error) {
	var g Graph
	if err := yamlx.Decode(data, &g); err != nil {
		return nil, err
	}

	// Populate Node.Name from map keys
	for name, node := range g.Nodes {
		if node == nil {
			continue
		}
		node.Name = name
	}

	if err := Validate(&g); err != nil {
		return nil, err
	}

	g.BuildSatisfierIndex()

	return &g, nil
}

// ParseFile reads a YAML file from disk and parses it into a Graph. Errors
// name the file.
func ParseFile(path string) (*Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading graph file: %w", err)
	}
	g, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return g, nil
}

// splitRef splits a "node.field" reference into its components.
func splitRef(ref string) (nodeName, fieldName string, err error) {
	parts := strings.SplitN(ref, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected format \"node.field\", got %q", ref)
	}
	return parts[0], parts[1], nil
}
