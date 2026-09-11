// Package httpstatus interprets the expected-status values that plans and
// assertions carry: an exact HTTP status code such as 201, or a status class
// such as "2xx". It is a foundation package with no aat imports, so both plan
// validation and assertion evaluation share one reading of these values.
package httpstatus

import (
	"encoding/json"
	"math"
	"strings"
)

// Class reports whether v is a status class such as "2xx" (any case) and
// returns its leading digit.
func Class(v any) (int, bool) {
	s, ok := v.(string)
	if !ok || len(s) != 3 || s[0] < '1' || s[0] > '5' || !strings.EqualFold(s[1:], "xx") {
		return 0, false
	}
	return int(s[0] - '0'), true
}

// Code reports whether v is an exact integral status code and returns it. It
// accepts the numeric forms YAML and JSON decoding produce: int, int64, an
// integral float64, and json.Number.
func Code(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n == math.Trunc(n) {
			return int(n), true
		}
		return 0, false
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

// ContradictsFailure reports whether a status assertion expecting v can never
// hold on a step that expects a failure: an exact code below 400, or a 1xx-3xx
// class. Values that are neither form do not contradict.
func ContradictsFailure(v any) bool {
	if code, ok := Code(v); ok {
		return code < 400
	}
	if class, ok := Class(v); ok {
		return class < 4
	}
	return false
}
