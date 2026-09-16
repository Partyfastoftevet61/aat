package archive

import (
	"encoding/json"
	"net/url"
	"strings"
)

// formMediaType is the media type of a form-encoded body.
const formMediaType = "application/x-www-form-urlencoded"

// maxFormKeyDepth bounds how deeply a form key's brackets nest, as the OpenAPI
// validator's own form decoder does.
const maxFormKeyDepth = 32

// FormField is one field of a form-encoded body, decoded, in the order the
// request sent it.
type FormField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HeaderValue returns a header's value, matched without regard to case, or ""
// when the headers don't have it.
func HeaderValue(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}

// IsFormMediaType reports whether a Content-Type names a form-encoded body.
func IsFormMediaType(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	return strings.EqualFold(strings.TrimSpace(mediaType), formMediaType)
}

// BodyText returns an archived body as the text it was sent as. A body that
// isn't JSON is archived as a JSON string, which this unquotes; any other body
// is its own bytes.
func BodyText(body json.RawMessage) string {
	var text string
	if json.Unmarshal(body, &text) == nil {
		return text
	}
	return string(body)
}

// FormFields decodes a form-encoded body into its fields, in the order they
// were sent, with %XX escapes and + undone. A field whose escapes don't decode
// keeps its text as it was sent.
func FormFields(body json.RawMessage) []FormField {
	text := BodyText(body)
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "&")
	fields := make([]FormField, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		name, value, _ := strings.Cut(part, "=")
		fields = append(fields, FormField{Name: unescapeForm(name), Value: unescapeForm(value)})
	}
	return fields
}

// unescapeForm undoes the escaping of one form name or value, leaving text it
// can't decode as it is.
func unescapeForm(s string) string {
	if decoded, err := url.QueryUnescape(s); err == nil {
		return decoded
	}
	return s
}

// FormObject decodes a form-encoded body into the value its bracketed keys
// describe, so a path can read it:
//   - metadata[source]=aat is {"metadata": {"source": "aat"}}
//   - a key ending in [], or one sent more than once, collects an array
//   - an index, as items[0][sku] writes, stays a key, which a path reads as
//     items.0.sku
//
// A key that isn't a name followed by bracketed segments is one literal name,
// and a field that would nest under an earlier scalar replaces it.
func FormObject(body json.RawMessage) map[string]any {
	root := map[string]any{}
	for _, field := range FormFields(body) {
		path, appendValue := splitFormKey(field.Name)
		setFormPath(root, path, appendValue, field.Value)
	}
	return root
}

// splitFormKey splits a form key into its path, metadata[source] into
// [metadata source], and reports whether it ends in [], which appends a value.
func splitFormKey(key string) (path []string, appendValue bool) {
	open := strings.IndexByte(key, '[')
	if open < 0 || !strings.HasSuffix(key, "]") {
		return []string{key}, false
	}
	path = []string{key[:open]}
	for rest := key[open:]; rest != ""; {
		if !strings.HasPrefix(rest, "[") {
			return []string{key}, false // not a name followed by brackets
		}
		end := strings.IndexByte(rest, ']')
		if end < 0 {
			return []string{key}, false
		}
		segment := rest[1:end]
		rest = rest[end+1:]
		if segment == "" && rest == "" {
			return path, true // the trailing [] of an array
		}
		if segment == "" || len(path) >= maxFormKeyDepth {
			return []string{key}, false
		}
		path = append(path, segment)
	}
	return path, false
}

// setFormPath sets value at path in root, nesting maps as it goes. A repeated
// key, and one that ends in [], collect an array.
func setFormPath(root map[string]any, path []string, appendValue bool, value string) {
	parent := root
	for _, segment := range path[:len(path)-1] {
		next, ok := parent[segment].(map[string]any)
		if !ok {
			next = map[string]any{}
			parent[segment] = next
		}
		parent = next
	}

	name := path[len(path)-1]
	switch current := parent[name].(type) {
	case nil:
		if appendValue {
			parent[name] = []any{value}
			return
		}
		parent[name] = value
	case []any:
		parent[name] = append(current, value)
	default:
		parent[name] = []any{current, value}
	}
}
