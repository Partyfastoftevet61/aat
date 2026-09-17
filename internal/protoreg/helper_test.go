package protoreg

import (
	"testing"

	"github.com/gburgyan/aat/internal/testutil"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/descriptorpb"
)

func shopFile() *descriptorpb.FileDescriptorProto { return testutil.ShopFile() }

func writeDescriptorSet(t *testing.T, name string, files ...*descriptorpb.FileDescriptorProto) string {
	t.Helper()
	return testutil.WriteDescriptorSet(t, name, files...)
}

// shopRegistry loads the shop descriptors.
func shopRegistry(t *testing.T) *Registry {
	t.Helper()
	reg, err := LoadDescriptorSets(writeDescriptorSet(t, "shop.protoset", shopFile()))
	require.NoError(t, err)
	return reg
}
