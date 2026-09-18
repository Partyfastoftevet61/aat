package adapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONPlaceholderPaths(t *testing.T) {
	tests := []struct {
		name string
		text string
		want map[string][]string // nil when the text can't be scanned
	}{
		{
			name: "flat",
			text: `{"collectionName": "{{collectionName}}", "wait": {{wait}}}`,
			want: map[string][]string{"collectionName": {"collectionName"}, "wait": {"wait"}},
		},
		{
			name: "nested objects",
			text: `{
			  "collectionName": "{{collectionName}}",
			  "vectorsConfig": {"params": {"size": "{{vectorSize}}", "distance": "{{distance}}"}}
			}`,
			want: map[string][]string{
				"collectionName": {"collectionName"},
				"vectorSize":     {"vectorsConfig.params.size"},
				"distance":       {"vectorsConfig.params.distance"},
			},
		},
		{
			name: "map values and arrays",
			text: `{"points": [{"id": {"num": "{{pointId}}"}, "payload": {"city": {"stringValue": "{{city}}"}}}, {"id": {"uuid": "{{otherId}}"}}]}`,
			want: map[string][]string{
				"pointId": {"points.0.id.num"},
				"city":    {"points.0.payload.city.stringValue"},
				"otherId": {"points.1.id.uuid"},
			},
		},
		{
			name: "a dotted key is escaped",
			text: `{"labels": {"my.key": "{{label}}"}}`,
			want: map[string][]string{"label": {`labels.my\.key`}},
		},
		{
			name: "an escaped key is decoded before escaping for gjson",
			text: `{"a\"b": "{{v}}"}`,
			want: map[string][]string{"v": {`a\"b`}}, // as gjson.Escape writes it
		},
		{
			name: "part of a string",
			text: `{"name": "aat-{{prefix}}-{{suffix}}"}`,
			want: map[string][]string{"prefix": {"name"}, "suffix": {"name"}},
		},
		{
			name: "a whole value that is a list or object",
			text: `{"vector": {{vector}}, "payload": {{payload}}}`,
			want: map[string][]string{"vector": {"vector"}, "payload": {"payload"}},
		},
		{
			name: "conditionals, comma leading and trailing",
			text: `{
			  "collectionName": "{{collectionName}}"{{?wait}},
			  "wait": {{wait}}{{/wait}}{{?limit}},
			  "limit": "{{limit}}"{{/limit}}
			}`,
			want: map[string][]string{"collectionName": {"collectionName"}, "wait": {"wait"}, "limit": {"limit"}},
		},
		{
			name: "a conditional as the first member",
			text: `{ {{?service}}"service": "{{service}}"{{/service}} }`,
			want: map[string][]string{"service": {"service"}},
		},
		{
			name: "a compound conditional",
			text: `{"a": 1{{?x|y}}, "filter": {"must": "{{x}}"}{{/x|y}}}`,
			want: map[string][]string{"x": {"filter.must"}},
		},
		{
			name: "iteration over scalars",
			text: `{"vector": [{{#vector}}{{.}}{{/vector}}]}`,
			want: map[string][]string{"vector": {"vector"}},
		},
		{
			name: "iteration over objects",
			text: `{"ids": [{{#ids}}{"num": "{{.num}}", "rank": {{@index}}}{{/ids}}]}`,
			want: map[string][]string{"ids": {"ids"}},
		},
		{
			name: "iteration writing map entries",
			text: `{"payload": { {{#fields}}"{{.key}}": {"stringValue": "{{.value}}"}{{/fields}} }}`,
			want: map[string][]string{"fields": {"payload"}},
		},
		{
			name: "a placeholder key",
			text: `{"payload": {"{{field}}": {"stringValue": "{{value}}"}}}`,
			want: map[string][]string{"field": {"payload.*"}, "value": {"payload.*.stringValue"}},
		},
		{
			name: "the same input twice",
			text: `{"a": "{{x}}", "b": {"c": "{{x}}"}, "d": "{{x}}"}`,
			want: map[string][]string{"x": {"a", "b.c", "d"}},
		},
		{
			name: "whitespace inside braces",
			text: `{"a": "{{ x }}"}`,
			want: map[string][]string{"x": {"a"}},
		},
		{name: "empty message", text: `{}`, want: map[string][]string{}},
		{name: "unbalanced", text: `{"a": {"b": "{{x}}"}`, want: nil},
		{name: "mismatched brackets", text: `{"a": [1}`, want: nil},
		{name: "unterminated string", text: `{"a": "{{x}}}`, want: nil},
		{name: "unterminated placeholder", text: `{"a": {{x}`, want: nil},
		{name: "not an object", text: `["{{x}}"]`, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := jsonPlaceholderPaths(tt.text)
			if tt.want == nil {
				assert.False(t, ok, "got %v", got)
				return
			}
			assert.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTemplate_MessageInputPaths(t *testing.T) {
	t.Run("metadata-only inputs are sent with no field to check", func(t *testing.T) {
		tmpl := &Template{Protocol: ProtocolGRPC, Request: TemplateRequest{
			RPC:      "qdrant.Collections/Get",
			Metadata: map[string]string{"x-tenant": "{{tenant}}", "x-trace": "run-{{collectionName}}"},
			Message:  `{"collectionName": "{{collectionName}}"}`,
		}}
		got, ok := tmpl.MessageInputPaths()
		assert.True(t, ok)
		assert.Equal(t, map[string][]string{"collectionName": {"collectionName"}, "tenant": {}}, got)
	})

	t.Run("an unreadable message says so", func(t *testing.T) {
		tmpl := &Template{Protocol: ProtocolGRPC, Request: TemplateRequest{Message: `{"a": `}}
		_, ok := tmpl.MessageInputPaths()
		assert.False(t, ok)
	})
}
