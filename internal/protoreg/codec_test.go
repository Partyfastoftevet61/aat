package protoreg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// cartDescriptor returns the shop.v1.Cart message descriptor.
func cartDescriptor(t *testing.T, reg *Registry) protoreflect.MessageDescriptor {
	t.Helper()
	md, err := reg.Method("shop.v1.Carts", "CreateCart")
	require.NoError(t, err)
	return md.Output()
}

func TestMessageToJSON_IsByteStable(t *testing.T) {
	// protojson varies its whitespace on purpose, which would make every
	// archive diff show changes that are not there. Compacting settles it.
	reg := shopRegistry(t)
	msg, err := reg.JSONToMessage(cartDescriptor(t, reg), []byte(`{"cartId":"c1","subtotal":"250"}`))
	require.NoError(t, err)

	first, err := reg.MessageToJSON(msg)
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		again, err := reg.MessageToJSON(msg)
		require.NoError(t, err)
		require.Equal(t, string(first), string(again), "the same message must always give the same bytes")
	}
}

func TestMessageToJSON_EmitsDefaultValues(t *testing.T) {
	// A proto3 scalar has no presence, so without EmitDefaultValues a field
	// that is legitimately zero would be absent and its extract rule would
	// fail. Empty lists are emitted for the same reason.
	reg := shopRegistry(t)
	msg, err := reg.JSONToMessage(cartDescriptor(t, reg), []byte(`{}`))
	require.NoError(t, err)

	out, err := reg.MessageToJSON(msg)
	require.NoError(t, err)
	assert.JSONEq(t, `{"cartId":"","subtotal":"0","status":"STATUS_UNSPECIFIED","items":[]}`, string(out))
	assert.NotContains(t, string(out), "couponCode",
		"an unset proto3 optional field stays absent, so fieldAbsent means something")
}

func TestMessageToJSON_Encodings(t *testing.T) {
	reg := shopRegistry(t)
	md := cartDescriptor(t, reg)

	msg, err := reg.JSONToMessage(md, []byte(`{
		"cartId": "c1",
		"subtotal": 4200,
		"status": "CHECKED_OUT",
		"items": [{"sku": "SKU-1", "quantity": 2}],
		"couponCode": "SAVE10"
	}`))
	require.NoError(t, err)
	out, err := reg.MessageToJSON(msg)
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"cartId": "c1",
		"subtotal": "4200",
		"status": "CHECKED_OUT",
		"items": [{"sku": "SKU-1", "quantity": 2}],
		"couponCode": "SAVE10"
	}`, string(out))

	// The encodings a plan author has to know about:
	assert.Contains(t, string(out), `"subtotal":"4200"`, "an int64 encodes as a JSON string")
	assert.Contains(t, string(out), `"quantity":2`, "an int32 stays a JSON number")
	assert.Contains(t, string(out), `"status":"CHECKED_OUT"`, "an enum encodes by name")
	assert.Contains(t, string(out), `"cartId"`, "field names are the canonical lowerCamelCase")
}

func TestJSONToMessage_AcceptsEitherFieldSpelling(t *testing.T) {
	// A request template may be written with proto names or JSON names; only
	// extract paths have to match, and graph/proto reports the ones that do not.
	reg := shopRegistry(t)
	md := cartDescriptor(t, reg)

	fromProtoNames, err := reg.JSONToMessage(md, []byte(`{"cart_id":"c1"}`))
	require.NoError(t, err)
	fromJSONNames, err := reg.JSONToMessage(md, []byte(`{"cartId":"c1"}`))
	require.NoError(t, err)

	a, err := reg.MessageToJSON(fromProtoNames)
	require.NoError(t, err)
	b, err := reg.MessageToJSON(fromJSONNames)
	require.NoError(t, err)
	assert.Equal(t, string(a), string(b))
}

func TestJSONToMessage_UnknownFieldIsAnError(t *testing.T) {
	// Unknown keys are errors in project YAML; a misspelled request field
	// should fail here rather than silently going out on the wire.
	reg := shopRegistry(t)
	_, err := reg.JSONToMessage(cartDescriptor(t, reg), []byte(`{"cartld":"c1"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shop.v1.Cart")
	assert.Contains(t, err.Error(), "cartld")
}

func TestJSONToMessage_MalformedJSON(t *testing.T) {
	reg := shopRegistry(t)
	_, err := reg.JSONToMessage(cartDescriptor(t, reg), []byte(`{"cartId":`))
	require.ErrorContains(t, err, "shop.v1.Cart")
}
