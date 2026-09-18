// Package proto validates graph nodes against protobuf descriptors, as
// graph/oas does against an OpenAPI document.
//
// It implements graph.SpecValidator, so `aat validate` checks a gRPC graph the
// same way it checks an HTTP one, offline and with no server running.
//
// The checks are fewer than the OpenAPI ones, because a gRPC call has less to
// get wrong: there are no query parameters, no header inputs, no form fields,
// and no path template. What a node sends is one message, and what it reads is
// one message, so the checks are that the method exists, that it is unary, and
// that the node's inputs and outputs are fields those two messages declare:
// each input where the template's message places it, each output along its
// extract path.
package proto
