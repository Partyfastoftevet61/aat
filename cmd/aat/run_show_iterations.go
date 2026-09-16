package main

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gburgyan/aat/archive"
)

// shownIterationRow is one request of a repeated step in aat run show's step
// view, and in the step's --json document.
type shownIterationRow struct {
	Index      int    `json:"index"`
	Status     int    `json:"status,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	UntilMet   bool   `json:"until_met,omitempty"`
	Retries    int    `json:"retries,omitempty"`
	Error      string `json:"error,omitempty"`
	// Sent, on a step that pages with repeat.next, holds the inputs this request
	// sent that differ from the step's own, which are the first request's: its
	// cursors, with null for one it no longer sent.
	Sent map[string]any `json:"sent,omitempty"`
	// Outputs holds the request's scalar outputs; --iteration prints them all.
	Outputs map[string]any `json:"outputs,omitempty"`

	outputsText string // every output on one line, arrays and objects by their size
}

func newShownIterationRow(step *archive.StepRecord, it archive.IterationRecord) shownIterationRow {
	row := shownIterationRow{
		Index:       it.Index,
		DurationMs:  it.DurationMs,
		UntilMet:    it.UntilMet,
		Retries:     it.RetryCount,
		Error:       it.Error,
		Sent:        sentInputs(step.Inputs, it.Inputs),
		outputsText: showValuesInline(it.Outputs, 80),
	}
	if it.Response != nil {
		row.Status = it.Response.Status
	}
	for name, v := range it.Outputs {
		switch v.(type) {
		case []any, map[string]any:
			continue
		}
		if row.Outputs == nil {
			row.Outputs = map[string]any{}
		}
		row.Outputs[name] = v
	}
	return row
}

// sentInputs returns the inputs a paging request sent that differ from the
// step's, with nil for an input the request no longer sent. A request that
// recorded no inputs of its own, as on a step without repeat.next, has none.
func sentInputs(step, sent map[string]any) map[string]any {
	if len(sent) == 0 {
		return nil
	}
	var diff map[string]any
	set := func(name string, v any) {
		if diff == nil {
			diff = map[string]any{}
		}
		diff[name] = v
	}
	for name, v := range sent {
		if old, ok := step[name]; !ok || !reflect.DeepEqual(old, v) {
			set(name, v)
		}
	}
	for name := range step {
		if _, ok := sent[name]; !ok {
			set(name, nil)
		}
	}
	return diff
}

// showValuesInline renders values on one line as "name value" pairs sorted by
// name, arrays and objects by their size, cut to limit characters.
func showValuesInline(values map[string]any, limit int) string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	pairs := make([]string, len(names))
	for i, name := range names {
		pairs[i] = name + " " + showValue(values[name])
	}
	return showShort(strings.Join(pairs, ", "), limit)
}

// writeShownIterations writes a repeated step's requests as a table: each one's
// status, time, and whether until held, the inputs it sent that differ from the
// step's when the step pages, and its outputs, or its error.
func writeShownIterations(b *strings.Builder, rows []shownIterationRow) {
	if len(rows) == 0 {
		return
	}
	sent := make([]string, len(rows))
	paging := false
	width := len("SENT")
	for i, row := range rows {
		sent[i] = "-"
		if len(row.Sent) > 0 {
			sent[i] = showValuesInline(row.Sent, 48)
			paging = true
		}
		width = max(width, utf8.RuneCountInString(sent[i]))
	}
	line := func(index, status, took, until, sentText, rest string) {
		text := fmt.Sprintf("%3s  %6s  %7s  %-5s  ", index, status, took, until)
		if paging {
			text += fmt.Sprintf("%-*s  ", width, sentText)
		}
		b.WriteString(strings.TrimRight(text+rest, " "))
		b.WriteByte('\n')
	}
	b.WriteString("requests:\n")
	line("#", "STATUS", "TIME", "UNTIL", "SENT", "OUTPUTS")
	for i, row := range rows {
		status := "-"
		if row.Status != 0 {
			status = strconv.Itoa(row.Status)
		}
		until := ""
		if row.UntilMet {
			until = "met"
		}
		rest := row.outputsText
		if row.Retries > 0 {
			rest = "retried " + pluralize(row.Retries, "time") + "; " + rest
		}
		if row.Error != "" {
			rest = "error: " + row.Error
		}
		line(strconv.Itoa(row.Index), status, formatDuration(time.Duration(row.DurationMs)*time.Millisecond), until, sent[i], rest)
	}
}

// findShownIteration returns request n of a repeated step, counting from 1.
func findShownIteration(step *archive.StepRecord, id string, n int) (*archive.IterationRecord, error) {
	switch count := len(step.Iterations); {
	case count == 0:
		return nil, fmt.Errorf("step %s recorded no repeated requests; --iteration reads a step with a repeat block", id)
	case n > count:
		return nil, fmt.Errorf("step %s sent %s; --iteration takes 1 to %d", id, pluralize(count, "request"), count)
	}
	return &step.Iterations[n-1], nil
}

// shownIteration is one request of a repeated step as aat run show --iteration
// prints it, and its --json document.
type shownIteration struct {
	StepID     string `json:"step_id"`
	Node       string `json:"node"`
	Index      int    `json:"index"`
	Requests   int    `json:"requests"` // the requests the step sent
	Method     string `json:"method,omitempty"`
	URL        string `json:"url,omitempty"`
	Status     int    `json:"status,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Retries    int    `json:"retries,omitempty"`
	UntilMet   bool   `json:"until_met,omitempty"`
	// RepeatStop, on the step's last request, says why the step stopped.
	RepeatStop string `json:"repeat_stop,omitempty"`
	Error      string `json:"error,omitempty"`
	// Inputs, on a step that pages with repeat.next, are the inputs this request
	// sent, its cursors included.
	Inputs            map[string]any               `json:"inputs,omitempty"`
	Outputs           map[string]any               `json:"outputs,omitempty"`
	OASValidation     *archive.OASValidationRecord `json:"oas_validation,omitempty"`
	RequestBodyBytes  int                          `json:"request_body_bytes,omitempty"`
	ResponseBodyBytes int                          `json:"response_body_bytes,omitempty"`
}

func buildShownIteration(step *archive.StepRecord, it *archive.IterationRecord, id string) shownIteration {
	view := shownIteration{
		StepID:        id,
		Node:          step.Node,
		Index:         it.Index,
		Requests:      len(step.Iterations),
		DurationMs:    it.DurationMs,
		Retries:       it.RetryCount,
		UntilMet:      it.UntilMet,
		Error:         it.Error,
		Inputs:        it.Inputs,
		Outputs:       it.Outputs,
		OASValidation: it.OASValidation,
	}
	if it.Index == len(step.Iterations) {
		view.RepeatStop = step.RepeatStop
	}
	if it.Request != nil {
		view.Method, view.URL = it.Request.Method, it.Request.URL
		view.RequestBodyBytes = compactSize(it.Request.Body)
	}
	if it.Response != nil {
		view.Status = it.Response.Status
		view.ResponseBodyBytes = compactSize(it.Response.Body)
	}
	return view
}

// showIteration prints one request of a repeated step: where it went, what came
// back, whether until held, the inputs it sent when the step pages, its outputs,
// its OpenAPI validation, and the sizes of its bodies.
func showIteration(out io.Writer, step *archive.StepRecord, it *archive.IterationRecord, id string, format showFormat) error {
	view := buildShownIteration(step, it, id)
	if format != showText {
		return writeShowJSON(out, view, format == showCompactJSON)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "step %s", view.StepID)
	if view.Node != view.StepID {
		fmt.Fprintf(&b, " (node %s)", view.Node)
	}
	fmt.Fprintf(&b, ", request %d of %d\n", view.Index, view.Requests)
	if view.Method != "" {
		fmt.Fprintf(&b, "%s %s\n", view.Method, view.URL)
	}
	status := "no response"
	if view.Status != 0 {
		status = fmt.Sprintf("status %d", view.Status)
	}
	fmt.Fprintf(&b, "%s  %s", status, formatDuration(time.Duration(view.DurationMs)*time.Millisecond))
	if view.Retries > 0 {
		b.WriteString("  retried " + pluralize(view.Retries, "time"))
	}
	if view.UntilMet {
		b.WriteString("  until met")
	}
	if view.RepeatStop != "" {
		fmt.Fprintf(&b, "  (stopped: %s)", view.RepeatStop)
	}
	b.WriteByte('\n')
	if view.Error != "" {
		fmt.Fprintf(&b, "error: %s\n", view.Error)
	}
	writeShownValues(&b, "inputs", view.Inputs)
	writeShownValues(&b, "outputs", view.Outputs)
	writeShownOAS(&b, view.OASValidation)
	fmt.Fprintf(&b, "request body: %s\n", showSize(view.RequestBodyBytes))
	fmt.Fprintf(&b, "response body: %s\n", showSize(view.ResponseBodyBytes))
	if view.ResponseBodyBytes > 0 {
		fmt.Fprintf(&b, "\nNext: --iteration %d --response --shape for its response's structure, --iteration %d --response --path PATH for one part of it\n", view.Index, view.Index)
	}
	_, err := io.WriteString(out, b.String())
	return err
}

// writeShownOAS writes a request's OpenAPI validation: whether its request and
// response bodies were valid, then each violation.
func writeShownOAS(b *strings.Builder, v *archive.OASValidationRecord) {
	if v == nil {
		return
	}
	if v.Skipped {
		fmt.Fprintf(b, "oas: not validated (%s)\n", v.SkipReason)
		return
	}
	var parts, violations []string
	add := func(what string, p *archive.OASPayloadRecord) {
		switch {
		case p == nil:
		case p.Skipped:
			parts = append(parts, what+" not validated")
		case p.Valid:
			parts = append(parts, what+" valid")
		default:
			parts = append(parts, what+" "+pluralize(len(p.Errors), "violation"))
			for _, e := range p.Errors {
				violations = append(violations, fmt.Sprintf("  %s %s: %s", what, e.Path, e.Message))
			}
		}
	}
	add("request", v.Request)
	add("response", v.Response)
	if len(parts) == 0 {
		return
	}
	fmt.Fprintf(b, "oas: %s\n", strings.Join(parts, ", "))
	for _, violation := range violations {
		b.WriteString(violation)
		b.WriteByte('\n')
	}
}

// showIterationPart prints one part of a repeated step's request, as
// showStepPart does for a step.
func showIterationPart(out, errOut io.Writer, it *archive.IterationRecord, id string, opts showOptions) error {
	if opts.Part == "inputs" && len(it.Inputs) == 0 {
		return fmt.Errorf("request %d of step %s recorded no inputs of its own: without repeat.next every request sends the step's inputs, which --step %s --inputs prints", it.Index, id, id)
	}
	doc, err := iterationPartJSON(it, opts.Part)
	if err != nil {
		return err
	}
	return showPart(out, errOut, doc, fmt.Sprintf("request %d of step %s", it.Index, id), opts)
}

// iterationPartJSON returns a part of a repeated step's request as JSON, or
// nothing when the request has none.
func iterationPartJSON(it *archive.IterationRecord, part string) ([]byte, error) {
	switch part {
	case "request":
		if it.Request == nil {
			return nil, nil
		}
		return it.Request.Body, nil
	case "response":
		if it.Response == nil {
			return nil, nil
		}
		return it.Response.Body, nil
	case "inputs":
		if len(it.Inputs) == 0 {
			return nil, nil
		}
		return json.Marshal(it.Inputs)
	default:
		if len(it.Outputs) == 0 {
			return nil, nil
		}
		return json.Marshal(it.Outputs)
	}
}
