package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveSpecPaths_GraphOnly(t *testing.T) {
	paths := ResolveSpecPaths([]string{"openapi.yaml", "other/spec.yaml"}, "project", nil)

	assert.Equal(t, []SpecPath{
		{Ref: "openapi.yaml", Path: "project/openapi.yaml"},
		{Ref: "other/spec.yaml", Path: "project/other/spec.yaml"},
	}, paths)
}

func TestResolveSpecPaths_GraphAbsoluteIsLeftAlone(t *testing.T) {
	paths := ResolveSpecPaths([]string{"/etc/aat/openapi.yaml"}, "project", nil)

	assert.Equal(t, []SpecPath{{Ref: "/etc/aat/openapi.yaml", Path: "/etc/aat/openapi.yaml"}}, paths)
}

func TestResolveSpecPaths_ProjectOnly(t *testing.T) {
	paths := ResolveSpecPaths(nil, "project", []string{"project/payments.protoset"})

	assert.Equal(t, []SpecPath{{Ref: "project/payments.protoset", Path: "project/payments.protoset"}}, paths)
}

// A manifest loaded through a relative path yields relative-but-already-resolved
// entries. Joining those onto the graph's directory doubled the prefix.
func TestResolveSpecPaths_ProjectPathNotJoinedOntoGraphDir(t *testing.T) {
	paths := ResolveSpecPaths(nil, "examples/shop", []string{"examples/shop/openapi.yaml"})

	assert.Equal(t, []SpecPath{{Ref: "examples/shop/openapi.yaml", Path: "examples/shop/openapi.yaml"}}, paths,
		"a project path is used exactly as given, however relative it looks")
}

func TestResolveSpecPaths_GraphRefComesFirst(t *testing.T) {
	paths := ResolveSpecPaths([]string{"payments.protoset"}, "project", []string{"project/payments.protoset"})

	assert.Equal(t, []SpecPath{
		{Ref: "payments.protoset", Path: "project/payments.protoset"},
		{Ref: "project/payments.protoset", Path: "project/payments.protoset"},
	}, paths, "both refs stay loadable; a node may name either")
	assert.Equal(t, []string{"project/payments.protoset"}, SpecPathList(paths),
		"but the file is loaded once")
}

func TestResolveSpecPaths_DuplicateRefsCollapse(t *testing.T) {
	paths := ResolveSpecPaths([]string{"a.yaml", "a.yaml"}, "project", []string{"b.yaml", "b.yaml"})

	assert.Equal(t, []SpecPath{
		{Ref: "a.yaml", Path: "project/a.yaml"},
		{Ref: "b.yaml", Path: "b.yaml"},
	}, paths)
}

func TestResolveSpecPaths_Empty(t *testing.T) {
	assert.Empty(t, ResolveSpecPaths(nil, "project", nil))
	assert.Empty(t, ResolveSpecPaths([]string{""}, "project", []string{""}), "an empty ref names no file")
}

func TestSpecPathList_DedupsOnPath(t *testing.T) {
	list := SpecPathList([]SpecPath{
		{Ref: "a.yaml", Path: "project/a.yaml"},
		{Ref: "project/a.yaml", Path: "project/a.yaml"},
		{Ref: "b.yaml", Path: "project/b.yaml"},
	})

	assert.Equal(t, []string{"project/a.yaml", "project/b.yaml"}, list)
}
