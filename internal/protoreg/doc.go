// Package protoreg loads protobuf descriptors and converts messages between
// their wire form and JSON.
//
// It is a foundation package: it imports the protobuf runtime and the standard
// library, and no other aat package. Both adapter, which builds and sends gRPC
// requests, and graph/proto, which validates a graph against the descriptors,
// depend on it, so it cannot sit in either of their tiers.
//
// Descriptors come from a FileDescriptorSet, the artifact
// `protoc --descriptor_set_out` and `buf build -o` produce. AAT reads a
// resolved artifact rather than parsing .proto source, as it does for OpenAPI.
//
// Messages cross into the rest of AAT as JSON. Everything downstream — the
// extract rules, predicates, assertions, archives, and the web UI — reads a
// response body as JSON, so a gRPC message that arrives as JSON needs none of
// them to know protobuf exists.
package protoreg
