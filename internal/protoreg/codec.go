package protoreg

import (
	"bytes"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// The protojson settings every message crosses, chosen once so a project reads
// the same way everywhere:
//
//   - Names are the canonical proto3 JSON ones, lowerCamelCase, so a graph that
//     mixes gRPC with a transcoded HTTP surface has one vocabulary. Requests
//     accept either spelling, because protojson.Unmarshal does; only extract
//     paths have to match, and graph/proto reports the ones that do not.
//   - EmitDefaultValues, not EmitUnpopulated: a proto3 scalar has no presence,
//     so without it a field that is legitimately 0 or "" is absent from the
//     JSON and its extract rule fails. EmitUnpopulated goes further and writes
//     null for unset message fields, which would make fieldExists true for
//     something that is not there.
//   - Enums by name, which survive renumbering and read better in assertions.
//   - Unknown request fields are an error, as unknown keys are in project YAML.
var (
	marshalJSON = protojson.MarshalOptions{
		UseProtoNames:     false,
		UseEnumNumbers:    false,
		EmitDefaultValues: true,
		AllowPartial:      true,
	}
	unmarshalJSON = protojson.UnmarshalOptions{
		DiscardUnknown: false,
		AllowPartial:   true,
	}
)

// MessageToJSON encodes a message as JSON.
//
// protojson deliberately varies its whitespace so output cannot be compared
// byte for byte, which would make every archive diff show phantom changes.
// Compacting settles it: the same message always gives the same bytes.
func (r *Registry) MessageToJSON(msg proto.Message) ([]byte, error) {
	opts := marshalJSON
	opts.Resolver = r.Types
	raw, err := opts.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encoding %s as JSON: %w", msg.ProtoReflect().Descriptor().FullName(), err)
	}
	var out bytes.Buffer
	if err := json.Compact(&out, raw); err != nil {
		return nil, fmt.Errorf("encoding %s as JSON: %w", msg.ProtoReflect().Descriptor().FullName(), err)
	}
	return out.Bytes(), nil
}

// JSONToMessage builds a message of the given type from JSON. A field the
// message does not declare is an error, naming the field.
func (r *Registry) JSONToMessage(md protoreflect.MessageDescriptor, data []byte) (*dynamicpb.Message, error) {
	msg := dynamicpb.NewMessage(md)
	opts := unmarshalJSON
	opts.Resolver = r.Types
	if err := opts.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("building %s: %w", md.FullName(), err)
	}
	return msg, nil
}
