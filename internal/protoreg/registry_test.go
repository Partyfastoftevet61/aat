package protoreg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDescriptorSets(t *testing.T) {
	reg := shopRegistry(t)
	assert.Equal(t, []string{"shop.v1.Carts"}, reg.Services())

	md, err := reg.Method("shop.v1.Carts", "CreateCart")
	require.NoError(t, err)
	assert.Equal(t, "shop.v1.CreateCartRequest", string(md.Input().FullName()))
	assert.Equal(t, "shop.v1.Cart", string(md.Output().FullName()))
	assert.False(t, md.IsStreamingServer())
	assert.False(t, md.IsStreamingClient())
}

func TestLoadDescriptorSets_Errors(t *testing.T) {
	t.Run("no paths", func(t *testing.T) {
		_, err := LoadDescriptorSets()
		require.ErrorContains(t, err, "no descriptor set given")
	})

	t.Run("missing file names the path", func(t *testing.T) {
		_, err := LoadDescriptorSets(filepath.Join(t.TempDir(), "absent.protoset"))
		require.ErrorContains(t, err, "absent.protoset")
	})

	t.Run("proto source instead of a descriptor set says so", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "carts.proto")
		require.NoError(t, os.WriteFile(path, []byte("syntax = \"proto3\";\npackage shop.v1;\n"), 0o600))
		_, err := LoadDescriptorSets(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "protoc --descriptor_set_out")
	})

	t.Run("empty set", func(t *testing.T) {
		_, err := LoadDescriptorSets(writeDescriptorSet(t, "empty.protoset"))
		require.ErrorContains(t, err, "declares no files")
	})
}

func TestLoadDescriptorSets_OverlappingSetsKeepOneCopy(t *testing.T) {
	// Two sets built from overlapping imports repeat their shared files.
	a := writeDescriptorSet(t, "a.protoset", shopFile())
	b := writeDescriptorSet(t, "b.protoset", shopFile())

	reg, err := LoadDescriptorSets(a, b)
	require.NoError(t, err)
	assert.Equal(t, []string{"shop.v1.Carts"}, reg.Services())
}

func TestRegistry_MethodErrors(t *testing.T) {
	reg := shopRegistry(t)

	t.Run("unknown service", func(t *testing.T) {
		_, err := reg.Method("shop.v1.Orders", "Create")
		require.ErrorContains(t, err, `service "shop.v1.Orders" not found`)
	})

	t.Run("service named without its package suggests the full name", func(t *testing.T) {
		_, err := reg.Method("Carts", "CreateCart")
		require.ErrorContains(t, err, `did you mean "shop.v1.Carts"?`)
	})

	t.Run("unknown method suggests a near miss", func(t *testing.T) {
		_, err := reg.Method("shop.v1.Carts", "createCart")
		require.ErrorContains(t, err, `did you mean "CreateCart"?`)
	})

	t.Run("a message is not a service", func(t *testing.T) {
		_, err := reg.Method("shop.v1.Cart", "CreateCart")
		require.ErrorContains(t, err, "is a message, not a service")
	})
}

func TestRegistry_StreamingMethodIsRecognised(t *testing.T) {
	reg := shopRegistry(t)
	md, err := reg.Method("shop.v1.Carts", "WatchCart")
	require.NoError(t, err)
	assert.True(t, md.IsStreamingServer(), "the validator rejects non-unary methods on this")
}
