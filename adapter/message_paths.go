package adapter

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gburgyan/aat/internal/gjsonpath"
)

// MessageInputPaths maps each input a gRPC template sends to where its message
// puts it: the GJSON path of the value the placeholder is, or is inside. An
// input used as a key is placed at its object's path with a "*" for the key; an
// iteration block's list at the path of the array or object it fills. An input
// that reaches the request with no field to check, only in metadata, maps to an
// empty list.
//
// The second result is false when the message can't be read as one JSON object
// with placeholders in it — unbalanced brackets, an unterminated string or
// placeholder — and then the map holds nothing to rely on.
func (t *Template) MessageInputPaths() (map[string][]string, bool) {
	paths, ok := jsonPlaceholderPaths(t.Request.Message)
	if !ok {
		return nil, false
	}
	for _, value := range t.Request.Metadata {
		for _, name := range placeholderNames(value) {
			if _, placed := paths[name]; !placed {
				paths[name] = []string{}
			}
		}
	}
	return paths, true
}

// jsonFrame is an object or array the scan is inside.
type jsonFrame struct {
	array bool
	// index is the element an array is on.
	index int
	// key is the segment of the value an object is on, and keyNext says the
	// next string is a key rather than a value.
	key     string
	keyNext bool
}

// jsonPlaceholderPaths scans a JSON template for placeholders, tolerating
// what makes a template not JSON yet: conditional and iteration tags between
// members, and placeholders standing for whole values.
func jsonPlaceholderPaths(text string) (map[string][]string, bool) {
	paths := make(map[string][]string)
	record := func(name, path string) {
		for _, p := range paths[name] {
			if p == path {
				return
			}
		}
		paths[name] = append(paths[name], path)
	}

	var stack []*jsonFrame
	// valuePath is the path of the value the scan is at: each frame's key or
	// index, from the outside in.
	valuePath := func() string {
		segs := make([]string, 0, len(stack))
		for _, f := range stack {
			if f.array {
				segs = append(segs, strconv.Itoa(f.index))
			} else {
				segs = append(segs, f.key)
			}
		}
		return strings.Join(segs, ".")
	}
	// containerPath is the path of the object or array the scan is in.
	containerPath := func() string {
		saved := stack
		stack = stack[:len(stack)-1]
		p := valuePath()
		stack = saved
		return p
	}
	top := func() *jsonFrame { return stack[len(stack)-1] }
	join := func(parent, seg string) string {
		if parent == "" {
			return seg
		}
		return parent + "." + seg
	}

	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "{") {
		return nil, false
	}
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '{' && strings.HasPrefix(text[i:], "{{"):
			end := strings.Index(text[i+2:], "}}")
			if end < 0 {
				return nil, false
			}
			tag := strings.TrimSpace(text[i+2 : i+2+end])
			i += end + 3
			if len(stack) == 0 {
				return nil, false
			}
			switch {
			case strings.HasPrefix(tag, "?"), strings.HasPrefix(tag, "/"):
				// A block's content stays where it is.
			case strings.HasPrefix(tag, "#"):
				// The list fills the array or object the block is in.
				record(strings.TrimSpace(tag[1:]), containerPath())
			case isElementRef(tag):
				// The element an iteration block is on, not an input.
			case !top().array && top().keyNext:
				// A placeholder written where a key goes: any key of the object.
				record(tag, join(containerPath(), "*"))
				top().keyNext = false
			default:
				record(tag, valuePath())
			}
		case c == '"':
			end, names, ok := scanJSONString(text, i)
			if !ok {
				return nil, false
			}
			if len(stack) == 0 {
				return nil, false
			}
			f := top()
			if !f.array && f.keyNext {
				f.keyNext = false
				key, isLiteral := jsonKey(text[i : end+1])
				if !isLiteral {
					for _, name := range names {
						if !isElementRef(name) {
							record(name, join(containerPath(), "*"))
						}
					}
					f.key = "*"
				} else {
					f.key = gjsonpath.KeySegment(key).Raw
				}
			} else {
				for _, name := range names {
					if !isElementRef(name) {
						record(name, valuePath())
					}
				}
			}
			i = end
		case c == '{':
			stack = append(stack, &jsonFrame{keyNext: true})
		case c == '[':
			stack = append(stack, &jsonFrame{array: true})
		case c == '}' || c == ']':
			if len(stack) == 0 || top().array != (c == ']') {
				return nil, false
			}
			stack = stack[:len(stack)-1]
		case c == ',':
			if len(stack) == 0 {
				return nil, false
			}
			if f := top(); f.array {
				f.index++
			} else {
				f.keyNext, f.key = true, ""
			}
		}
	}
	if len(stack) != 0 {
		return nil, false
	}
	return paths, true
}

// scanJSONString reads the JSON string starting at text[start], a quote. It
// returns the index of the closing quote and the names of the placeholders
// inside, element references included.
func scanJSONString(text string, start int) (int, []string, bool) {
	var names []string
	for i := start + 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '"':
			return i, names, true
		case '{':
			if strings.HasPrefix(text[i:], "{{") {
				end := strings.Index(text[i+2:], "}}")
				if end < 0 {
					return 0, nil, false
				}
				tag := strings.TrimSpace(text[i+2 : i+2+end])
				if tag != "" && !strings.ContainsAny(tag[:1], "?/#") {
					names = append(names, tag)
				}
				i += end + 3
			}
		}
	}
	return 0, nil, false
}

// jsonKey decodes a quoted JSON key. A key holding a placeholder is not a
// literal: the input picks the key.
func jsonKey(quoted string) (string, bool) {
	if strings.Contains(quoted, "{{") {
		return "", false
	}
	var key string
	if err := json.Unmarshal([]byte(quoted), &key); err != nil {
		return strings.Trim(quoted, `"`), true
	}
	return key, true
}

// isElementRef reports whether a placeholder names the element an iteration
// block is on: {{.}}, {{.field}}, or {{@index}}.
func isElementRef(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@")
}

// placeholderNames returns the inputs a text's placeholders name, leaving out
// block tags and element references.
func placeholderNames(text string) []string {
	var names []string
	for {
		start := strings.Index(text, "{{")
		if start < 0 {
			return names
		}
		end := strings.Index(text[start+2:], "}}")
		if end < 0 {
			return names
		}
		tag := strings.TrimSpace(text[start+2 : start+2+end])
		if tag != "" && !strings.ContainsAny(tag[:1], "?/#.@") {
			names = append(names, tag)
		}
		text = text[start+2+end+2:]
	}
}
