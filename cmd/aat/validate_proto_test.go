package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gburgyan/aat/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// grpcProject writes a project whose one gRPC node is described by a template
// and a descriptor set. manifestProto and graphProto choose which file names
// the descriptor set; protocol is the template's.
func grpcProject(t *testing.T, manifestProto, graphProto bool, protocol, rpc string) string {
	t.Helper()
	dir := t.TempDir()
	dir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	set := testutil.DescriptorSetBytes(t, testutil.ShopFile())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "shop.protoset"), set, 0644))

	manifest := "name: grpctest\ngraph: graph.yaml\ntemplates: templates/\n"
	if manifestProto {
		manifest += "proto: shop.protoset\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "aat-project.yaml"), []byte(manifest), 0644))

	g := "version: \"1.0.0\"\ntitle: t\n"
	if graphProto {
		g += "proto: shop.protoset\n"
	}
	g += `nodes:
  createCart:
    description: Open a cart
    adapter: createCart
    proto: shop.v1.Carts/CreateCart
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "graph.yaml"), []byte(g), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "templates"), 0755))
	tmpl := "adapter: createCart\n"
	if protocol == "grpc" {
		tmpl += "protocol: grpc\nrequest:\n  rpc: " + rpc + "\n  message: \"{}\"\n"
	} else {
		tmpl += "request:\n  method: POST\n  path: /carts\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "templates", "createCart.yaml"), []byte(tmpl), 0644))
	return dir
}

func runValidate(t *testing.T, dir string, strict bool) (string, int) {
	t.Helper()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origDir) }()
	t.Setenv("AAT_PROJECT", "")
	require.NoError(t, os.Chdir(dir))

	var buf bytes.Buffer
	code := validateCommand(&validateArgs{Strict: strict}, &buf)
	return buf.String(), code
}

// The manifest form docs/user/grpc.md teaches first: a descriptor set named in
// aat-project.yaml and nowhere else.
func TestValidate_ManifestProtoOnly(t *testing.T) {
	out, code := runValidate(t, grpcProject(t, true, false, "grpc", "shop.v1.Carts/CreateCart"), true)

	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, "Protobuf validation: OK", "the section must render, not vanish")
	assert.Contains(t, out, "Node protocols:      OK")
}

func TestValidate_ManifestProtoCatchesABadMethod(t *testing.T) {
	dir := grpcProject(t, true, false, "grpc", "shop.v1.Carts/CreateCart")
	g, err := os.ReadFile(filepath.Join(dir, "graph.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "graph.yaml"),
		bytes.ReplaceAll(g, []byte("CreateCart"), []byte("Nope")), 0644))

	out, code := runValidate(t, dir, true)
	assert.Equal(t, 1, code)
	assert.Contains(t, out, "Nope")
}

// A file named in both places loads once, and both refs stay resolvable.
func TestValidate_ManifestAndGraphNameTheSameDescriptorSet(t *testing.T) {
	out, code := runValidate(t, grpcProject(t, true, true, "grpc", "shop.v1.Carts/CreateCart"), true)

	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, "Protobuf validation: OK")
}

func TestValidate_NoDescriptorSetAnywhereFails(t *testing.T) {
	out, code := runValidate(t, grpcProject(t, false, false, "grpc", "shop.v1.Carts/CreateCart"), false)

	assert.Equal(t, 1, code)
	assert.Contains(t, out, "names no descriptor set")
	assert.Contains(t, out, "aat-project.yaml")
}

func TestValidate_ProtoNodeWithHTTPTemplateFails(t *testing.T) {
	out, code := runValidate(t, grpcProject(t, true, false, "http", ""), false)

	assert.Equal(t, 1, code)
	assert.Contains(t, out, "Node protocols:")
	assert.Contains(t, out, "set protocol: grpc on the template")
}

func TestValidate_ProtoAndRPCDisagreeFails(t *testing.T) {
	out, code := runValidate(t, grpcProject(t, true, false, "grpc", "shop.v1.Carts/GetCart"), false)

	assert.Equal(t, 1, code)
	assert.Contains(t, out, "name the same method")
}

// A project that sets up descriptors before writing its first gRPC node is
// mid-build; --strict must not fail it.
func TestValidate_ManifestProtoWithNoGRPCNodes(t *testing.T) {
	dir := grpcProject(t, true, false, "http", "")
	g, err := os.ReadFile(filepath.Join(dir, "graph.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "graph.yaml"),
		bytes.ReplaceAll(g, []byte("    proto: shop.v1.Carts/CreateCart\n"), nil), 0644))

	out, code := runValidate(t, dir, true)
	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, "no gRPC nodes")
	assert.Contains(t, out, "a node is checked against it once it names a method")
}
