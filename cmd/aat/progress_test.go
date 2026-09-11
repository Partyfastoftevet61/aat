package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/engine"
	"github.com/gburgyan/aat/graph/oas"
	"github.com/gburgyan/aat/plan"
	"github.com/gburgyan/aat/validate"
	"github.com/stretchr/testify/assert"
)

var noColorTerm = TerminalInfo{IsTTY: false, Width: 80}

func TestCLIProgressObserver_StepComplete_Success(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnStepComplete(0, 3, engine.StepResult{
		Node:       "searchFlights",
		StatusCode: 200,
		Response:   &adapter.Response{StatusCode: 200},
		Duration:   150 * time.Millisecond,
	})

	output := buf.String()
	assert.Contains(t, output, "[1/3]")
	assert.Contains(t, output, "searchFlights")
	assert.Contains(t, output, "200")
	assert.Contains(t, output, "150ms")
}

func TestCLIProgressObserver_StepComplete_Error(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnStepComplete(1, 3, engine.StepResult{
		Node:  "confirmItinerary",
		Error: assert.AnError,
	})

	output := buf.String()
	assert.Contains(t, output, "[2/3]")
	assert.Contains(t, output, "confirmItinerary")
	assert.Contains(t, output, "ERROR:")
}

func TestCLIProgressObserver_StepComplete_WithRetries(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnStepComplete(0, 1, engine.StepResult{
		Node:       "searchFlights",
		Error:      assert.AnError,
		RetryCount: 2,
		RetriedOn:  []engine.ErrorCategory{engine.CategoryNetwork, engine.CategoryNetwork},
		ErrorClass: &engine.ErrorClassification{Category: engine.CategoryNetwork},
	})

	output := buf.String()
	assert.Contains(t, output, "ERROR: "+assert.AnError.Error())
	assert.Contains(t, output, "retried 2x: network", "a failed step notes its retries like a recovered one")
}

func TestCLIProgressObserver_StepComplete_RecoveredAfterRetries(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnStepComplete(0, 1, engine.StepResult{
		Node:       "getShipment",
		StatusCode: 200,
		Response:   &adapter.Response{StatusCode: 200},
		RetryCount: 2,
		RetriedOn:  []engine.ErrorCategory{engine.CategoryTransient, engine.CategoryTransient},
	})

	output := buf.String()
	assert.Contains(t, output, "200")
	assert.Contains(t, output, "retried 2x: transient")
	assert.NotContains(t, output, "ERROR")
}

func TestCLIProgressObserver_StepComplete_DisplayOutputs(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnStepComplete(0, 1, engine.StepResult{
		Node:       "confirmItinerary",
		StatusCode: 200,
		Response:   &adapter.Response{StatusCode: 200},
		Duration:   200 * time.Millisecond,
		DisplayOutputs: []engine.DisplayOutput{
			{Label: "PNR", Name: "locator", Value: "ABCDEF"},
			{Label: "Reservation ID", Name: "reservationId", Value: "res-123"},
		},
	})

	output := buf.String()
	assert.Contains(t, output, "PNR: ABCDEF")
	assert.Contains(t, output, "Reservation ID: res-123")
}

func TestCLIProgressObserver_StepComplete_AssertionsFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnStepComplete(0, 1, engine.StepResult{
		Node:       "verify",
		StatusCode: 200,
		Response:   &adapter.Response{StatusCode: 200},
		Duration:   50 * time.Millisecond,
		Validation: &validate.MechanicalResult{Passed: false},
	})

	output := buf.String()
	assert.Contains(t, output, "ASSERTIONS FAILED")
}

func TestCLIProgressObserver_StepComplete_NoResponse(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnStepComplete(0, 1, engine.StepResult{
		Node: "brokenStep",
	})

	output := buf.String()
	assert.Contains(t, output, "(no response)")
}

func TestCLIProgressObserver_CleanupOutput(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnCleanupStart(2)
	obs.OnCleanupStepComplete(0, 2, engine.StepResult{
		Node:       "deleteBooking",
		StatusCode: 204,
		Response:   &adapter.Response{StatusCode: 204},
		Duration:   80 * time.Millisecond,
	})
	obs.OnCleanupStepComplete(1, 2, engine.StepResult{
		Node:  "deleteSession",
		Error: assert.AnError,
	})

	output := buf.String()
	assert.Contains(t, output, "cleanup:")
	assert.Contains(t, output, "deleteBooking")
	assert.Contains(t, output, "204")
	assert.Contains(t, output, "deleteSession")
	assert.Contains(t, output, "ERROR:")
}

func TestCLIProgressObserver_RunComplete_Passed(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnRunComplete(&engine.RunResult{
		Outcome: engine.OutcomePassed,
		Steps: []engine.StepResult{
			{Duration: 100 * time.Millisecond},
			{Duration: 200 * time.Millisecond},
		},
	})

	output := buf.String()
	assert.Contains(t, output, "PASSED (2/2 steps, 300ms)")
}

func TestCLIProgressObserver_RunComplete_Failed(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnRunComplete(&engine.RunResult{
		Outcome: engine.OutcomeFailed,
		Error:   assert.AnError,
		Steps:   []engine.StepResult{{Duration: 100 * time.Millisecond}},
	})

	output := buf.String()
	assert.Contains(t, output, "FAILED:")
}

func TestCLIProgressObserver_RunComplete_Error(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnRunComplete(&engine.RunResult{
		Outcome: engine.OutcomeError,
		Error:   assert.AnError,
	})

	output := buf.String()
	assert.Contains(t, output, "ERROR:")
}

func TestCLIProgressObserver_OnStepStart_NoOp(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnStepStart(0, 3, plan.Step{Node: "test"})

	assert.Empty(t, buf.String(), "OnStepStart should be a no-op for CLI")
}

func TestCLIProgressObserver_OnRunStart_CachesTotal(t *testing.T) {
	obs := &CLIProgressObserver{term: noColorTerm}
	obs.OnRunStart(5)
	assert.Equal(t, 5, obs.total)
}

func TestCLIProgressObserver_StepComplete_WithColor(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: TerminalInfo{IsTTY: true, Width: 80}}

	obs.OnStepComplete(0, 3, engine.StepResult{
		Node:       "searchFlights",
		StatusCode: 200,
		Response:   &adapter.Response{StatusCode: 200},
		Duration:   150 * time.Millisecond,
	})

	output := buf.String()
	// Should contain ANSI codes for cyan node and green status
	assert.Contains(t, output, colorCyan)
	assert.Contains(t, output, colorGreen)
	assert.Contains(t, output, colorReset)
	assert.Contains(t, output, "searchFlights")
	assert.Contains(t, output, "200")
}

func TestCLIProgressObserver_WideTerminal_NodeTruncation(t *testing.T) {
	var buf bytes.Buffer
	// Narrow terminal: node column = max(15, 50-60) = 15
	obs := &CLIProgressObserver{out: &buf, term: TerminalInfo{IsTTY: false, Width: 50}}

	obs.OnStepComplete(0, 1, engine.StepResult{
		Node:       "veryLongNodeNameThatExceedsColumn",
		StatusCode: 200,
		Response:   &adapter.Response{StatusCode: 200},
		Duration:   50 * time.Millisecond,
	})

	output := buf.String()
	assert.Contains(t, output, "~")
	assert.Contains(t, output, "200")
}

// TestCLIProgressObserver_RetryNoteIgnoresWidth checks that a failed step's
// retry note reads the same on narrow and wide terminals.
func TestCLIProgressObserver_RetryNoteIgnoresWidth(t *testing.T) {
	step := engine.StepResult{
		Node:       "searchFlights",
		Error:      assert.AnError,
		RetryCount: 2,
		RetriedOn:  []engine.ErrorCategory{engine.CategoryTimeout, engine.CategoryNetwork},
	}
	for _, width := range []int{80, 120} {
		var buf bytes.Buffer
		obs := &CLIProgressObserver{out: &buf, term: TerminalInfo{IsTTY: false, Width: width}}
		obs.OnStepComplete(0, 1, step)
		assert.Contains(t, buf.String(), "ERROR: "+assert.AnError.Error()+"  retried 2x: timeout, network\n", "width %d", width)
	}
}

func TestStepLabel(t *testing.T) {
	tests := []struct {
		name   string
		result engine.StepResult
		width  int
		want   string
	}{
		{name: "id equals node", result: engine.StepResult{StepID: "getCart", Node: "getCart"}, width: 20, want: "getCart             "},
		{name: "no id shows the node", result: engine.StepResult{Node: "deleteCart"}, width: 20, want: "deleteCart          "},
		{name: "id and node fit", result: engine.StepResult{StepID: "addProduct", Node: "addItem"}, width: 20, want: "addProduct (addItem)"},
		{name: "node does not fit", result: engine.StepResult{StepID: "checkout", Node: "checkoutCart"}, width: 20, want: "checkout            "},
		{name: "wide column fits the node", result: engine.StepResult{StepID: "checkout", Node: "checkoutCart"}, width: 30, want: "checkout (checkoutCart)       "},
		{name: "long id is truncated", result: engine.StepResult{StepID: "aVeryLongStepIdentifier", Node: "x"}, width: 15, want: "aVeryLongStepI~"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stepLabel(tt.result, tt.width, false))
			colored := stepLabel(tt.result, tt.width, true)
			assert.Equal(t, tt.want, stripANSI(colored), "colour does not change the visible text or its padding")
		})
	}
}

// TestPlanAndBatchStepLinesMatch guards against the plan and sequential batch
// renderers drifting apart again: apart from the indent, they print the same
// lines.
func TestPlanAndBatchStepLinesMatch(t *testing.T) {
	steps := []engine.StepResult{
		{StepID: "addSocks", Node: "addItem", StatusCode: 201, Response: &adapter.Response{StatusCode: 201}, Duration: 12 * time.Millisecond,
			DisplayOutputs: []engine.DisplayOutput{{Label: "Lines", Name: "lineCount", Value: 2}}},
		{StepID: "getShipment", Node: "getShipment", StatusCode: 200, Response: &adapter.Response{StatusCode: 200}, Duration: 1600 * time.Millisecond,
			RetryCount: 2, RetriedOn: []engine.ErrorCategory{engine.CategoryTransient, engine.CategoryTransient}},
		{StepID: "pay", Node: "paymentCharge", StatusCode: 404, Response: &adapter.Response{StatusCode: 404},
			Validation: &validate.MechanicalResult{Results: []validate.AssertionResult{{Type: validate.AssertStatus, Message: "expected status 201, got 404"}}}},
		{StepID: "ship", Node: "shipOrder", Error: assert.AnError},
	}
	for _, term := range []TerminalInfo{noColorTerm, {IsTTY: true, Width: 120}} {
		var planOut, batchOut bytes.Buffer
		planObs := &CLIProgressObserver{out: &planOut, term: term}
		batchObs := NewBatchStreamObserver(&batchOut, "smoke", term, 0, 1)
		for i, step := range steps {
			planObs.OnStepComplete(i, len(steps), step)
			batchObs.OnStepComplete(i, len(steps), step)
		}
		planLines := strings.Split(strings.TrimSuffix(planOut.String(), "\n"), "\n")
		batchLines := strings.Split(strings.TrimSuffix(batchOut.String(), "\n"), "\n")
		if assert.Len(t, batchLines, len(planLines)) {
			for i := range planLines {
				assert.Equal(t, "  "+planLines[i], batchLines[i])
			}
		}
	}
}

func TestCLIProgressObserver_RunComplete_UsesWallClock(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	obs.OnRunComplete(&engine.RunResult{
		Outcome:  engine.OutcomePassed,
		Duration: 2900 * time.Millisecond,
		Steps:    []engine.StepResult{{Duration: 100 * time.Millisecond}, {Duration: 200 * time.Millisecond}},
	})

	assert.Contains(t, buf.String(), "PASSED (2/2 steps, 2.9s)", "retry waits between steps count")
}

// stripANSI removes the colour escape sequences run output uses.
func stripANSI(s string) string {
	for _, code := range []string{colorReset, colorRed, colorGreen, colorYellow, colorCyan, colorBold, colorDim} {
		s = strings.ReplaceAll(s, code, "")
	}
	return s
}

// TestCLIProgressObserver_ReportsWhyAStepFailed checks that run output names
// each failed assertion and counts OpenAPI violations, on the step line and in
// a total after the outcome.
func TestCLIProgressObserver_ReportsWhyAStepFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}

	step := engine.StepResult{
		Node:       "addItem",
		StatusCode: 400,
		Response:   &adapter.Response{StatusCode: 400},
		Validation: &validate.MechanicalResult{Results: []validate.AssertionResult{
			{Type: validate.AssertStatus, Passed: false, Message: "expected status 201, got 400"},
			{Type: validate.AssertFieldExists, Passed: true, Message: `field "cartId" exists`},
			{Type: validate.AssertSchema, Passed: true, Skipped: true, Message: "schema validation unavailable"},
		}},
		OASValidation: &oas.ValidationResult{Request: &oas.PayloadResult{
			Errors: []oas.SchemaError{{Path: "$.quantity", Message: "got string, want integer"}},
		}},
	}
	obs.OnRunStart(1)
	obs.OnStepComplete(0, 1, step)
	obs.OnRunComplete(&engine.RunResult{Outcome: engine.OutcomeFailed, Steps: []engine.StepResult{step}})

	output := buf.String()
	assert.Contains(t, output, "ASSERTIONS FAILED  OAS: 1 warning(s)")
	assert.Contains(t, output, "status: expected status 201, got 400")
	assert.NotContains(t, output, "cartId", "passed and skipped assertions are not listed")
	assert.Contains(t, output, "\nOAS: 1 warning(s)\n")
}

func TestCLIProgressObserver_AbortedCountsPlannedSteps(t *testing.T) {
	var buf bytes.Buffer
	obs := &CLIProgressObserver{out: &buf, term: noColorTerm}
	obs.OnRunStart(5)
	obs.OnRunComplete(&engine.RunResult{Outcome: engine.OutcomeAborted, Steps: []engine.StepResult{{Node: "a"}, {Node: "b"}}})
	assert.Contains(t, buf.String(), "ABORTED (2/5 steps")
}
