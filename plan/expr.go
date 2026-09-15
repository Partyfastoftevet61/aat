package plan

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ExprContext provides runtime values for expression evaluation.
type ExprContext struct {
	Now    time.Time           // anchor for "today", "now", and "unixtime" (default: time.Now())
	Env    func(string) string // env var lookup (default: os.Getenv)
	Values map[string]any      // already-resolved inputs for relative refs
	Random io.Reader           // source for "uuid" and "random N" (default: crypto/rand)
	// Outputs looks up an earlier step's output for a {{step.output}}
	// reference. Assertions and repeat conditions set it; where it is nil, such
	// a reference is an error.
	Outputs func(stepID, output string) (any, error)
}

// defaults fills in zero-valued fields with production defaults.
func (ec ExprContext) defaults() ExprContext {
	if ec.Now.IsZero() {
		ec.Now = time.Now()
	}
	if ec.Env == nil {
		ec.Env = os.Getenv
	}
	if ec.Values == nil {
		ec.Values = make(map[string]any)
	}
	if ec.Random == nil {
		ec.Random = crand.Reader
	}
	return ec
}

// ContainsExpr reports whether s contains at least one {{...}} expression.
func ContainsExpr(s string) bool {
	return strings.Contains(s, "{{")
}

// ContainsExprValue reports whether v holds a {{...}} expression: a string that
// contains one, or a list or map with one in an item, at any depth.
func ContainsExprValue(v any) bool {
	switch t := v.(type) {
	case string:
		return ContainsExpr(t)
	case []any:
		return slices.ContainsFunc(t, ContainsExprValue)
	case map[string]any:
		for _, item := range t {
			if ContainsExprValue(item) {
				return true
			}
		}
	}
	return false
}

// ValidateExpr checks that all {{...}} expressions in raw are syntactically valid
// without evaluating them.
func ValidateExpr(raw string) error {
	segments, err := splitExprSegments(raw)
	if err != nil {
		return err
	}
	for _, seg := range segments {
		if seg.isExpr {
			if _, err := parseExprInner(seg.text); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateExprValue checks the expressions in v the way ValidateExpr checks a
// string, in the items of lists and maps too, at any depth. An error names the
// item.
func ValidateExprValue(v any) error {
	switch t := v.(type) {
	case string:
		if ContainsExpr(t) {
			return ValidateExpr(t)
		}
	case []any:
		for i, item := range t {
			if err := ValidateExprValue(item); err != nil {
				return fmt.Errorf("item %d: %w", i, err)
			}
		}
	case map[string]any:
		for _, key := range slices.Sorted(maps.Keys(t)) {
			if err := ValidateExprValue(t[key]); err != nil {
				return fmt.Errorf("key %q: %w", key, err)
			}
		}
	}
	return nil
}

// EvalExpr evaluates expression templates in raw. A string without {{...}}
// delimiters, and any other scalar, is returned unchanged. A list or map is
// evaluated item by item, at any depth, into a new list or map, so the plan's
// own value keeps its expressions for the next run; a list or map without
// expressions is returned as it is.
func EvalExpr(raw any, ctx ExprContext) (any, error) {
	switch t := raw.(type) {
	case []any:
		if !ContainsExprValue(t) {
			return raw, nil
		}
		out := make([]any, len(t))
		for i, item := range t {
			v, err := EvalExpr(item, ctx)
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", i, err)
			}
			out[i] = v
		}
		return out, nil
	case map[string]any:
		if !ContainsExprValue(t) {
			return raw, nil
		}
		out := make(map[string]any, len(t))
		// Sorted keys draw generated values in the same order every run.
		for _, key := range slices.Sorted(maps.Keys(t)) {
			v, err := EvalExpr(t[key], ctx)
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", key, err)
			}
			out[key] = v
		}
		return out, nil
	}

	s, ok := raw.(string)
	if !ok {
		return raw, nil
	}
	if !ContainsExpr(s) {
		return raw, nil
	}

	ctx = ctx.defaults()

	segments, err := splitExprSegments(s)
	if err != nil {
		return nil, err
	}

	// Single expression with no surrounding text: return the computed value directly.
	if len(segments) == 1 && segments[0].isExpr {
		return evalOneExpr(segments[0].text, ctx)
	}

	// Mixed: concatenate all segments as strings.
	var b strings.Builder
	for _, seg := range segments {
		if seg.isExpr {
			val, err := evalOneExpr(seg.text, ctx)
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&b, "%v", val)
		} else {
			b.WriteString(seg.text)
		}
	}
	return b.String(), nil
}

// segment is a piece of the input string, either literal text or an expression.
type segment struct {
	text   string
	isExpr bool
}

// splitExprSegments splits a string into literal and expression segments.
func splitExprSegments(s string) ([]segment, error) {
	var segments []segment
	for {
		start := strings.Index(s, "{{")
		if start == -1 {
			if s != "" {
				segments = append(segments, segment{text: s})
			}
			break
		}
		if start > 0 {
			segments = append(segments, segment{text: s[:start]})
		}
		end := strings.Index(s[start:], "}}")
		if end == -1 {
			return nil, fmt.Errorf("unclosed expression delimiter in %q", s)
		}
		inner := strings.TrimSpace(s[start+2 : start+end])
		if inner == "" {
			return nil, fmt.Errorf("empty expression in %q", s)
		}
		segments = append(segments, segment{text: inner, isExpr: true})
		s = s[start+end+2:]
	}
	return segments, nil
}

// exprKind identifies the type of a parsed expression.
type exprKind int

const (
	exprToday    exprKind = iota // "today" optionally with offset
	exprEnv                      // "env.VAR"
	exprRef                      // "identifier" optionally with offset
	exprUUID                     // "uuid"
	exprRandom                   // "random N"
	exprNow                      // "now" optionally with a time offset
	exprUnixtime                 // "unixtime" optionally with a time offset
	exprOutput                   // "step.output", an earlier step's output
)

// parsedExpr is an intermediate representation of a single expression.
type parsedExpr struct {
	kind     exprKind
	envVar   string        // for exprEnv
	refName  string        // for exprRef
	stepID   string        // for exprOutput
	output   string        // for exprOutput
	offset   int           // days offset for exprToday (positive or negative)
	hasArith bool          // whether an offset was written
	length   int           // for exprRandom
	duration time.Duration // time offset for exprNow, exprUnixtime, and a reference's unit offset
	hasUnit  bool          // a reference's offset names a unit of time
	number   string        // a reference's offset without a unit, signed as written, such as "-500"
}

// maxRandomLength is the longest value {{random N}} generates.
const maxRandomLength = 64

// Regex for expression parsing.
var (
	exprEnvRe       = regexp.MustCompile(`^env\.([A-Za-z_][A-Za-z0-9_]*)$`)
	exprOutputRe    = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)$`)
	exprDashedRefRe = regexp.MustCompile(`^[A-Za-z0-9_]*-[A-Za-z0-9_-]*\.[A-Za-z_][A-Za-z0-9_]*$`)
	exprOutputRefRe = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)(\s*[+-]\s*\d+(?:\.\d+)?(?:\s+[A-Za-z]+)?)?\s*\}\}`)
	exprArithRe     = regexp.MustCompile(`^(\S+)\s*([+-])\s*(\d+)\s+days?$`)
	exprIdentOnlyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	exprClockRe     = regexp.MustCompile(`^(now|unixtime)(?:\s*([+-])\s*(\d+)\s+([A-Za-z]+))?$`)
	exprRandomRe    = regexp.MustCompile(`^random\s+(\S+)$`)
)

func parseExprInner(inner string) (*parsedExpr, error) {
	inner = strings.TrimSpace(inner)

	// env.VAR
	if m := exprEnvRe.FindStringSubmatch(inner); m != nil {
		return &parsedExpr{kind: exprEnv, envVar: m[1]}, nil
	}

	// step.output, an earlier step's output. env.NAME above always means the
	// environment.
	if m := exprOutputRe.FindStringSubmatch(inner); m != nil {
		return &parsedExpr{kind: exprOutput, stepID: m[1], output: m[2]}, nil
	}
	if exprDashedRefRe.MatchString(inner) {
		return nil, fmt.Errorf("invalid expression syntax: %q; step IDs in {{step.output}} use letters, digits, and underscores, so give the step such an id", inner)
	}

	// now and unixtime, optionally +/- N seconds, minutes, hours, or days
	if m := exprClockRe.FindStringSubmatch(inner); m != nil {
		pe := &parsedExpr{kind: exprNow}
		if m[1] == "unixtime" {
			pe.kind = exprUnixtime
		}
		if m[2] != "" {
			d, err := offsetDuration(m[3], m[4], inner)
			if err != nil {
				return nil, err
			}
			if m[2] == "-" {
				d = -d
			}
			pe.duration = d
		}
		return pe, nil
	}

	// uuid
	if inner == "uuid" {
		return &parsedExpr{kind: exprUUID}, nil
	}

	// random N
	if m := exprRandomRe.FindStringSubmatch(inner); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 1 || n > maxRandomLength {
			return nil, fmt.Errorf("random takes a length from 1 to %d, not %q, in %q", maxRandomLength, m[1], inner)
		}
		return &parsedExpr{kind: exprRandom, length: n}, nil
	}

	// today +/- N days
	if m := exprArithRe.FindStringSubmatch(inner); m != nil && m[1] == "today" {
		n, err := strconv.Atoi(m[3])
		if err != nil {
			return nil, fmt.Errorf("invalid day count in expression %q: %w", inner, err)
		}
		if m[2] == "-" {
			n = -n
		}
		return &parsedExpr{kind: exprToday, offset: n, hasArith: true}, nil
	}

	// A reference with an offset: an input or an earlier step's output, plus
	// or minus a number, or a number of units of time
	if m := exprRefOffsetRe.FindStringSubmatch(inner); m != nil {
		return parseRefOffset(inner, m)
	}

	// Plain "today"
	if inner == "today" {
		return &parsedExpr{kind: exprToday}, nil
	}

	// Plain identifier reference
	if exprIdentOnlyRe.MatchString(inner) {
		return &parsedExpr{kind: exprRef, refName: inner}, nil
	}

	return nil, fmt.Errorf("invalid expression syntax: %q", inner)
}

var (
	// exprRefOffsetRe matches a reference with an offset: an input or
	// step.output, then + or - a number, then optionally a unit of time.
	exprRefOffsetRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)(?:\.([A-Za-z_][A-Za-z0-9_]*))?\s*([+-])\s*(\d+(?:\.\d+)?)(?:\s+([A-Za-z]+))?$`)
	// decimalTextRe matches a decimal number written as text, such as "221.78".
	decimalTextRe = regexp.MustCompile(`^[+-]?\d+(?:\.\d+)?$`)
)

// parseRefOffset parses a reference with an offset, as exprRefOffsetRe matched
// it in inner: an input or step.output, plus or minus a number, with a unit for
// a time offset. A base that isn't a reference says what it takes instead.
func parseRefOffset(inner string, m []string) (*parsedExpr, error) {
	base, output, sign, number, unit := m[1], m[2], m[3], m[4], m[5]
	if output == "" {
		switch base {
		case "uuid", "random":
			return nil, fmt.Errorf("%s takes no offset, in %q", base, inner)
		case "today":
			if unit != "" {
				return nil, fmt.Errorf("today counts days, in %q; for a time use {{now %s %s %s}} or {{unixtime %s %s %s}}",
					inner, sign, number, unit, sign, number, unit)
			}
			return nil, fmt.Errorf("today takes an offset in days, as {{today %s %s days}}, in %q", sign, number, inner)
		case "now", "unixtime":
			return nil, fmt.Errorf("%s takes an offset with a unit, as {{%s %s %s minutes}}, in %q", base, base, sign, number, inner)
		}
	} else if base == "env" {
		return nil, fmt.Errorf("an environment variable takes no offset, in %q", inner)
	}

	pe := &parsedExpr{kind: exprRef, refName: base, hasArith: true}
	if output != "" {
		pe = &parsedExpr{kind: exprOutput, stepID: base, output: output, hasArith: true}
	}
	if unit == "" {
		pe.number = sign + number
		return pe, nil
	}
	if strings.Contains(number, ".") {
		return nil, fmt.Errorf("a time offset counts whole units, not %s, in %q", number, inner)
	}
	d, err := offsetDuration(number, unit, inner)
	if err != nil {
		return nil, err
	}
	if sign == "-" {
		d = -d
	}
	pe.duration, pe.hasUnit = d, true
	return pe, nil
}

func evalOneExpr(inner string, ctx ExprContext) (any, error) {
	pe, err := parseExprInner(inner)
	if err != nil {
		return nil, err
	}

	switch pe.kind {
	case exprToday:
		d := ctx.Now.AddDate(0, 0, pe.offset)
		return d.Format("2006-01-02"), nil

	case exprEnv:
		val := ctx.Env(pe.envVar)
		if val == "" {
			return nil, fmt.Errorf("environment variable %q is not set or empty", pe.envVar)
		}
		return val, nil

	case exprUUID:
		return newUUID(ctx.Random)

	case exprRandom:
		return randomString(ctx.Random, pe.length)

	case exprNow:
		return ctx.Now.Add(pe.duration).UTC().Format(time.RFC3339), nil

	case exprUnixtime:
		return ctx.Now.Add(pe.duration).Unix(), nil

	case exprRef:
		raw, ok := ctx.Values[pe.refName]
		if !ok {
			if pe.refName == "random" {
				return nil, fmt.Errorf("reference %q not found in resolved values; for a random value write {{random N}}", pe.refName)
			}
			return nil, fmt.Errorf("reference %q not found in resolved values", pe.refName)
		}
		if !pe.hasArith {
			// Plain reference: return as-is
			return raw, nil
		}
		return applyOffset(fmt.Sprintf("reference %q", pe.refName), raw, pe)

	case exprOutput:
		if ctx.Outputs == nil {
			return nil, fmt.Errorf("{{%s.%s}} reads a step's output, which can't be read here; step values, assertions, repeat.until, and selection filters can read one",
				pe.stepID, pe.output)
		}
		v, err := ctx.Outputs(pe.stepID, pe.output)
		if err != nil {
			return nil, err
		}
		if v, err = outputExprValue(pe.stepID, pe.output, v); err != nil || !pe.hasArith {
			return v, err
		}
		return applyOffset(fmt.Sprintf("{{%s.%s}}", pe.stepID, pe.output), v, pe)

	default:
		return nil, fmt.Errorf("unknown expression kind %d", pe.kind)
	}
}

// applyOffset adds a reference's offset to its value v; what names the
// reference in errors. A time offset moves Unix seconds by that much time, or a
// YYYY-MM-DD date by whole days. A number offset adds to a number.
func applyOffset(what string, v any, pe *parsedExpr) (any, error) {
	if !pe.hasUnit {
		return addNumber(what, v, pe.number)
	}
	if date, ok := v.(string); ok {
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, fmt.Errorf("%s value %q is not a valid date (YYYY-MM-DD): %w", what, date, err)
		}
		if pe.duration%(24*time.Hour) != 0 {
			return nil, fmt.Errorf("%s is a date, which moves by whole days", what)
		}
		return t.AddDate(0, 0, int(pe.duration/(24*time.Hour))).Format("2006-01-02"), nil
	}
	secs, ok := wholeNumber(v)
	if !ok {
		return nil, fmt.Errorf("%s is %T, not a date string or Unix seconds", what, v)
	}
	return secs + int64(pe.duration/time.Second), nil
}

// wholeNumber returns v as an int64 when it is a whole number: an int, an
// int64, a json.Number that holds one, or a float64 without a fraction.
func wholeNumber(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case json.Number:
		i, err := x.Int64()
		return i, err == nil
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1<<53 {
			return int64(x), true
		}
	}
	return 0, false
}

// addNumber adds offset, a signed number as written such as "-500" or "+0.25",
// to v. A whole number plus a whole offset is an int64, and any other pair of
// numbers a float64. A decimal number written as text, such as "221.78", gives
// text with as many decimal places as the more precise of the two.
func addNumber(what string, v any, offset string) (any, error) {
	if s, ok := v.(string); ok {
		sum, ok := addDecimalText(s, offset)
		if !ok {
			return nil, fmt.Errorf("%s value %q is not a number", what, s)
		}
		return sum, nil
	}
	if i, ok := wholeNumber(v); ok && !strings.Contains(offset, ".") {
		n, err := strconv.ParseInt(offset, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("offset %s is out of range: %w", offset, err)
		}
		return i + n, nil
	}
	var base float64
	switch x := v.(type) {
	case int:
		base = float64(x)
	case int64:
		base = float64(x)
	case float64:
		base = x
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return nil, fmt.Errorf("%s value %q is not a number", what, x.String())
		}
		base = f
	default:
		return nil, fmt.Errorf("%s is %T, not a number", what, v)
	}
	n, err := strconv.ParseFloat(offset, 64)
	if err != nil {
		return nil, fmt.Errorf("offset %s is not a number: %w", offset, err)
	}
	return base + n, nil
}

// addDecimalText adds offset to s, both decimal numbers written as text, and
// writes the sum with as many decimal places as the more precise of the two. It
// reports false when s isn't such a number or the sum is out of range.
func addDecimalText(s, offset string) (string, bool) {
	if !decimalTextRe.MatchString(s) {
		return "", false
	}
	places := max(decimalPlaces(s), decimalPlaces(offset))
	a, errA := scaledInt(s, places)
	b, errB := scaledInt(offset, places)
	if errA != nil || errB != nil {
		return "", false
	}
	return formatScaled(a+b, places), true
}

// decimalPlaces counts the digits after a decimal number's point.
func decimalPlaces(s string) int {
	_, frac, _ := strings.Cut(s, ".")
	return len(frac)
}

// scaledInt returns decimal text s times 10^places as an integer; s has at most
// that many decimal places.
func scaledInt(s string, places int) (int64, error) {
	whole, frac, _ := strings.Cut(s, ".")
	return strconv.ParseInt(whole+frac+strings.Repeat("0", places-len(frac)), 10, 64)
}

// formatScaled writes n divided by 10^places, with exactly places decimal
// places.
func formatScaled(n int64, places int) string {
	if places == 0 {
		return strconv.FormatInt(n, 10)
	}
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	digits := strconv.FormatInt(n, 10)
	if len(digits) <= places {
		digits = strings.Repeat("0", places-len(digits)+1) + digits
	}
	return sign + digits[:len(digits)-places] + "." + digits[len(digits)-places:]
}

// outputExprValue returns a step's output as a {{step.output}} reference reads
// it: a number, a boolean, or text. An extracted json.Number becomes an int64 or
// a float64. A null, list, or object output is an error, since a reference reads
// a single value.
func outputExprValue(stepID, output string, v any) (any, error) {
	switch x := v.(type) {
	case nil:
		return nil, fmt.Errorf("step %q output %q is null", stepID, output)
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i, nil
		}
		if f, err := x.Float64(); err == nil {
			return f, nil
		}
		return x.String(), nil
	case []any:
		return nil, fmt.Errorf("step %q output %q is a list; a reference reads a string, number, or boolean", stepID, output)
	case map[string]any:
		return nil, fmt.Errorf("step %q output %q is an object; a reference reads a string, number, or boolean", stepID, output)
	}
	return v, nil
}

// OutputRef names an earlier step's output that a {{step.output}} expression
// reads.
type OutputRef struct {
	Step   string
	Output string
}

// String returns the reference as it is written, as {{step.output}}.
func (r OutputRef) String() string { return "{{" + r.Step + "." + r.Output + "}}" }

// ExprOutputRefs returns the {{step.output}} references among raw's
// expressions, in order. Text that doesn't parse holds none; ValidateExpr
// reports why.
func ExprOutputRefs(raw string) []OutputRef {
	if !ContainsExpr(raw) {
		return nil
	}
	segments, err := splitExprSegments(raw)
	if err != nil {
		return nil
	}
	var refs []OutputRef
	for _, seg := range segments {
		if !seg.isExpr {
			continue
		}
		if pe, err := parseExprInner(seg.text); err == nil && pe.kind == exprOutput {
			refs = append(refs, OutputRef{Step: pe.stepID, Output: pe.output})
		}
	}
	return refs
}

// ExprValueOutputRefs returns the {{step.output}} references in v: a string's,
// or those in the items of a list or map, at any depth.
func ExprValueOutputRefs(v any) []OutputRef {
	switch t := v.(type) {
	case string:
		return ExprOutputRefs(t)
	case []any:
		var refs []OutputRef
		for _, item := range t {
			refs = append(refs, ExprValueOutputRefs(item)...)
		}
		return refs
	case map[string]any:
		var refs []OutputRef
		for _, key := range slices.Sorted(maps.Keys(t)) {
			refs = append(refs, ExprValueOutputRefs(t[key])...)
		}
		return refs
	}
	return nil
}

// RewriteExprRefs renames the steps that s's {{step.output}} expressions read,
// as idMap maps old step IDs to new ones. {{env.NAME}} and references to steps
// idMap doesn't name are left as they are.
func RewriteExprRefs(s string, idMap map[string]string) string {
	if !ContainsExpr(s) {
		return s
	}
	return exprOutputRefRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := exprOutputRefRe.FindStringSubmatch(m)
		if newID, ok := idMap[sub[1]]; ok && sub[1] != "env" {
			return "{{" + newID + "." + sub[2] + sub[3] + "}}" // an offset stays with its reference
		}
		return m
	})
}

// offsetDuration converts an offset of number units, as written in expression
// inner, to a duration. A day is 24 hours.
func offsetDuration(number, unit, inner string) (time.Duration, error) {
	n, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid offset in expression %q: %w", inner, err)
	}
	var per time.Duration
	switch strings.TrimSuffix(strings.ToLower(unit), "s") {
	case "second":
		per = time.Second
	case "minute":
		per = time.Minute
	case "hour":
		per = time.Hour
	case "day":
		per = 24 * time.Hour
	default:
		return 0, fmt.Errorf("unknown time unit %q in expression %q; use seconds, minutes, hours, or days", unit, inner)
	}
	if n > int64(math.MaxInt64/per) {
		return 0, fmt.Errorf("offset too large in expression %q", inner)
	}
	return time.Duration(n) * per, nil
}

// newUUID returns a random version 4 UUID, in lowercase, built from bytes read
// from r.
func newUUID(r io.Reader) (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return "", fmt.Errorf("generating uuid: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// randomAlphabet is what {{random N}} draws from: digits and lowercase letters,
// which need no escaping in a path, query, header, or JSON string, and stay
// distinct where case is ignored.
const randomAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// randomString returns n characters drawn uniformly from randomAlphabet, using
// bytes read from r. A byte of 252 or more is skipped, since 252 is the largest
// multiple of 36 below 256, so every character is equally likely.
func randomString(r io.Reader, n int) (string, error) {
	out := make([]byte, 0, n)
	buf := make([]byte, n)
	for len(out) < n {
		chunk := buf[:n-len(out)]
		if _, err := io.ReadFull(r, chunk); err != nil {
			return "", fmt.Errorf("generating random value: %w", err)
		}
		for _, c := range chunk {
			if c < 252 {
				out = append(out, randomAlphabet[c%36])
			}
		}
	}
	return string(out), nil
}
