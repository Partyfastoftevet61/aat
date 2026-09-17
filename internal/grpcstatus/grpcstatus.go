// Package grpcstatus maps gRPC status codes to the names plans write and to
// the HTTP statuses the engine compares against.
//
// It is a foundation package and deliberately depends on nothing: the sixteen
// codes and their canonical HTTP equivalents are stable, so the table is
// copied rather than pulled in from grpc-go or grpc-gateway.
//
// The mapping is what lets one engine serve both protocols. Every comparison
// the engine makes against a status — whether a step succeeded, which retry
// category an error falls in, whether an expected failure matched — is written
// in HTTP terms, and the mapping keeps every one of them correct: OK becomes
// 200, and every other code becomes something at or above 400.
package grpcstatus

import "strings"

// The gRPC status codes, as google.rpc.Code numbers them.
const (
	OK                 uint32 = 0
	Canceled           uint32 = 1
	Unknown            uint32 = 2
	InvalidArgument    uint32 = 3
	DeadlineExceeded   uint32 = 4
	NotFound           uint32 = 5
	AlreadyExists      uint32 = 6
	PermissionDenied   uint32 = 7
	ResourceExhausted  uint32 = 8
	FailedPrecondition uint32 = 9
	Aborted            uint32 = 10
	OutOfRange         uint32 = 11
	Unimplemented      uint32 = 12
	Internal           uint32 = 13
	Unavailable        uint32 = 14
	DataLoss           uint32 = 15
	Unauthenticated    uint32 = 16
)

// MaxCode is the highest code google.rpc.Code defines.
const MaxCode = Unauthenticated

type entry struct {
	name string
	http int
}

// table holds each code's canonical name and its HTTP equivalent. The HTTP
// statuses are the ones grpc-gateway maps to, so a gRPC step classifies the
// way the equivalent HTTP step would: UNAUTHENTICATED and PERMISSION_DENIED
// are auth failures, UNAVAILABLE and RESOURCE_EXHAUSTED and DEADLINE_EXCEEDED
// are transient, and so on.
var table = [...]entry{
	OK:                 {"OK", 200},
	Canceled:           {"CANCELLED", 499},
	Unknown:            {"UNKNOWN", 500},
	InvalidArgument:    {"INVALID_ARGUMENT", 400},
	DeadlineExceeded:   {"DEADLINE_EXCEEDED", 504},
	NotFound:           {"NOT_FOUND", 404},
	AlreadyExists:      {"ALREADY_EXISTS", 409},
	PermissionDenied:   {"PERMISSION_DENIED", 403},
	ResourceExhausted:  {"RESOURCE_EXHAUSTED", 429},
	FailedPrecondition: {"FAILED_PRECONDITION", 400},
	Aborted:            {"ABORTED", 409},
	OutOfRange:         {"OUT_OF_RANGE", 400},
	Unimplemented:      {"UNIMPLEMENTED", 501},
	Internal:           {"INTERNAL", 500},
	Unavailable:        {"UNAVAILABLE", 503},
	DataLoss:           {"DATA_LOSS", 500},
	Unauthenticated:    {"UNAUTHENTICATED", 401},
}

// Name returns the code's canonical name, or "" when no code has that number.
func Name(code uint32) string {
	if code > MaxCode {
		return ""
	}
	return table[code].name
}

// HTTPStatus returns the HTTP status a code maps to. An unknown code maps to
// 500, as UNKNOWN does, so it still reads as a failure.
func HTTPStatus(code uint32) int {
	if code > MaxCode {
		return 500
	}
	return table[code].http
}

// CodeByName resolves a code written by name. It accepts the canonical
// SCREAMING_SNAKE_CASE spelling and the forms people reach for instead: any
// case, hyphens or nothing in place of underscores, and Go's one-L "Canceled".
func CodeByName(name string) (uint32, bool) {
	normalized := foldName(name)
	if normalized == "CANCELED" {
		return Canceled, true
	}
	for code, e := range table {
		if foldName(e.name) == normalized {
			return uint32(code), true //nolint:gosec // a table index, at most 16
		}
	}
	return 0, false
}

// foldName reduces a name to what is left when case and separators are
// ignored, so "NOT_FOUND", "NotFound", and "not-found" all compare equal.
func foldName(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "")
	return strings.ReplaceAll(s, "_", "")
}

// IsName reports whether a value written in a plan names a gRPC code rather
// than being some other string.
func IsName(v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	_, found := CodeByName(s)
	return found
}

// Names returns every code's canonical name, lowest code first. Errors list
// them when a plan names one that does not exist.
func Names() []string {
	names := make([]string, 0, len(table))
	for _, e := range table {
		names = append(names, e.name)
	}
	return names
}

// FromHTTPStatus maps an HTTP status to the gRPC code an equivalent service
// would return. It is the inverse of HTTPStatus where one exists, and is used
// by a gateway or a façade that fronts an HTTP API with a gRPC one.
//
// The mapping is lossy in the direction HTTPStatus is not: several codes share
// one HTTP status, so 400 comes back as INVALID_ARGUMENT, the most common of
// them, and 409 as ABORTED.
func FromHTTPStatus(status int) uint32 {
	switch status {
	case 200, 201, 202, 204:
		return OK
	case 400:
		return InvalidArgument
	case 401:
		return Unauthenticated
	case 403:
		return PermissionDenied
	case 404:
		return NotFound
	case 409:
		return Aborted
	case 429:
		return ResourceExhausted
	case 499:
		return Canceled
	case 501:
		return Unimplemented
	case 503:
		return Unavailable
	case 504:
		return DeadlineExceeded
	}
	switch {
	case status >= 200 && status < 300:
		return OK
	case status == 412 || status == 422:
		// A request that is well formed but cannot be applied in the
		// resource's current state.
		return FailedPrecondition
	case status >= 400 && status < 500:
		return InvalidArgument
	default:
		return Internal
	}
}
