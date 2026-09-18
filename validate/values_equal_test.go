package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected any
		want     bool
	}{
		{"string", `"Berlin"`, "Berlin", true},
		{"a string is not a number", `"4"`, 4, false},
		{"a number is not a string", `4`, "4", false},
		{"int against a JSON number", `4`, 4, true},
		{"int64 against a decimal", `4.0`, int64(4), true},
		{"uint64", `18446744073709551615`, uint64(18446744073709551615), true},
		{"float", `0.05`, 0.05, true},
		{"a large number that %v would print in exponent form", `3645000`, 3645000, true},
		{"bool", `true`, true, true},
		{"null", `null`, nil, true},
		{"null is not a value", `0`, nil, false},
		{"list of numbers", `[3,4,0,0]`, []any{3, 4, 0, 0}, true},
		{"list of strings is not a list of numbers", `[1]`, []any{"1"}, false},
		{"list of numbers is not a list of strings", `["1"]`, []any{1}, false},
		{"list length differs", `[1,2]`, []any{1}, false},
		{"list order matters", `["a","b"]`, []any{"b", "a"}, false},
		{"empty list", `[]`, []any{}, true},
		{"object with a nested large number", `{"city":"Berlin","population":3645000}`, map[string]any{"city": "Berlin", "population": 3645000}, true},
		{"object key order doesn't matter", `{"b":1,"a":2}`, map[string]any{"a": 2, "b": 1}, true},
		{"object with an extra key", `{"a":1,"data":[]}`, map[string]any{"a": 1}, false},
		{"object missing a key", `{"a":1}`, map[string]any{"a": 1, "b": 2}, false},
		{"nested string number stays a string", `{"id":{"num":"3"}}`, map[string]any{"id": map[string]any{"num": "3"}}, true},
		{"nested string number is not a number", `{"id":{"num":"3"}}`, map[string]any{"id": map[string]any{"num": 3}}, false},
		{"the empty key", `{"":{"x":1}}`, map[string]any{"": map[string]any{"x": 1}}, true},
		{"empty object", `{}`, map[string]any{}, true},
		{"a YAML map with non-string keys", `{"1":"a"}`, map[any]any{1: "a"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ValuesEqual(gjson.Parse(tt.json), tt.expected))
		})
	}
}

func TestCheckFieldEquals_DescribesListsAndObjectsAsJSON(t *testing.T) {
	body := []byte(`{"payload":{"city":"Berlin","population":3645000}}`)
	ar := checkFieldEquals(body, MechanicalAssertion{Type: AssertFieldEquals, Path: "payload", Value: map[string]any{"city": "Berlin", "population": "3645000"}})
	assert.False(t, ar.Passed)
	assert.Equal(t, `field "payload": expected {"city":"Berlin","population":"3645000"}, got {"city":"Berlin","population":3645000}`, ar.Message)
}
