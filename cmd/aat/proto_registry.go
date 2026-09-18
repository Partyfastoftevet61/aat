package main

import (
	"fmt"
	"path/filepath"

	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/graph/proto"
	"github.com/gburgyan/aat/internal/protoreg"
)

// loadProtoRegistry loads the descriptor sets a project's gRPC nodes need: the
// ones the graph names, resolved against the graph's directory, and the ones
// aat-project.yaml names, which arrive already resolved. It returns nil when
// neither names any, which is every HTTP-only project.
//
// Unlike the OpenAPI cache, which validates what a run sends and can be turned
// off, descriptors are what a gRPC request is built from. A project that names
// them and cannot load them cannot run, so the error is always fatal.
func loadProtoRegistry(g *graph.Graph, graphPath string, projectProto []string) (*protoreg.Registry, error) {
	specs := graph.ResolveSpecPaths(proto.NewValidator().CollectSpecPaths(g), filepath.Dir(graphPath), projectProto)
	paths := graph.SpecPathList(specs)
	if len(paths) == 0 {
		return nil, nil
	}

	reg, err := protoreg.LoadDescriptorSets(paths...)
	if err != nil {
		return nil, fmt.Errorf("loading descriptors: %w", err)
	}
	return reg, nil
}
