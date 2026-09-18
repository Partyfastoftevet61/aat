package gjsonpath

import (
	"strings"

	"github.com/tidwall/gjson"
)

// Kind is what a segment of a GJSON path is.
type Kind uint8

const (
	// Key is an object key, or an array index when it is all digits. Its Key
	// field has the escapes removed.
	Key Kind = iota
	// All is "#": every element of an array when more path follows, the
	// number of elements when it is last.
	All
	// Query is "#(...)", the first element that matches, or "#(...)#", every
	// element that does.
	Query
	// Wildcard is a key with an unescaped * or ?, which matches any number of
	// keys.
	Wildcard
	// Opaque is the rest of the path from a form Split does not model: a
	// modifier such as @reverse, a pipe, a multipath, or a literal. It is
	// always the last segment.
	Opaque
)

// Segment is one component of a GJSON path.
type Segment struct {
	Kind Kind
	// Key is the key with its escapes removed, for Key and Wildcard segments.
	Key string
	// Raw is the segment as the path wrote it. For an Opaque segment that a
	// pipe began, it keeps the leading "|".
	Raw string
}

// IsIndex reports whether the segment is an array index: a Key of digits.
func (s Segment) IsIndex() bool {
	if s.Kind != Key || s.Key == "" {
		return false
	}
	for i := 0; i < len(s.Key); i++ {
		if s.Key[i] < '0' || s.Key[i] > '9' {
			return false
		}
	}
	return true
}

// Split returns the segments of a GJSON path. An empty path has none. A
// leading "$." is not GJSON syntax and is not removed here; callers normalize
// JSONPath-style paths first, as extraction does.
func Split(path string) []Segment {
	var segs []Segment
	opaque := func(raw string) []Segment {
		return append(segs, Segment{Kind: Opaque, Raw: raw})
	}
	// A whole path that starts with a modifier, a literal, or a multipath is
	// not a chain of keys, and neither is a JSON Lines path.
	if startsSubExpression(path, true) || strings.HasPrefix(path, "..") {
		return opaque(path)
	}
	for path != "" {
		var seg Segment
		var rest string
		if strings.HasPrefix(path, "#(") {
			end := queryEnd(path)
			if end < 0 {
				return opaque(path)
			}
			seg, rest = Segment{Kind: Query, Raw: path[:end]}, path[end:]
		} else {
			seg, rest = readKey(path)
		}
		segs = append(segs, seg)
		switch {
		case rest == "":
			return segs
		case rest[0] == '|':
			return opaque(rest)
		case rest[0] == '.':
			rest = rest[1:]
			// After a dot, a modifier or a multipath is piped the rest of the
			// path rather than read as a key.
			if rest == "" || startsSubExpression(rest, false) {
				return opaque(rest)
			}
			path = rest
		default:
			// A query followed by something other than a separator.
			return opaque(rest)
		}
	}
	return segs
}

// Join writes segments back as a path. Segments from Split join to the path
// they came from; a Key segment made with Escape joins as that key.
func Join(segs []Segment) string {
	var b strings.Builder
	for i, s := range segs {
		if i > 0 && !strings.HasPrefix(s.Raw, "|") {
			b.WriteByte('.')
		}
		b.WriteString(s.Raw)
	}
	return b.String()
}

// KeySegment returns a Key segment for key, escaped so that GJSON reads it as
// that key and nothing else.
func KeySegment(key string) Segment {
	return Segment{Kind: Key, Key: key, Raw: gjson.Escape(key)}
}

// readKey reads one key from the front of path, up to an unescaped "." or
// "|", as gjson's parseObjectPath does.
func readKey(path string) (Segment, string) {
	var key strings.Builder
	wild := false
	i := 0
	for ; i < len(path); i++ {
		c := path[i]
		if c == '.' || c == '|' {
			break
		}
		if c == '\\' {
			i++
			if i < len(path) {
				key.WriteByte(path[i])
			}
			continue
		}
		if c == '*' || c == '?' {
			wild = true
		}
		key.WriteByte(c)
	}
	seg := Segment{Kind: Key, Key: key.String(), Raw: path[:i]}
	switch {
	case wild:
		seg.Kind = Wildcard
	case seg.Raw == "#":
		seg.Kind = All
	}
	return seg, path[i:]
}

// queryEnd returns the index just past a "#(...)" or "#(...)#" query at the
// front of path, honoring nested parentheses and quoted strings, or -1 when
// the parentheses never close.
func queryEnd(path string) int {
	depth := 0
	for i := 1; i < len(path); i++ {
		switch path[i] {
		case '\\':
			i++
		case '"':
			for i++; i < len(path) && path[i] != '"'; i++ {
				if path[i] == '\\' {
					i++
				}
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				if i+1 < len(path) && path[i+1] == '#' {
					return i + 2
				}
				return i + 1
			}
		}
	}
	return -1
}

// startsSubExpression reports whether s begins with something gjson reads as
// other than a key: a registered modifier, or a multipath. At the start of a
// whole path a static literal ("!true") counts too.
func startsSubExpression(s string, atStart bool) bool {
	if s == "" {
		return false
	}
	switch s[0] {
	case '[', '{':
		return true
	case '!':
		return atStart
	case '@':
		if gjson.DisableModifiers {
			return false
		}
		end := strings.IndexAny(s, ".|:")
		if end < 0 {
			end = len(s)
		}
		return gjson.ModifierExists(s[1:end], nil)
	}
	return false
}
