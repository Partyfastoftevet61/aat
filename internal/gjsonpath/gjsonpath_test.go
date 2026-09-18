package gjsonpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestSplit(t *testing.T) {
	key := func(k string) Segment { return Segment{Kind: Key, Key: k, Raw: k} }
	tests := []struct {
		path string
		want []Segment
	}{
		{"", nil},
		{"status", []Segment{key("status")}},
		{"result.config.params", []Segment{key("result"), key("config"), key("params")}},
		{"items.0.sku", []Segment{key("items"), key("0"), key("sku")}},
		{"items.#", []Segment{key("items"), {Kind: All, Key: "#", Raw: "#"}}},
		{"items.#.sku", []Segment{key("items"), {Kind: All, Key: "#", Raw: "#"}, key("sku")}},
		// An escaped dot is part of the key, not a separator.
		{`labels.my\.key`, []Segment{key("labels"), {Kind: Key, Key: "my.key", Raw: `my\.key`}}},
		{`a\\b.c`, []Segment{{Kind: Key, Key: `a\b`, Raw: `a\\b`}, key("c")}},
		// A query is one segment, dots and parentheses inside it included.
		{`items.#(sku=="a.b").quantity`, []Segment{key("items"), {Kind: Query, Raw: `#(sku=="a.b")`}, key("quantity")}},
		{`items.#(sku%"a(b")#.quantity`, []Segment{key("items"), {Kind: Query, Raw: `#(sku%"a(b")#`}, key("quantity")}},
		{`items.#(nested.#(x>1))`, []Segment{key("items"), {Kind: Query, Raw: `#(nested.#(x>1))`}}},
		{`items.#(sku=="a"`, []Segment{key("items"), {Kind: Opaque, Raw: `#(sku=="a"`}}},
		// @type is a key; only a registered modifier is a modifier.
		{"detail.@type", []Segment{key("detail"), key("@type")}},
		{"@type", []Segment{key("@type")}},
		{"items.@reverse", []Segment{key("items"), {Kind: Opaque, Raw: "@reverse"}}},
		{"@this", []Segment{{Kind: Opaque, Raw: "@this"}}},
		{"cartId|@tostr", []Segment{key("cartId"), {Kind: Opaque, Raw: "|@tostr"}}},
		{"a|b", []Segment{key("a"), {Kind: Opaque, Raw: "|b"}}},
		{"a.[b,c]", []Segment{key("a"), {Kind: Opaque, Raw: "[b,c]"}}},
		{"{a,b}", []Segment{{Kind: Opaque, Raw: "{a,b}"}}},
		{"!true", []Segment{{Kind: Opaque, Raw: "!true"}}},
		{"..0", []Segment{{Kind: Opaque, Raw: "..0"}}},
		// Wildcards match many keys; an escaped star is a plain key.
		{"meta.ci*", []Segment{key("meta"), {Kind: Wildcard, Key: "ci*", Raw: "ci*"}}},
		{`meta.a\*`, []Segment{key("meta"), {Kind: Key, Key: "a*", Raw: `a\*`}}},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := Split(tt.path)
			assert.Equal(t, tt.want, got)
			if len(got) > 0 {
				assert.Equal(t, tt.path, Join(got), "Join undoes Split")
			}
		})
	}
}

func TestSegment_IsIndex(t *testing.T) {
	assert.True(t, Segment{Kind: Key, Key: "0"}.IsIndex())
	assert.True(t, Segment{Kind: Key, Key: "12"}.IsIndex())
	assert.False(t, Segment{Kind: Key, Key: ""}.IsIndex())
	assert.False(t, Segment{Kind: Key, Key: "1a"}.IsIndex())
	assert.False(t, Segment{Kind: All, Key: "#"}.IsIndex())
	assert.False(t, Segment{Kind: Wildcard, Key: "1?"}.IsIndex())
}

func TestKeySegment_RoundTrips(t *testing.T) {
	// Any key, however awkward, escapes to a segment that Split reads back as
	// exactly that key.
	for _, k := range []string{"plain", "my.key", "a*b", "@type", "a|b", "#", "with space", `back\slash`} {
		t.Run(k, func(t *testing.T) {
			seg := KeySegment(k)
			got := Split("x." + seg.Raw)
			if assert.Len(t, got, 2) {
				assert.Equal(t, k, got[1].Key)
			}
		})
	}
}

func TestSplit_AgreesWithGJSON(t *testing.T) {
	// Following the key segments of a path by hand finds what gjson.Get finds.
	doc := `{"labels":{"my.key":"v","a*":"star"},"items":[{"sku":"a.b","quantity":2}],"detail":{"@type":"t"}}`
	for _, path := range []string{`labels.my\.key`, `labels.a\*`, `items.0.sku`, `detail.@type`, `items.0.quantity`} {
		t.Run(path, func(t *testing.T) {
			res := gjson.Parse(doc)
			for _, seg := range Split(path) {
				res = res.Get(gjson.Escape(seg.Key))
			}
			assert.Equal(t, gjson.Get(doc, path).Raw, res.Raw)
			assert.True(t, res.Exists())
		})
	}
}
