package httpstatus

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClass(t *testing.T) {
	tests := []struct {
		name  string
		in    any
		class int
		ok    bool
	}{
		{"lowercase", "2xx", 2, true},
		{"uppercase", "4XX", 4, true},
		{"mixed case", "5xX", 5, true},
		{"one hundreds", "1xx", 1, true},
		{"out of range", "6xx", 0, false},
		{"zero class", "0xx", 0, false},
		{"exact code as string", "200", 0, false},
		{"too long", "2xxx", 0, false},
		{"garbage", "abc", 0, false},
		{"integer", 200, 0, false},
		{"nil", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			class, ok := Class(tt.in)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.class, class)
		})
	}
}

func TestCode(t *testing.T) {
	tests := []struct {
		name string
		in   any
		code int
		ok   bool
	}{
		{"int", 201, 201, true},
		{"int64", int64(404), 404, true},
		{"integral float", float64(409), 409, true},
		{"fractional float", 200.5, 0, false},
		{"json number", json.Number("503"), 503, true},
		{"non-integral json number", json.Number("5.5"), 0, false},
		{"class string", "2xx", 0, false},
		{"numeric string", "200", 0, false},
		{"nil", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, ok := Code(tt.in)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.code, code)
		})
	}
}

func TestContradictsFailure(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want bool
	}{
		{"success code", 200, true},
		{"redirect code", 302, true},
		{"client error code", 409, false},
		{"server error code", float64(503), false},
		{"success class", "2xx", true},
		{"redirect class", "3XX", true},
		{"client error class", "4xx", false},
		{"server error class", "5xx", false},
		{"unrecognized value", "abc", false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ContradictsFailure(tt.in))
		})
	}
}
