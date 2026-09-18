package plan

import (
	"github.com/gburgyan/aat/config"
	"github.com/gburgyan/aat/internal/grpcstatus"
	"github.com/gburgyan/aat/internal/httpstatus"
)

// ExpectedStatus is a status a step is expected to fail with, written as an
// HTTP status code or as a gRPC status name. It is config's type, so an
// override's expectFailure and a plan step's are read by the same code.
type ExpectedStatus = config.ExpectedStatus

// ExpectedStatuses is a list of expected statuses.
type ExpectedStatuses = config.ExpectedStatuses

// HTTPStatus returns an expected status written as an HTTP code.
func HTTPStatus(code int) ExpectedStatus { return config.HTTPStatus(code) }

// HTTPStatuses converts plain codes, for callers that still hold ints.
func HTTPStatuses(codes []int) ExpectedStatuses { return config.HTTPStatuses(codes) }

// ParseExpectedStatus reads a status written as a number or a gRPC name.
func ParseExpectedStatus(v any) (ExpectedStatus, error) { return config.ParseExpectedStatus(v) }

// ContradictsFailure reports whether a status assertion's expected value can
// never hold on a step that expects to fail: a success code, a success class,
// or a gRPC status name that maps to one.
//
// internal/httpstatus answers the first two and imports nothing, so it cannot
// know the names; a name is resolved here, where grpcstatus is already in
// scope. OK contradicts an expectFailure, and CANCELLED does not, because it
// maps to 499.
func ContradictsFailure(v any) bool {
	if httpstatus.ContradictsFailure(v) {
		return true
	}
	if name, ok := v.(string); ok {
		if code, known := grpcstatus.CodeByName(name); known {
			return grpcstatus.HTTPStatus(code) < 400
		}
	}
	return false
}
