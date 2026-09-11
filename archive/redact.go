package archive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"
)

// Redacted replaces a credential in archives.
const Redacted = "[REDACTED]"

// minSubstringSecretLen is the length from which a secret is also replaced
// inside longer strings. A shorter secret, such as a sandbox's "demo" password,
// is an ordinary word in data too, so it is replaced only where a whole string
// equals it: {"password": "demo"}, but not demo@example.com.
const minSubstringSecretLen = 8

// sensitiveHeaders lists header names whose values should be redacted in archives.
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"x-api-key":           true,
	"x-auth-token":        true,
	"cookie":              true,
	"set-cookie":          true,
	"proxy-authorization": true,
}

// RedactHeaders returns a copy of headers in which the values of well-known
// credential headers, matched case-insensitively, are replaced by [REDACTED].
// These carry tokens AAT does not know as secrets, such as an OAuth2 access
// token. Known secrets under any other header are replaced by Redact.
func RedactHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}
	result := make(map[string]string, len(headers))
	for k, v := range headers {
		if sensitiveHeaders[strings.ToLower(k)] {
			result[k] = Redacted
		} else {
			result[k] = v
		}
	}
	return result
}

// Redact returns a deep copy of v in which every string holding a known secret
// has it replaced by [REDACTED]: URLs, request and response bodies, headers,
// outputs, resolved values, error and assertion messages, and the archived plan.
// A string that equals a secret is replaced whole. A secret of at least eight
// characters is also replaced inside longer strings, in its plain, URL
// query-escaped, and path-escaped forms; overlapping matches are replaced
// together, so no part of a longer secret survives. Inside JSON bodies only
// string values change, never keys, numbers, or formatting. Struct fields
// tagged redact:"-" hold identifiers such as step IDs and outcomes and are left
// alone. The copy is made through encoding/json, so v must be marshalable; with
// no secrets, v itself is returned.
func Redact[T any](v *T, secrets map[string]bool) (*T, error) {
	if v == nil || len(secrets) == 0 {
		return v, nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("redacting: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // numbers in outputs and bodies keep their exact text
	cp := new(T)
	if err := dec.Decode(cp); err != nil {
		return nil, fmt.Errorf("redacting: %w", err)
	}
	newRedactor(secrets).walk(reflect.ValueOf(cp).Elem())
	return cp, nil
}

// redactor replaces a set of secrets in strings.
type redactor struct {
	exact  map[string]bool // every form of every secret, replaced when a whole string equals it
	substr []string        // forms long enough to replace inside strings, longest first
}

func newRedactor(secrets map[string]bool) *redactor {
	r := &redactor{exact: make(map[string]bool)}
	for secret := range secrets {
		if secret == "" {
			continue
		}
		for _, form := range []string{secret, url.QueryEscape(secret), url.PathEscape(secret)} {
			if r.exact[form] {
				continue
			}
			r.exact[form] = true
			if len(form) >= minSubstringSecretLen {
				r.substr = append(r.substr, form)
			}
		}
	}
	sort.Slice(r.substr, func(i, j int) bool {
		if len(r.substr[i]) != len(r.substr[j]) {
			return len(r.substr[i]) > len(r.substr[j])
		}
		return r.substr[i] < r.substr[j]
	})
	return r
}

// redact returns s with its secrets replaced.
func (r *redactor) redact(s string) string {
	if r.exact[s] {
		return Redacted
	}
	var covered []bool
	for _, form := range r.substr {
		for from := 0; ; {
			i := strings.Index(s[from:], form)
			if i < 0 {
				break
			}
			if covered == nil {
				covered = make([]bool, len(s))
			}
			start := from + i
			for k := start; k < start+len(form); k++ {
				covered[k] = true
			}
			from = start + 1
		}
	}
	if covered == nil {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if !covered[i] {
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteString(Redacted)
		for i < len(s) && covered[i] {
			i++
		}
	}
	return b.String()
}

var (
	timeType       = reflect.TypeOf(time.Time{})
	rawMessageType = reflect.TypeOf(json.RawMessage(nil))
	numberType     = reflect.TypeOf(json.Number(""))
)

// walk redacts every string reachable from v, which must be settable.
func (r *redactor) walk(v reflect.Value) {
	switch v.Kind() {
	case reflect.String:
		if v.Type() != numberType && v.CanSet() {
			v.SetString(r.redact(v.String()))
		}
	case reflect.Pointer:
		if !v.IsNil() {
			r.walk(v.Elem())
		}
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		// A decoded any holds a string, a json.Number, a bool, a map, or a
		// slice. Copy it out to make it settable, then put the result back.
		inner := reflect.New(v.Elem().Type()).Elem()
		inner.Set(v.Elem())
		r.walk(inner)
		if v.CanSet() {
			v.Set(inner)
		}
	case reflect.Struct:
		if v.Type() == timeType {
			return
		}
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if field.IsExported() && field.Tag.Get("redact") != "-" {
				r.walk(v.Field(i))
			}
		}
	case reflect.Slice:
		if v.Type() == rawMessageType {
			if v.Len() > 0 {
				v.SetBytes(r.redactJSON(v.Bytes()))
			}
			return
		}
		for i := 0; i < v.Len(); i++ {
			r.walk(v.Index(i))
		}
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			r.walk(v.Index(i))
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			elem := reflect.New(v.Type().Elem()).Elem()
			elem.Set(v.MapIndex(key))
			r.walk(elem)
			v.SetMapIndex(key, elem)
		}
	}
}

// redactJSON redacts the string values of a JSON document in place of their
// tokens, leaving keys, numbers, and formatting untouched. It returns data
// unchanged when nothing matched or data is not valid JSON.
func (r *redactor) redactJSON(data []byte) []byte {
	if !json.Valid(data) {
		return data
	}
	var out []byte
	last := 0
	for i := 0; i < len(data); i++ {
		if data[i] != '"' {
			continue
		}
		end := i + 1
		for data[end] != '"' {
			if data[end] == '\\' {
				end++
			}
			end++
		}
		token := data[i : end+1]
		next := end + 1
		for next < len(data) && isJSONSpace(data[next]) {
			next++
		}
		if next == len(data) || data[next] != ':' { // a value, not a key
			var s string
			if err := json.Unmarshal(token, &s); err == nil {
				if redacted := r.redact(s); redacted != s {
					encoded, _ := json.Marshal(redacted)
					out = append(out, data[last:i]...)
					out = append(out, encoded...)
					last = end + 1
				}
			}
		}
		i = end
	}
	if out == nil {
		return data
	}
	return append(out, data[last:]...)
}

func isJSONSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
