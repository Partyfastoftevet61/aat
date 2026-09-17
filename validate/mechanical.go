package validate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/gburgyan/aat/internal/grpcstatus"
	"github.com/gburgyan/aat/internal/httpstatus"
)

// AssertionType enumerates the kinds of mechanical assertion.
type AssertionType string

const (
	AssertStatus      AssertionType = "status"
	AssertSchema      AssertionType = "schema"
	AssertFieldExists AssertionType = "fieldExists"
	AssertFieldAbsent AssertionType = "fieldAbsent"
	AssertFieldEquals AssertionType = "fieldEquals"
	AssertPredicate   AssertionType = "predicate"
	// AssertRepeat is the result a repeated step records when its repeat.until
	// condition never held or couldn't be evaluated. A plan can't declare it.
	AssertRepeat AssertionType = "repeat"
)

// MechanicalAssertion describes a structured check on a response.
type MechanicalAssertion struct {
	Type   AssertionType
	Expect any
	Ref    string
	Path   string
	Value  any
	Expr   string
	Raw    bool
	// Display is a predicate as its message shows it, with the {{…}}
	// expressions in its literals expanded. The message shows Expr when it is
	// empty.
	Display string
}

// AssertionResult records the outcome of a single assertion.
type AssertionResult struct {
	Type    AssertionType
	Passed  bool
	Skipped bool // true when assertion was not evaluated (e.g., not yet implemented)
	Message string
	Path    string
	Expr    string
	Raw     bool
}

// MechanicalResult aggregates the outcomes of all mechanical assertions for a step.
type MechanicalResult struct {
	Passed  bool
	Results []AssertionResult
}

// PredicateEvalFunc evaluates a predicate expression against a context map.
// It returns true/false or an error if the expression is invalid.
type PredicateEvalFunc func(expr string, context map[string]any) (bool, error)

// SchemaCheckFunc evaluates a schema assertion. Callers provide this to surface
// OAS response validation (already computed elsewhere) as an assertion outcome.
// Return a result with Skipped=true when schema context is unavailable.
type SchemaCheckFunc func(a MechanicalAssertion) AssertionResult

// RunMechanical evaluates all mechanical assertions against a response.
// It returns an aggregate result indicating whether all assertions passed.
// schemaCheck may be nil; when nil, schema assertions return Skipped.
func RunMechanical(status StatusInfo, body []byte, assertions []MechanicalAssertion, predicateEval PredicateEvalFunc, schemaCheck SchemaCheckFunc) *MechanicalResult {
	result := &MechanicalResult{Passed: true}

	for _, a := range assertions {
		var ar AssertionResult
		switch a.Type {
		case AssertStatus:
			ar = checkStatus(status, a)
		case AssertSchema:
			ar = checkSchema(a, schemaCheck)
		case AssertFieldExists:
			ar = checkFieldExists(body, a)
		case AssertFieldAbsent:
			ar = checkFieldAbsent(body, a)
		case AssertFieldEquals:
			ar = checkFieldEquals(body, a)
		case AssertPredicate:
			ar = checkPredicate(body, a, predicateEval)
		default:
			ar = AssertionResult{
				Type:    a.Type,
				Passed:  false,
				Message: fmt.Sprintf("unknown assertion type %q", a.Type),
			}
		}

		ar.Raw = a.Raw
		result.Results = append(result.Results, ar)
		if !ar.Passed {
			result.Passed = false
		}
	}

	return result
}

// StatusInfo is the status a response came back with, as assertions read it.
//
// Code is an HTTP status whatever the protocol: a gRPC response carries the
// one its code maps to, so a class such as "2xx" and an exact code such as 404
// mean the same thing for both. GRPCName is set only for a gRPC response, and
// lets an assertion name the code the server actually sent.
type StatusInfo struct {
	Code     int
	GRPCName string
}

// HTTPStatusInfo is the status of an HTTP response.
func HTTPStatusInfo(code int) StatusInfo { return StatusInfo{Code: code} }

// checkStatus compares the response status against the expected value: an
// exact code such as 201, a class such as "2xx", or — on a gRPC response — a
// status name such as NOT_FOUND.
func checkStatus(status StatusInfo, a MechanicalAssertion) AssertionResult {
	ar := AssertionResult{Type: AssertStatus}

	if a.Expect == nil {
		ar.Passed = false
		ar.Message = "status assertion missing 'expect' value"
		return ar
	}

	// A gRPC status is named before it is numbered: several codes share one
	// HTTP status, so comparing names is the only way to tell them apart.
	if grpcstatus.IsName(a.Expect) {
		expected, _ := a.Expect.(string)
		if status.GRPCName == "" {
			ar.Passed = false
			ar.Message = fmt.Sprintf("expected gRPC status %s, but the step was not a gRPC call (status %d)", expected, status.Code)
			return ar
		}
		// Compare the codes the two names resolve to, so every accepted
		// spelling — NOT_FOUND, not_found, NotFound, not-found — matches.
		expectedCode, _ := grpcstatus.CodeByName(expected)
		actualCode, known := grpcstatus.CodeByName(status.GRPCName)
		ar.Passed = known && expectedCode == actualCode
		if ar.Passed {
			ar.Message = "status is " + status.GRPCName
		} else {
			ar.Message = fmt.Sprintf("expected status %s, got %s", strings.ToUpper(expected), status.GRPCName)
		}
		return ar
	}

	if class, ok := httpstatus.Class(a.Expect); ok {
		ar.Passed = status.Code/100 == class
		if ar.Passed {
			ar.Message = fmt.Sprintf("status code %d is %dxx", status.Code, class)
		} else {
			ar.Message = fmt.Sprintf("expected status %dxx, got %d", class, status.Code)
		}
		return ar
	}

	expected, ok := httpstatus.Code(a.Expect)
	if !ok {
		ar.Passed = false
		ar.Message = fmt.Sprintf("cannot read expect value %v (%T) as a status code, a class such as \"2xx\", or a gRPC status name", a.Expect, a.Expect)
		return ar
	}

	if status.Code == expected {
		ar.Passed = true
		ar.Message = statusIsMessage(status)
	} else {
		ar.Passed = false
		ar.Message = fmt.Sprintf("expected status %d, got %s", expected, statusGotMessage(status))
	}
	return ar
}

// statusIsMessage describes a matched status, naming a gRPC code where there
// is one so the message reads the way the plan's author thinks.
func statusIsMessage(status StatusInfo) string {
	if status.GRPCName != "" {
		return fmt.Sprintf("status is %s (%d)", status.GRPCName, status.Code)
	}
	return fmt.Sprintf("status code is %d", status.Code)
}

func statusGotMessage(status StatusInfo) string {
	if status.GRPCName != "" {
		return fmt.Sprintf("%s (%d)", status.GRPCName, status.Code)
	}
	return fmt.Sprintf("%d", status.Code)
}

// checkSchema delegates to the caller-provided SchemaCheckFunc, which typically
// surfaces OAS response validation results. When no callback is available, the
// assertion is reported as Skipped so archive consumers can distinguish
// "passed" from "not evaluated".
func checkSchema(a MechanicalAssertion, schemaCheck SchemaCheckFunc) AssertionResult {
	if schemaCheck == nil {
		return AssertionResult{
			Type:    AssertSchema,
			Passed:  true,
			Skipped: true,
			Message: "schema validation unavailable (no OAS context)",
		}
	}
	ar := schemaCheck(a)
	ar.Type = AssertSchema
	return ar
}

// checkFieldExists verifies that a field exists and is not null in the response body.
func checkFieldExists(body []byte, a MechanicalAssertion) AssertionResult {
	ar := AssertionResult{Type: AssertFieldExists, Path: a.Path}

	path := NormalizeJSONPath(a.Path)
	if path == "" {
		ar.Passed = false
		ar.Message = "fieldExists assertion requires a non-empty path"
		return ar
	}

	r := gjson.GetBytes(body, path)
	if !r.Exists() || r.Type == gjson.Null {
		ar.Passed = false
		ar.Message = fmt.Sprintf("field %q does not exist or is null", a.Path)
	} else {
		ar.Passed = true
		ar.Message = fmt.Sprintf("field %q exists", a.Path)
	}
	return ar
}

// checkFieldAbsent verifies that a field is missing or null in the response
// body, the counterpart of checkFieldExists, such as the code an error body
// leaves out.
func checkFieldAbsent(body []byte, a MechanicalAssertion) AssertionResult {
	ar := AssertionResult{Type: AssertFieldAbsent, Path: a.Path}

	path := NormalizeJSONPath(a.Path)
	if path == "" {
		ar.Passed = false
		ar.Message = "fieldAbsent assertion requires a non-empty path"
		return ar
	}

	r := gjson.GetBytes(body, path)
	if !r.Exists() || r.Type == gjson.Null {
		ar.Passed = true
		ar.Message = fmt.Sprintf("field %q is absent", a.Path)
		return ar
	}
	value := r.Raw
	if len(value) > 80 {
		value = value[:77] + "..."
	}
	ar.Passed = false
	ar.Message = fmt.Sprintf("field %q is present: %s", a.Path, value)
	return ar
}

// checkFieldEquals verifies that a field in the response body equals the expected value.
func checkFieldEquals(body []byte, a MechanicalAssertion) AssertionResult {
	ar := AssertionResult{Type: AssertFieldEquals, Path: a.Path}

	path := NormalizeJSONPath(a.Path)
	if path == "" {
		ar.Passed = false
		ar.Message = "fieldEquals assertion requires a non-empty path"
		return ar
	}

	r := gjson.GetBytes(body, path)
	if !r.Exists() {
		ar.Passed = false
		ar.Message = fmt.Sprintf("field %q does not exist", a.Path)
		return ar
	}

	if ValuesEqual(r, a.Value) {
		ar.Passed = true
		ar.Message = fmt.Sprintf("field %q equals %v", a.Path, a.Value)
	} else {
		ar.Passed = false
		ar.Message = fmt.Sprintf("field %q: expected %v, got %v", a.Path, a.Value, r.Value())
	}
	return ar
}

// checkPredicate evaluates a predicate expression against the response body.
func checkPredicate(body []byte, a MechanicalAssertion, predicateEval PredicateEvalFunc) AssertionResult {
	ar := AssertionResult{Type: AssertPredicate, Expr: a.Expr}

	if predicateEval == nil {
		ar.Passed = false
		ar.Message = "predicate evaluator is not available"
		return ar
	}

	if a.Expr == "" {
		ar.Passed = false
		ar.Message = "predicate assertion requires a non-empty 'expr'"
		return ar
	}

	var bodyMap map[string]any
	if err := json.Unmarshal(body, &bodyMap); err != nil {
		ar.Passed = false
		ar.Message = fmt.Sprintf("response body is not a JSON object: %v", err)
		return ar
	}

	result, err := predicateEval(a.Expr, bodyMap)
	if err != nil {
		ar.Passed = false
		ar.Message = fmt.Sprintf("predicate evaluation error: %v", err)
		return ar
	}

	shown := a.Expr
	if a.Display != "" {
		shown = a.Display
	}
	ar.Passed = result
	if result {
		ar.Message = fmt.Sprintf("predicate %q is true", shown)
	} else {
		ar.Message = fmt.Sprintf("predicate %q is false", shown)
	}
	return ar
}

// ValuesEqual compares a gjson.Result with an expected value, handling numeric coercion.
func ValuesEqual(r gjson.Result, expected any) bool {
	switch e := expected.(type) {
	case string:
		return r.Type == gjson.String && r.Str == e
	case bool:
		return r.Type == gjson.True && e || r.Type == gjson.False && !e
	case float64:
		return r.Type == gjson.Number && r.Num == e
	case int:
		return r.Type == gjson.Number && r.Num == float64(e)
	case int64:
		return r.Type == gjson.Number && r.Num == float64(e)
	default:
		// Fallback: compare string representations
		return fmt.Sprintf("%v", r.Value()) == fmt.Sprintf("%v", expected)
	}
}
