package archive

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    map[string]string
	}{
		{
			name:    "nil headers",
			headers: nil,
			want:    nil,
		},
		{
			name:    "empty headers",
			headers: map[string]string{},
			want:    map[string]string{},
		},
		{
			name: "no sensitive headers",
			headers: map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
			},
			want: map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
			},
		},
		{
			name: "authorization redacted",
			headers: map[string]string{
				"Authorization": "Bearer secret-token",
				"Content-Type":  "application/json",
			},
			want: map[string]string{
				"Authorization": "[REDACTED]",
				"Content-Type":  "application/json",
			},
		},
		{
			name: "case insensitive matching",
			headers: map[string]string{
				"AUTHORIZATION": "Bearer secret",
				"X-Api-Key":     "key-123",
				"x-auth-token":  "tok-456",
			},
			want: map[string]string{
				"AUTHORIZATION": "[REDACTED]",
				"X-Api-Key":     "[REDACTED]",
				"x-auth-token":  "[REDACTED]",
			},
		},
		{
			name: "cookie and set-cookie redacted",
			headers: map[string]string{
				"Cookie":     "session=abc",
				"Set-Cookie": "session=abc; Path=/",
			},
			want: map[string]string{
				"Cookie":     "[REDACTED]",
				"Set-Cookie": "[REDACTED]",
			},
		},
		{
			name: "proxy-authorization redacted",
			headers: map[string]string{
				"Proxy-Authorization": "Basic abc123",
			},
			want: map[string]string{
				"Proxy-Authorization": "[REDACTED]",
			},
		},
		{
			name: "mixed sensitive and non-sensitive",
			headers: map[string]string{
				"Authorization": "Bearer token",
				"Content-Type":  "application/json",
				"X-Request-Id":  "req-123",
				"X-API-KEY":     "secret",
			},
			want: map[string]string{
				"Authorization": "[REDACTED]",
				"Content-Type":  "application/json",
				"X-Request-Id":  "req-123",
				"X-API-KEY":     "[REDACTED]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactHeaders(tt.headers)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRedactHeaders_DoesNotMutateInput(t *testing.T) {
	original := map[string]string{
		"Authorization": "Bearer secret",
		"Content-Type":  "application/json",
	}
	RedactHeaders(original)
	assert.Equal(t, "Bearer secret", original["Authorization"])
}

func TestRedactor_Redact(t *testing.T) {
	secrets := map[string]bool{
		"demo":              true, // short: whole strings only
		"k3y-value-long":    true,
		"abcdefgh12":        true,
		"xxabcdefgh":        true, // overlaps the one above
		"p@ss word/with?&=": true,
	}
	r := newRedactor(secrets)
	tests := []struct {
		name, in, want string
	}{
		{name: "short secret as a whole string", in: "demo", want: "[REDACTED]"},
		{name: "short secret inside data survives", in: "demo@example.com", want: "demo@example.com"},
		{name: "long secret as a whole string", in: "k3y-value-long", want: "[REDACTED]"},
		{name: "long secret inside a string", in: "Bearer k3y-value-long; x", want: "Bearer [REDACTED]; x"},
		{name: "overlapping secrets are covered together", in: "--xxabcdefgh12--", want: "--[REDACTED]--"},
		{name: "query-escaped form", in: "https://api.test/x?pw=p%40ss+word%2Fwith%3F%26%3D&y=1", want: "https://api.test/x?pw=[REDACTED]&y=1"},
		{name: "path-escaped form", in: "/users/p@ss%20word%2Fwith%3F&=/items", want: "/users/[REDACTED]/items"},
		{name: "no secret", in: "nothing to see", want: "nothing to see"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, r.redact(tt.in))
		})
	}
}

func TestRedactor_RedactJSON(t *testing.T) {
	r := newRedactor(map[string]bool{"token-12345": true, "12345678": true, "demo": true})
	tests := []struct {
		name, in, want string
	}{
		{
			name: "string values change, keys and numbers do not",
			in:   `{"token-12345": 12345678, "auth": "Bearer token-12345", "user": "demo", "email": "demo@example.com"}`,
			want: `{"token-12345": 12345678, "auth": "Bearer [REDACTED]", "user": "[REDACTED]", "email": "demo@example.com"}`,
		},
		{
			name: "an escaped token is decoded before matching",
			in:   `{"a":"tok\u0065n-12345","b":["x","\"quoted\" token-12345"]}`,
			want: `{"a":"[REDACTED]","b":["x","\"quoted\" [REDACTED]"]}`,
		},
		{
			name: "a body that was not JSON is one string",
			in:   `"grant_type=password&secret=token-12345"`,
			want: `"grant_type=password\u0026secret=[REDACTED]"`,
		},
		{
			name: "nothing matched leaves the bytes alone",
			in:   "{\n  \"html\": \"<b>&amp;</b>\",\n  \"n\": 9007199254740993\n}",
			want: "{\n  \"html\": \"<b>&amp;</b>\",\n  \"n\": 9007199254740993\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.redactJSON([]byte(tt.in))
			assert.Equal(t, tt.want, string(got))
			assert.True(t, json.Valid(got))
		})
	}
}

// TestRedact_Archive checks that a secret is redacted from every part of an
// archive, that the caller's archive is not modified, and that identifiers
// tagged redact:"-" and numbers are kept.
func TestRedact_Archive(t *testing.T) {
	secret := "pay-demo-key-123"
	a := &Archive{
		Metadata: ArchiveMetadata{RunID: "run-1", Environment: "us"},
		Steps: []StepRecord{{
			StepID: "pay",
			Node:   "paymentCharge",
			Inputs: map[string]any{"apiKey": secret, "amount": json.Number("10340")},
			Request: &RequestRecord{
				Method:  "POST",
				URL:     "https://pay.test/charges?key=" + secret,
				Headers: map[string]string{"X-Shop-Token": secret, "Accept": "application/json"},
				Body:    json.RawMessage(`{"key":"` + secret + `","amount":10340}`),
			},
			Response: &ResponseRecord{Status: 201, Body: json.RawMessage(`{"echo":"` + secret + `"}`)},
			Outputs:  map[string]any{"receipt": "paid with " + secret, "total": 10340},
			Error:    "step failed sending " + secret,
			Validation: &ValidationRecord{Results: []AssertionRecord{
				{Type: "fieldEquals", Message: `field "key": expected x, got ` + secret},
			}},
		}},
		Result: ArchiveResult{Outcome: "passed", DurationMs: 2900},
	}

	got, err := Redact(a, map[string]bool{secret: true})
	require.NoError(t, err)

	data, err := json.Marshal(got)
	require.NoError(t, err)
	assert.NotContains(t, string(data), secret)
	assert.Contains(t, string(data), `"amount":10340`)
	assert.Equal(t, "pay", got.Steps[0].StepID)
	assert.Equal(t, "paymentCharge", got.Steps[0].Node)
	assert.Equal(t, int64(2900), got.Result.DurationMs)
	assert.Equal(t, "application/json", got.Steps[0].Request.Headers["Accept"])
	assert.Equal(t, "https://pay.test/charges?key=[REDACTED]", got.Steps[0].Request.URL)

	assert.Equal(t, secret, a.Steps[0].Inputs["apiKey"], "the caller's archive is not modified")
	assert.Contains(t, string(a.Steps[0].Request.Body), secret)
}

func TestRedact_NoSecretsReturnsInput(t *testing.T) {
	a := &Archive{Result: ArchiveResult{Outcome: "passed"}}
	got, err := Redact(a, nil)
	require.NoError(t, err)
	assert.Same(t, a, got)
}

// TestRedact_UnmatchedSecretChangesNothing checks that the round trip Redact
// makes preserves an archive byte for byte when no secret occurs in it,
// including large integers and HTML characters in bodies.
func TestRedact_UnmatchedSecretChangesNothing(t *testing.T) {
	a := &Archive{
		Metadata: ArchiveMetadata{RunID: "run-1", Timestamp: time.Date(2026, 9, 11, 12, 0, 0, 123, time.UTC)},
		Steps: []StepRecord{{
			Node:     "getOrder",
			Inputs:   map[string]any{"big": json.Number("9007199254740993"), "ratio": 0.5, "ok": true, "none": nil},
			Response: &ResponseRecord{Status: 200, Body: json.RawMessage(`{"html":"<b>&</b>","n":9007199254740993}`)},
		}},
	}
	before, err := json.MarshalIndent(a, "", "  ")
	require.NoError(t, err)

	got, err := Redact(a, map[string]bool{"never-appears-anywhere": true})
	require.NoError(t, err)
	after, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
}

// TestRedact_CoversEveryField fills every string, any, and raw JSON field of
// an archive and a batch archive with a secret, and checks that the secret
// survives only in fields tagged redact:"-". A new field that holds request or
// response data is covered without changes to the redaction code.
func TestRedact_CoversEveryField(t *testing.T) {
	const secret = "s3cret-value-for-coverage"
	for _, v := range []any{&Archive{}, &BatchArchive{}} {
		t.Run(reflect.TypeOf(v).Elem().Name(), func(t *testing.T) {
			tagged := fillWithSecret(reflect.ValueOf(v).Elem(), secret, false)
			require.NotZero(t, tagged.untagged, "the fill reached untagged fields")

			var got any
			var err error
			switch x := v.(type) {
			case *Archive:
				got, err = Redact(x, map[string]bool{secret: true})
			case *BatchArchive:
				got, err = Redact(x, map[string]bool{secret: true})
			}
			require.NoError(t, err)
			data, err := json.Marshal(got)
			require.NoError(t, err)
			assert.Equal(t, tagged.tagged, strings.Count(string(data), secret),
				"the secret remains only in the %d fields tagged redact:\"-\"", tagged.tagged)
		})
	}
}

type fillCounts struct{ tagged, untagged int }

// fillWithSecret sets every string, any, and json.RawMessage reachable from v
// to hold secret, allocating pointers, one slice element, and one map entry
// along the way, and counts how many it set under a redact:"-" tag.
func fillWithSecret(v reflect.Value, secret string, underTag bool) fillCounts {
	var c fillCounts
	count := func() {
		if underTag {
			c.tagged++
		} else {
			c.untagged++
		}
	}
	add := func(o fillCounts) { c.tagged += o.tagged; c.untagged += o.untagged }
	switch v.Kind() {
	case reflect.String:
		v.SetString(secret)
		count()
	case reflect.Interface:
		v.Set(reflect.ValueOf(secret))
		count()
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		add(fillWithSecret(v.Elem(), secret, underTag))
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			return c
		}
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if f.IsExported() {
				add(fillWithSecret(v.Field(i), secret, underTag || f.Tag.Get("redact") == "-"))
			}
		}
	case reflect.Slice:
		if v.Type() == reflect.TypeOf(json.RawMessage(nil)) {
			v.SetBytes([]byte(`{"k":"` + secret + `"}`))
			count()
			return c
		}
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		add(fillWithSecret(v.Index(0), secret, underTag))
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		elem := reflect.New(v.Type().Elem()).Elem()
		add(fillWithSecret(elem, secret, underTag))
		key := reflect.New(v.Type().Key()).Elem()
		if key.Kind() == reflect.String {
			key.SetString("key") // map keys are never redacted, so keep them free of the secret
		}
		v.SetMapIndex(key, elem)
	}
	return c
}
