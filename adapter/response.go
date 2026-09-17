package adapter

import (
	"encoding/json"
	"net/http"
)

// Response represents the result of a call an executor made.
//
// StatusCode is an HTTP status whatever the protocol: a gRPC response carries
// the HTTP status its code maps to (see internal/grpcstatus), so that every
// check the engine makes — whether the step succeeded, which retry category
// the failure falls in, whether an expected failure matched — is written once
// and is correct for both. The gRPC code itself stays in GRPC, for the
// assertions and the archives that should name it.
type Response struct {
	StatusCode int
	// Headers are the response headers, or a gRPC call's metadata and
	// trailers merged, trailers last.
	Headers http.Header
	// Body is the response body, always JSON for gRPC: the reply message, or
	// the status envelope when the call failed.
	Body []byte

	// Protocol names how the response arrived. An empty value means HTTP.
	Protocol string
	// GRPC is the native status of a gRPC call, and nil for an HTTP response.
	GRPC *GRPCStatus
}

// GRPCStatus is what a gRPC call reports besides its message: the code, the
// message that came with it, and whatever details the server attached.
type GRPCStatus struct {
	// Code is the gRPC status code, 0 through 16.
	Code uint32
	// Name is the code's canonical name, such as "NOT_FOUND".
	Name string
	// Message is the server's explanation.
	Message string
	// Details are the status details, each already encoded as JSON so that
	// nothing downstream has to know protobuf to carry them.
	Details []json.RawMessage
}

// IsGRPC reports whether the response came from a gRPC call.
func (r *Response) IsGRPC() bool { return r.GRPC != nil }
