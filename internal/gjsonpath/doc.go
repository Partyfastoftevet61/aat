// Package gjsonpath splits a GJSON path into the segments GJSON itself reads,
// so a static check can walk a schema the way extraction walks a response.
//
// Extraction hands a whole path to gjson.Get, which reads it one component at
// a time: a key (with backslash escapes), an index, "#" for every element or
// the count, a "#(...)" query, and a few forms that are not keys at all —
// modifiers such as "@reverse", pipes, and multipaths. Splitting on "." gets
// escaped dots and queries wrong, so a checker that did that would report a
// valid path as broken, or pass a broken one. Split follows gjson v1.18.0's
// own rules and marks the forms it does not model as Opaque, which a checker
// treats as "nothing more to check" rather than as an error.
package gjsonpath
