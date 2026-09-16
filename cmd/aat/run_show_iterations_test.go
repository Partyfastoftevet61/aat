package main

import (
	"encoding/json"
	"testing"

	"github.com/gburgyan/aat/archive"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// showRepeatArchive is showTestArchive with two repeated steps: waitForExport
// polls until its third response is complete, retrying its second request, and
// listOrders reads two pages with a cursor.
func showRepeatArchive() *archive.Archive {
	a := showTestArchive()
	poll := func(index int, status string, met bool) archive.IterationRecord {
		return archive.IterationRecord{
			Index:      index,
			DurationMs: int64(index),
			Request:    &archive.RequestRecord{Method: "GET", URL: "http://localhost:8765/us/v1/exports/exp_0001"},
			Response:   &archive.ResponseRecord{Status: 200, Body: json.RawMessage(`{"status": "` + status + `", "rows": []}`)},
			Outputs:    map[string]any{"status": status, "rows": []any{}},
			UntilMet:   met,
		}
	}
	polls := []archive.IterationRecord{poll(1, "running", false), poll(2, "running", false), poll(3, "complete", true)}
	polls[1].RetryCount = 1
	polls[2].OASValidation = &archive.OASValidationRecord{
		OperationID: "getExport",
		Request:     &archive.OASPayloadRecord{Skipped: true, SkipReason: "no body"},
		Response:    &archive.OASPayloadRecord{Errors: []archive.OASSchemaError{{Path: "/rows", Message: "expected array"}}},
	}
	last := polls[2]
	page := func(index int, inputs map[string]any, body string, outputs map[string]any) archive.IterationRecord {
		return archive.IterationRecord{
			Index:      index,
			DurationMs: 1,
			Inputs:     inputs,
			Request:    &archive.RequestRecord{Method: "GET", URL: "http://localhost:8765/us/v1/orders"},
			Response:   &archive.ResponseRecord{Status: 200, Body: json.RawMessage(body)},
			Outputs:    outputs,
		}
	}
	a.Steps = append(a.Steps,
		archive.StepRecord{
			StepID: "waitForExport", Node: "getExport", DurationMs: 6,
			Inputs:  map[string]any{"exportId": "exp_0001"},
			Request: last.Request, Response: last.Response, Outputs: last.Outputs,
			Iterations: polls, RepeatStop: "until",
		},
		archive.StepRecord{
			StepID: "listOrders", Node: "listOrders", DurationMs: 2,
			Inputs: map[string]any{"limit": 2.0},
			Iterations: []archive.IterationRecord{
				page(1, map[string]any{"limit": 2.0}, `{"orders": [{"id": "ord_1"}, {"id": "ord_2"}], "next": "c2"}`,
					map[string]any{"orderCount": 2.0, "nextCursor": "c2"}),
				page(2, map[string]any{"limit": 2.0, "after": "c2"}, `{"orders": [{"id": "ord_3"}], "next": ""}`,
					map[string]any{"orderCount": 1.0, "nextCursor": ""}),
			},
			RepeatStop: "exhausted",
		},
	)
	return a
}

func TestRunShow_StepRequests(t *testing.T) {
	dir := t.TempDir()
	writeShowArchive(t, dir, showRunID, showRepeatArchive())

	out, _, err := runShow(t, dir, "latest", showOptions{Step: "waitForExport"})
	require.NoError(t, err)
	assert.Contains(t, out, "  3 requests (stopped: until)\n")
	assert.Regexp(t, `(?m)^requests:\n\s+#\s+STATUS\s+TIME\s+UNTIL\s+OUTPUTS$`, out)
	assert.Regexp(t, `(?m)^\s+1\s+200\s+\S+\s+rows \[0 items\], status "running"$`, out)
	assert.Regexp(t, `(?m)^\s+2\s+200\s+\S+\s+retried 1 time; rows \[0 items\], status "running"$`, out)
	assert.Regexp(t, `(?m)^\s+3\s+200\s+\S+\s+met\s+rows \[0 items\], status "complete"$`, out)
	assert.Contains(t, out, "\nNext: --iteration N (1 to 3) for one request")
	assert.NotContains(t, out, "SENT", "a step without repeat.next sends the same inputs every time")

	out, _, err = runShow(t, dir, "latest", showOptions{Step: "listOrders"})
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^\s+#\s+STATUS\s+TIME\s+UNTIL\s+SENT\s+OUTPUTS$`, out)
	assert.Regexp(t, `(?m)^\s+1\s+200\s+\S+\s+-\s+nextCursor "c2", orderCount 2$`, out)
	assert.Regexp(t, `(?m)^\s+2\s+200\s+\S+\s+after "c2"\s+nextCursor "", orderCount 1$`, out)

	out, _, err = runShow(t, dir, "latest", showOptions{Step: "checkout"})
	require.NoError(t, err)
	assert.NotContains(t, out, "requests:", "a step that didn't repeat lists no requests")
}

func TestRunShow_StepRequestsJSON(t *testing.T) {
	dir := t.TempDir()
	writeShowArchive(t, dir, showRunID, showRepeatArchive())

	out, _, err := runShow(t, dir, "latest", showOptions{Step: "listOrders", JSON: true})
	require.NoError(t, err)
	var step shownStep
	require.NoError(t, json.Unmarshal([]byte(out), &step), out)
	require.Len(t, step.Iterations, 2)
	assert.Nil(t, step.Iterations[0].Sent, "the first page sends the step's own inputs")
	assert.Equal(t, map[string]any{"after": "c2"}, step.Iterations[1].Sent)
	assert.Equal(t, map[string]any{"orderCount": 1.0, "nextCursor": ""}, step.Iterations[1].Outputs)

	out, _, err = runShow(t, dir, "latest", showOptions{Step: "waitForExport", JSON: true})
	require.NoError(t, err)
	step = shownStep{}
	require.NoError(t, json.Unmarshal([]byte(out), &step), out)
	require.Len(t, step.Iterations, 3)
	assert.True(t, step.Iterations[2].UntilMet)
	assert.Equal(t, 1, step.Iterations[1].Retries)
	assert.Equal(t, map[string]any{"status": "complete"}, step.Iterations[2].Outputs, "scalar outputs only")
	assert.Contains(t, out, `"until_met": true`)
}

func TestRunShow_Iteration(t *testing.T) {
	dir := t.TempDir()
	writeShowArchive(t, dir, showRunID, showRepeatArchive())

	out, _, err := runShow(t, dir, "latest", showOptions{Step: "waitForExport", Iteration: 2})
	require.NoError(t, err)
	assert.Regexp(t, `^step waitForExport \(node getExport\), request 2 of 3\n`, out)
	assert.Contains(t, out, "\nGET http://localhost:8765/us/v1/exports/exp_0001\n")
	assert.Regexp(t, `(?m)^status 200  \S+  retried 1 time$`, out)
	assert.Regexp(t, `(?m)^  status\s+"running"$`, out)
	assert.NotContains(t, out, "inputs:", "a step without repeat.next records no inputs per request")

	out, _, err = runShow(t, dir, "latest", showOptions{Step: "getExport", Iteration: 3})
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^status 200  \S+  until met  \(stopped: until\)$`, out)
	assert.Contains(t, out, "\noas: request not validated, response 1 violation\n  response /rows: expected array\n")
	assert.Contains(t, out, "\nNext: --iteration 3 --response --shape")

	out, _, err = runShow(t, dir, "latest", showOptions{Step: "listOrders", Iteration: 2})
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^inputs:\n  after\s+"c2"\n  limit\s+2$`, out)
	assert.Contains(t, out, "(stopped: exhausted)")

	out, _, err = runShow(t, dir, "latest", showOptions{Step: "listOrders", Iteration: 1, JSON: true})
	require.NoError(t, err)
	var view shownIteration
	require.NoError(t, json.Unmarshal([]byte(out), &view), out)
	assert.Equal(t, 1, view.Index)
	assert.Equal(t, 2, view.Requests)
	assert.Empty(t, view.RepeatStop, "only the last request says why the step stopped")
	assert.Equal(t, "c2", view.Outputs["nextCursor"])
	assert.Equal(t, 200, view.Status)
}

func TestRunShow_IterationParts(t *testing.T) {
	dir := t.TempDir()
	writeShowArchive(t, dir, showRunID, showRepeatArchive())

	tests := []struct {
		name string
		opts showOptions
		want string // the JSON printed, compared as JSON
	}{
		{name: "response", opts: showOptions{Step: "waitForExport", Iteration: 1, Part: "response"}, want: `{"status": "running", "rows": []}`},
		{name: "response path", opts: showOptions{Step: "listOrders", Iteration: 1, Part: "response", Path: "orders.#.id"}, want: `["ord_1", "ord_2"]`},
		{name: "outputs", opts: showOptions{Step: "listOrders", Iteration: 2, Part: "outputs"}, want: `{"orderCount": 1, "nextCursor": ""}`},
		{name: "inputs", opts: showOptions{Step: "listOrders", Iteration: 2, Part: "inputs"}, want: `{"limit": 2, "after": "c2"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _, err := runShow(t, dir, "latest", tt.opts)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, out)
		})
	}
}

func TestRunShow_IterationErrors(t *testing.T) {
	dir := t.TempDir()
	writeShowArchive(t, dir, showRunID, showRepeatArchive())

	tests := []struct {
		name    string
		opts    showOptions
		wantErr string
	}{
		{name: "past the last request", opts: showOptions{Step: "waitForExport", Iteration: 4},
			wantErr: "step waitForExport sent 3 requests; --iteration takes 1 to 3"},
		{name: "a step that didn't repeat", opts: showOptions{Step: "checkout", Iteration: 1},
			wantErr: "step checkout recorded no repeated requests; --iteration reads a step with a repeat block"},
		{name: "inputs without next", opts: showOptions{Step: "waitForExport", Iteration: 1, Part: "inputs"},
			wantErr: "request 1 of step waitForExport recorded no inputs of its own: without repeat.next every request sends the step's inputs, which --step waitForExport --inputs prints"},
		{name: "a path that matches nothing", opts: showOptions{Step: "waitForExport", Iteration: 1, Part: "response", Path: "nope"},
			wantErr: "--path nope matches nothing in the response body of request 1 of step waitForExport; its top-level keys are status, rows"},
		{name: "no request body", opts: showOptions{Step: "waitForExport", Iteration: 1, Part: "request"},
			wantErr: "request 1 of step waitForExport has no request body"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := runShow(t, dir, "latest", tt.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestShowOptionsFromFlags_Iteration(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    showOptions
		wantErr string
	}{
		{name: "a request's response", args: []string{"--step", "waitForExport", "--iteration", "2", "--response"},
			want: showOptions{Step: "waitForExport", Iteration: 2, Part: "response", MaxBytes: defaultShowMaxBytes}},
		{name: "a path reads the request's response", args: []string{"--step", "listOrders", "--iteration", "1", "--path", "next"},
			want: showOptions{Step: "listOrders", Iteration: 1, Part: "response", Path: "next", MaxBytes: defaultShowMaxBytes}},
		{name: "without --step", args: []string{"--iteration", "2"}, wantErr: "--iteration needs --step"},
		{name: "below 1", args: []string{"--step", "waitForExport", "--iteration", "-1"}, wantErr: "--iteration counts a step's requests from 1"},
		{name: "resolutions", args: []string{"--step", "waitForExport", "--iteration", "1", "--resolutions"},
			wantErr: "--iteration and --resolutions: a repeated step resolves its inputs once"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			addShowFlags(cmd)
			require.NoError(t, cmd.Flags().Parse(tt.args))
			opts, err := showOptionsFromFlags(cmd)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}
