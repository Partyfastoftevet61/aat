package main

import (
	"fmt"
	"path/filepath"

	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/graph/proto"
	"github.com/gburgyan/aat/internal/protoreg"
)

// loadProtoRegistry loads the descriptor sets a graph's gRPC nodes need,
// resolving relative paths against the graph's directory. It returns nil when
// the graph has no gRPC nodes, which is every HTTP-only project.
//
// Unlike the OpenAPI cache, which validates what a run sends and can be turned
// off, descriptors are what a gRPC request is built from. A graph that names
// them and cannot load them cannot run, so the error is always fatal.
func loadProtoRegistry(g *graph.Graph, graphPath string) (*protoreg.Registry, error) {
	paths := proto.NewValidator().CollectSpecPaths(g)
	if len(paths) == 0 {
		return nil, nil
	}

	graphDir := filepath.Dir(graphPath)
	resolved := make([]string, len(paths))
	for i, p := range paths {
		resolved[i] = p
		if !filepath.IsAbs(p) {
			resolved[i] = filepath.Join(graphDir, p)
		}
	}

	reg, err := protoreg.LoadDescriptorSets(resolved...)
	if err != nil {
		return nil, fmt.Errorf("loading descriptors: %w", err)
	}
	return reg, nil
}
