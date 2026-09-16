package archive

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// stripeBody is a form body as an archive stores one: a JSON string, since the
// body itself isn't JSON.
func stripeBody(t *testing.T, text string) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(text)
	assert.NoError(t, err)
	return encoded
}

func TestBodyText(t *testing.T) {
	assert.Equal(t, "a=1&b=2", BodyText(stripeBody(t, "a=1&b=2")), "a body archived as a JSON string is its text")
	assert.Equal(t, `{"a":1}`, BodyText(json.RawMessage(`{"a":1}`)), "a JSON body is its own bytes")
	assert.Equal(t, "", BodyText(nil))
}

func TestIsFormMediaType(t *testing.T) {
	assert.True(t, IsFormMediaType("application/x-www-form-urlencoded"))
	assert.True(t, IsFormMediaType("application/x-www-form-urlencoded; charset=utf-8"))
	assert.True(t, IsFormMediaType("Application/X-WWW-Form-Urlencoded"))
	assert.False(t, IsFormMediaType("application/json"))
	assert.False(t, IsFormMediaType(""))
}

func TestHeaderValue(t *testing.T) {
	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Idempotency-Key": "k"}
	assert.Equal(t, "application/x-www-form-urlencoded", HeaderValue(headers, "content-type"))
	assert.Equal(t, "k", HeaderValue(headers, "Idempotency-Key"))
	assert.Equal(t, "", HeaderValue(headers, "Accept"))
	assert.Equal(t, "", HeaderValue(nil, "Content-Type"))
}

func TestFormFields(t *testing.T) {
	fields := FormFields(stripeBody(t, "email=ada%40example.com&name=AAT+Stripe&metadata[source]=aat-stripe"))
	assert.Equal(t, []FormField{
		{Name: "email", Value: "ada@example.com"},
		{Name: "name", Value: "AAT Stripe"},
		{Name: "metadata[source]", Value: "aat-stripe"},
	}, fields, "fields keep the order they were sent, with escapes undone")

	assert.Nil(t, FormFields(nil))
	assert.Equal(t, []FormField{{Name: "a", Value: ""}}, FormFields(stripeBody(t, "a=")))
	assert.Equal(t, []FormField{{Name: "flag", Value: ""}}, FormFields(stripeBody(t, "flag")), "a field with no value")
	assert.Equal(t, []FormField{{Name: "a%zz", Value: "1"}}, FormFields(stripeBody(t, "a%zz=1")), "escapes that don't decode stay as they were sent")
}

func TestFormObject(t *testing.T) {
	tests := []struct {
		name string
		body string
		want map[string]any
	}{
		{
			name: "flat fields",
			body: "amount=2000&currency=usd",
			want: map[string]any{"amount": "2000", "currency": "usd"},
		},
		{
			name: "bracketed keys nest",
			body: "metadata[source]=aat-stripe&payment_method_options[card][cvc_token]=cvctok_1",
			want: map[string]any{
				"metadata":               map[string]any{"source": "aat-stripe"},
				"payment_method_options": map[string]any{"card": map[string]any{"cvc_token": "cvctok_1"}},
			},
		},
		{
			name: "a key ending in [] collects an array",
			body: "payment_method_types[]=card&payment_method_types[]=link&expand[]=latest_charge",
			want: map[string]any{
				"payment_method_types": []any{"card", "link"},
				"expand":               []any{"latest_charge"},
			},
		},
		{
			name: "a repeated key collects an array",
			body: "amounts=32&amounts=45",
			want: map[string]any{"amounts": []any{"32", "45"}},
		},
		{
			name: "an index stays a key, which a path reads as items.0.sku",
			body: "items[0][sku]=SKU-1&items[1][sku]=SKU-2",
			want: map[string]any{"items": map[string]any{
				"0": map[string]any{"sku": "SKU-1"},
				"1": map[string]any{"sku": "SKU-2"},
			}},
		},
		{
			name: "a key that isn't a name followed by brackets is one name",
			body: "a[b=1",
			want: map[string]any{"a[b": "1"},
		},
		{
			name: "a field nesting under an earlier scalar replaces it",
			body: "a=1&a[b]=2",
			want: map[string]any{"a": map[string]any{"b": "2"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormObject(stripeBody(t, tt.body)))
		})
	}
}
