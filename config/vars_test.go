package config

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fillStrings sets every string reachable from v to value, allocating nil
// pointers and giving every map and slice one entry, so a test can check that
// a walk over the type reaches all of them.
func fillStrings(v reflect.Value, value string) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(value)
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		fillStrings(v.Elem(), value)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				fillStrings(v.Field(i), value)
			}
		}
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fillStrings(v.Index(0), value)
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		key := reflect.New(v.Type().Key()).Elem()
		if key.Kind() == reflect.String {
			key.SetString("key")
		}
		elem := reflect.New(v.Type().Elem()).Elem()
		if elem.Kind() == reflect.Interface {
			elem.Set(reflect.ValueOf(value))
		} else {
			fillStrings(elem, value)
		}
		v.SetMapIndex(key, elem)
	}
}

// TestSubstituteVars_ReachesEveryStringField guards against the substitution
// drifting behind EnvironmentPartial again: every string field, including ones
// added later, must have its placeholders replaced.
func TestSubstituteVars_ReachesEveryStringField(t *testing.T) {
	var p EnvironmentPartial
	fillStrings(reflect.ValueOf(&p).Elem(), "${v}")
	p.Vars = map[string]string{"v": "resolved"}

	require.NoError(t, substituteVars(&p))

	data, err := json.Marshal(p)
	require.NoError(t, err)
	assert.NotContains(t, string(data), `${v}`, "a string field kept its placeholder: %s", data)
	assert.Contains(t, string(data), "resolved")
	assert.Equal(t, map[string]string{"v": "resolved"}, p.Vars, "vars themselves are not rewritten")
}

func TestSubstituteVars_FieldsTheOldListSkipped(t *testing.T) {
	p := EnvironmentPartial{
		Vars: map[string]string{"host": "pay.local", "hdr": "X-Pay-Key", "key": "secret", "grant": "client_credentials"},
		Auth: &AuthConfig{
			Type:        "oauth2",
			GrantType:   "${grant}",
			ExtraParams: map[string]string{"audience": "https://${host}"},
		},
		LLM: &LLMConfig{APIKey: SecretRef{Source: "env", Var: "KEY_FOR_${host}"}},
		Overrides: []HostOverride{{
			Match: "payment*",
			Auth: &AuthConfig{
				Type:        "apikey",
				HeaderName:  "${hdr}",
				Credentials: map[string]SecretRef{"key": {Source: "literal", Value: "${key}"}},
			},
			Values:        map[string]any{"note": "via ${host}", "nested": []any{"${host}", 3}},
			ExpectFailure: &OverrideExpectFailure{Status: HTTPStatuses([]int{402}), Description: "declined on ${host}"},
		}},
		Settings: &RuntimeSettings{OASValidation: "${grant}"},
	}

	require.NoError(t, substituteVars(&p))

	assert.Equal(t, "client_credentials", p.Auth.GrantType)
	assert.Equal(t, "https://pay.local", p.Auth.ExtraParams["audience"])
	assert.Equal(t, "KEY_FOR_pay.local", p.LLM.APIKey.Var)
	ov := p.Overrides[0]
	assert.Equal(t, "X-Pay-Key", ov.Auth.HeaderName)
	assert.Equal(t, "secret", ov.Auth.Credentials["key"].Value)
	assert.Equal(t, "via pay.local", ov.Values["note"])
	assert.Equal(t, []any{"pay.local", 3}, ov.Values["nested"])
	assert.Equal(t, "declined on pay.local", ov.ExpectFailure.Description)
	assert.Equal(t, "client_credentials", p.Settings.OASValidation)
}

func TestSubstituteVars_UnresolvedInOverrideCredential(t *testing.T) {
	p := EnvironmentPartial{
		Overrides: []HostOverride{{
			Match: "payment*",
			Auth: &AuthConfig{
				Type:        "apikey",
				HeaderName:  "X-Key",
				Credentials: map[string]SecretRef{"key": {Source: "literal", Value: "${payKey}"}},
			},
		}},
	}

	err := substituteVars(&p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "${payKey}")
}

func TestParseVars(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    map[string]string
		wantErr string
	}{
		{name: "none", in: nil, want: nil},
		{name: "simple", in: []string{"apiHost=localhost:9000"}, want: map[string]string{"apiHost": "localhost:9000"}},
		{name: "value with equals and commas", in: []string{"q=a=b,c"}, want: map[string]string{"q": "a=b,c"}},
		{name: "empty value", in: []string{"region="}, want: map[string]string{"region": ""}},
		{name: "last wins", in: []string{"a=1", "a=2"}, want: map[string]string{"a": "2"}},
		{name: "missing equals", in: []string{"apiHost"}, wantErr: "expected KEY=VALUE"},
		{name: "invalid name", in: []string{"api-host=x"}, wantErr: "not a valid variable name"},
		{name: "empty name", in: []string{"=x"}, wantErr: "not a valid variable name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVars(tt.in)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

const varsEnvYAML = `
environments:
  _base:
    vars:
      apiHost: "localhost:8765"
    apiBaseUrl: http://${apiHost}/${region}/v1
    auth:
      type: none
  us:
    extends: _base
    vars:
      region: us
`

func TestLoadNamedEnvironmentWithVars(t *testing.T) {
	path := writeTempYAML(t, varsEnvYAML)

	tests := []struct {
		name    string
		vars    map[string]string
		wantURL string
		wantErr string
	}{
		{name: "file vars only", wantURL: "http://localhost:8765/us/v1"},
		{name: "external var beats a declared one", vars: map[string]string{"apiHost": "127.0.0.1:9999"}, wantURL: "http://127.0.0.1:9999/us/v1"},
		{name: "external var beats an inherited one", vars: map[string]string{"region": "eu"}, wantURL: "http://localhost:8765/eu/v1"},
		{name: "unknown key", vars: map[string]string{"apihost": "x"}, wantErr: "unknown var(s) apihost: not declared or referenced"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := LoadNamedEnvironmentWithVars(path, "us", tt.vars)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, env.APIBaseURL)
		})
	}
}

func TestLoadNamedEnvironmentWithVars_ReferencedButUndeclared(t *testing.T) {
	path := writeTempYAML(t, `
environments:
  dev:
    apiBaseUrl: http://${host}/v1
    auth:
      type: none
`)

	_, err := LoadNamedEnvironment(path, "dev")
	require.Error(t, err, "without the var the placeholder is unresolved")
	assert.Contains(t, err.Error(), "${host}")

	env, err := LoadNamedEnvironmentWithVars(path, "dev", map[string]string{"host": "api.local"})
	require.NoError(t, err)
	assert.Equal(t, "http://api.local/v1", env.APIBaseURL)
}

func TestLoadNamedEnvironmentWithVars_SingleEnvironmentFile(t *testing.T) {
	path := writeTempYAML(t, `
environment: test
apiBaseUrl: https://api.example.com
auth:
  type: none
`)

	_, err := LoadNamedEnvironmentWithVars(path, "", map[string]string{"host": "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--var applies only to multi-environment files")
}
