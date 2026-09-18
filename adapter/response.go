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
	// Headers are the response headers, or a gRPC call's header metadata.
	Headers http.Header
	// Trailers are a gRPC call's trailing metadata, which is where a server
	// puts what it only knows once the reply is written. It is nil for HTTP.
	// Reads go through HeaderValues, which spans both.
	Trailers http.Header
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

// mergedHeaders returns the response's headers and trailers as one set, for
// the places that read a name without caring which half carried it. A key sent
// in both keeps both values, headers first, so a reader taking the first gets
// the header — the same precedence HeaderValues gives.
func (r *Response) mergedHeaders() http.Header {
	if len(r.Trailers) == 0 {
		return r.Headers
	}
	merged := make(http.Header, len(r.Headers)+len(r.Trailers))
	for _, set := range []http.Header{r.Headers, r.Trailers} {
		for k, values := range set {
			for _, v := range values {
				merged.Add(k, v)
			}
		}
	}
	return merged
}

// HeaderValues returns the values a response carries under a name, matched in
// any case. A gRPC call's trailing metadata is searched after its header
// metadata, so a template reads a value wherever the server chose to put it.
func (r *Response) HeaderValues(name string) []string {
	if values := headerValues(r.Headers, name); len(values) > 0 {
		return values
	}
	return headerValues(r.Trailers, name)
}
