package graph

import (
	"testing"

	"github.com/gburgyan/aat/internal/yamlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestProtoRef_Unmarshal(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    ProtoRef
		wantErr string
	}{
		{
			name: "scalar, as the wire names it",
			yaml: "proto: shop.v1.Carts/CreateCart\n",
			want: ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
		},
		{
			name: "scalar with a leading slash",
			yaml: "proto: /shop.v1.Carts/CreateCart\n",
			want: ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
		},
		{
			name: "scalar as a protobuf full name",
			yaml: "proto: shop.v1.Carts.CreateCart\n",
			want: ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"},
		},
		{
			name: "mapping with its own descriptor set",
			yaml: "proto:\n  service: shop.v1.Carts\n  method: CreateCart\n  descriptor: carts.protoset\n",
			want: ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart", Descriptor: "carts.protoset"},
		},
		{
			name:    "scalar naming no method",
			yaml:    "proto: Carts\n",
			wantErr: `must name a service and a method`,
		},
		{
			name:    "mapping missing the method",
			yaml:    "proto:\n  service: shop.v1.Carts\n",
			wantErr: "needs both a service and a method",
		},
		{
			name:    "unknown key in the mapping",
			yaml:    "proto:\n  service: shop.v1.Carts\n  method: CreateCart\n  spec: carts.protoset\n",
			wantErr: `unknown key "spec"`,
		},
		{
			name:    "a list is neither form",
			yaml:    "proto: [shop.v1.Carts/CreateCart]\n",
			wantErr: "a service/method reference or a mapping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var holder struct {
				Proto *ProtoRef `yaml:"proto"`
			}
			err := yamlx.Decode([]byte(tt.yaml), &holder)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, holder.Proto)
			assert.Equal(t, tt.want, *holder.Proto)
		})
	}
}

func TestProtoRef_FullMethodAndString(t *testing.T) {
	ref := ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart"}
	assert.Equal(t, "/shop.v1.Carts/CreateCart", ref.FullMethod(), "the form gRPC puts on the wire")
	assert.Equal(t, "shop.v1.Carts/CreateCart", ref.String())
}

func TestProtoRef_MarshalRoundTrip(t *testing.T) {
	t.Run("scalar form when no descriptor is named", func(t *testing.T) {
		out, err := yaml.Marshal(map[string]ProtoRef{"proto": {Service: "shop.v1.Carts", Method: "CreateCart"}})
		require.NoError(t, err)
		assert.Equal(t, "proto: shop.v1.Carts/CreateCart\n", string(out))
	})

	t.Run("mapping form keeps the descriptor", func(t *testing.T) {
		ref := ProtoRef{Service: "shop.v1.Carts", Method: "CreateCart", Descriptor: "carts.protoset"}
		out, err := yaml.Marshal(ref)
		require.NoError(t, err)

		var back ProtoRef
		require.NoError(t, yamlx.Decode(out, &back))
		assert.Equal(t, ref, back)
	})
}

func TestNodeAcceptsProtoRef(t *testing.T) {
	g, err := Parse([]byte(`
version: "1.0.0"
proto: shop.protoset
nodes:
  createCart:
    description: Open a cart
    adapter: createCart
    proto: shop.v1.Carts/CreateCart
    inputs: []
    outputs:
      - name: cartId
        type: string
`))
	require.NoError(t, err)
	assert.Equal(t, "shop.protoset", g.Proto, "the graph names a default descriptor set")
	require.NotNil(t, g.Nodes["createCart"].Proto)
	assert.Equal(t, "shop.v1.Carts", g.Nodes["createCart"].Proto.Service)
	assert.Equal(t, "CreateCart", g.Nodes["createCart"].Proto.Method)
}
