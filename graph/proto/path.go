package proto

import (
	"fmt"

	"github.com/gburgyan/aat/internal/gjsonpath"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// cursorKind is what a path walk is standing on.
type cursorKind int

const (
	atMessage cursorKind = iota // a message: the next segment names a field
	atList                      // a repeated field: the next segment picks elements
	atMap                       // a map field: the next segment is a key, any key
	atAny                       // a google.protobuf.Any: "@type", then its message's fields
	atLeaf                      // a scalar, an enum, or a well-known type that encodes as one
	atOpen                      // somewhere the descriptors don't describe: nothing below is checked
)

// cursor is a position in a message, reached by walking a path.
type cursor struct {
	kind  cursorKind
	msg   protoreflect.MessageDescriptor // atMessage
	field protoreflect.FieldDescriptor   // the field the walk arrived through, if any
	// name is what the path called this position — a field's JSON name, or
	// a map key — for problem messages.
	name string
	// leaf says what a leaf is, when it is a well-known type rather than a
	// scalar, such as "a google.protobuf.Timestamp, encoded as an RFC
	// 3339 string".
	leaf string
}

// walkResult is where a path walk ended.
type walkResult struct {
	// problem says why the path reads nothing, or is "" when it resolves.
	problem string
	// protoNames lists the segments that named a field by its proto name
	// where its JSON name differs. A request accepts either spelling; a
	// response is encoded with JSON names, so there such a path reads nothing.
	protoNames []protoNameUse
}

// protoNameUse is a path segment that spelled a field by its proto name.
type protoNameUse struct {
	segment int
	field   protoreflect.FieldDescriptor
}

// walkPath follows a GJSON path through msg the way extraction follows it
// through the JSON the codec makes of msg: a key names a field, an index, "#",
// or a query picks list elements, a map field takes any key, and a form it
// can't follow (a modifier, a pipe, a wildcard) ends the walk without a
// problem. Well-known types are walked by their JSON form, not their fields.
func walkPath(msg protoreflect.MessageDescriptor, path string) walkResult {
	cur := cursor{kind: atMessage, msg: msg}
	var res walkResult
	for i, seg := range gjsonpath.Split(path) {
		if seg.Kind == gjsonpath.Opaque || cur.kind == atOpen {
			return res
		}
		switch cur.kind {
		case atMessage:
			switch {
			case seg.Kind == gjsonpath.Wildcard:
				return res
			case seg.Kind == gjsonpath.Key && !seg.IsIndex():
				fd := fieldByAnyName(cur.msg, seg.Key)
				if fd == nil {
					res.problem = fmt.Sprintf("%s does not declare %q%s", cur.msg.FullName(), seg.Key, suggestField(cur.msg, seg.Key))
					return res
				}
				if seg.Key == string(fd.Name()) && fd.JSONName() != seg.Key {
					res.protoNames = append(res.protoNames, protoNameUse{segment: i, field: fd})
				}
				cur = enterField(fd)
			default:
				res.problem = fmt.Sprintf("%s is a message, not a list, so it has no element %q", describe(cur), seg.Raw)
				return res
			}
		case atList:
			switch {
			case seg.IsIndex(), seg.Kind == gjsonpath.Query, seg.Kind == gjsonpath.All:
				// "#" last is the count, and a count is a leaf; otherwise each
				// of these picks elements, and the walk goes on in one.
				cur = enterElement(cur.field, cur.name)
			default:
				res.problem = fmt.Sprintf("%s is a list, so an index, # or a query comes next, not %q", describe(cur), seg.Raw)
				return res
			}
		case atMap:
			// A map encodes as a JSON object keyed by the map's keys, so any
			// key reads a value; gjson reads "#" and digits on an object as
			// keys too. A query over an object's values is past what the
			// descriptors say.
			if seg.Kind == gjsonpath.Query {
				return res
			}
			cur = enterElement(cur.field.MapValue(), seg.Key)
		case atAny:
			// An Any encodes as its message's fields plus "@type". The
			// message's type is only known at run time, so beyond "@type"
			// nothing can be checked.
			if seg.Kind == gjsonpath.Key && seg.Key == "@type" {
				cur = cursor{kind: atLeaf, name: "@type", leaf: "the type URL, a string"}
				continue
			}
			return res
		case atLeaf:
			res.problem = fmt.Sprintf("%s is %s, which has nothing below it", describe(cur), leafName(cur))
			return res
		}
	}
	return res
}

// enterField is the cursor after a segment names fd.
func enterField(fd protoreflect.FieldDescriptor) cursor {
	switch {
	case fd.IsMap():
		return cursor{kind: atMap, field: fd, name: fd.JSONName()}
	case fd.IsList():
		return cursor{kind: atList, field: fd, name: fd.JSONName()}
	}
	return enterElement(fd, fd.JSONName())
}

// enterElement is the cursor on one value of fd, which the path calls name: an
// element when fd is repeated, a map's value, or the value itself.
func enterElement(fd protoreflect.FieldDescriptor, name string) cursor {
	if fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind {
		return cursor{kind: atLeaf, field: fd, name: name}
	}
	md := fd.Message()
	if kind, leaf, ok := wellKnownJSON(md); ok {
		return cursor{kind: kind, msg: md, field: fd, name: name, leaf: leaf}
	}
	return cursor{kind: atMessage, msg: md, field: fd, name: name}
}

// wellKnownJSON says how the JSON codec encodes a google.protobuf well-known
// type that doesn't encode as its fields: Struct, Value, and ListValue as any
// JSON at all, Any as its message plus "@type", and the rest as a single
// string or value. ok is false for any other message, a fork of one of these
// included, which the codec encodes as an ordinary message.
func wellKnownJSON(md protoreflect.MessageDescriptor) (kind cursorKind, leaf string, ok bool) {
	if md.ParentFile() == nil || md.ParentFile().Package() != "google.protobuf" {
		return 0, "", false
	}
	full := string(md.FullName())
	switch md.Name() {
	case "Struct", "Value", "ListValue":
		return atOpen, "", true
	case "Any":
		return atAny, "", true
	case "Timestamp":
		return atLeaf, "a " + full + ", encoded as an RFC 3339 string", true
	case "Duration":
		return atLeaf, "a " + full + `, encoded as a string such as "3s"`, true
	case "FieldMask":
		return atLeaf, "a " + full + ", encoded as a comma-separated string", true
	case "DoubleValue", "FloatValue", "Int64Value", "UInt64Value", "Int32Value", "UInt32Value", "BoolValue", "StringValue", "BytesValue":
		return atLeaf, "a " + full + ", encoded as its bare value", true
	}
	return 0, "", false
}

// describe names what the cursor stands on, for a problem message.
func describe(c cursor) string {
	if c.name != "" {
		return fmt.Sprintf("%q", c.name)
	}
	return string(c.msg.FullName())
}

// leafName is what a leaf is, with its article, for a problem message: a
// scalar field's type, or what a well-known type encodes as.
func leafName(c cursor) string {
	if c.leaf != "" {
		return c.leaf
	}
	switch c.field.Kind() {
	case protoreflect.EnumKind:
		return "an enum"
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		return "an " + c.field.Kind().String()
	}
	return "a " + c.field.Kind().String()
}
