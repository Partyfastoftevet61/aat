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
	atLeaf                      // a scalar or enum: nothing is below it
	atOpen                      // somewhere the descriptors don't describe: nothing below is checked
)

// cursor is a position in a message, reached by walking a path.
type cursor struct {
	kind  cursorKind
	msg   protoreflect.MessageDescriptor // atMessage
	field protoreflect.FieldDescriptor   // the field the walk arrived through, if any
}

// walkResult is where a path walk ended.
type walkResult struct {
	// problem says why the path reads nothing, or is "" when it resolves.
	problem string
	// last is the field the final segment named, when it named one, and
	// lastKey is how the path spelled it.
	last    protoreflect.FieldDescriptor
	lastKey string
}

// walkPath follows a GJSON path through msg the way extraction follows it
// through the JSON the codec makes of msg: a key names a field, an index, "#",
// or a query picks list elements, and a form it can't follow (a modifier, a
// pipe, a wildcard) ends the walk without a problem.
func walkPath(msg protoreflect.MessageDescriptor, path string) walkResult {
	cur := cursor{kind: atMessage, msg: msg}
	var last protoreflect.FieldDescriptor
	var lastKey string
	for _, seg := range gjsonpath.Split(path) {
		last, lastKey = nil, ""
		if seg.Kind == gjsonpath.Opaque || cur.kind == atOpen {
			return walkResult{}
		}
		switch cur.kind {
		case atMessage:
			switch {
			case seg.Kind == gjsonpath.Wildcard:
				return walkResult{}
			case seg.Kind == gjsonpath.Key && !seg.IsIndex():
				fd := fieldByAnyName(cur.msg, seg.Key)
				if fd == nil {
					return walkResult{problem: fmt.Sprintf("%s does not declare %q%s", cur.msg.FullName(), seg.Key, suggestField(cur.msg, seg.Key))}
				}
				cur, last, lastKey = enterField(fd), fd, seg.Key
			default:
				return walkResult{problem: fmt.Sprintf("%s is a message, not a list, so it has no element %q", describe(cur), seg.Raw)}
			}
		case atList:
			switch {
			case seg.IsIndex(), seg.Kind == gjsonpath.Query, seg.Kind == gjsonpath.All:
				// "#" last is the count, and a count is a leaf; otherwise each
				// of these picks elements, and the walk goes on in one.
				cur = enterElement(cur.field)
			default:
				return walkResult{problem: fmt.Sprintf("%s is a list, so an index, # or a query comes next, not %q", describe(cur), seg.Raw)}
			}
		case atLeaf:
			return walkResult{problem: fmt.Sprintf("%s is %s, which has nothing below it", describe(cur), kindName(cur.field))}
		}
	}
	return walkResult{last: last, lastKey: lastKey}
}

// enterField is the cursor after a segment names fd.
func enterField(fd protoreflect.FieldDescriptor) cursor {
	if fd.IsList() {
		return cursor{kind: atList, field: fd}
	}
	return enterElement(fd)
}

// enterElement is the cursor on one value of fd: an element when fd is
// repeated, the value itself when it is not.
func enterElement(fd protoreflect.FieldDescriptor) cursor {
	if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
		return cursor{kind: atMessage, msg: fd.Message(), field: fd}
	}
	return cursor{kind: atLeaf, field: fd}
}

// describe names what the cursor stands on, for a problem message.
func describe(c cursor) string {
	if c.field != nil {
		return fmt.Sprintf("%q", c.field.JSONName())
	}
	return string(c.msg.FullName())
}

// kindName is a scalar field's type, with its article, for a problem message.
func kindName(fd protoreflect.FieldDescriptor) string {
	switch fd.Kind() {
	case protoreflect.EnumKind:
		return "an enum"
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		return "an " + fd.Kind().String()
	}
	return "a " + fd.Kind().String()
}
