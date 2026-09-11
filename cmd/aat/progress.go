package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gburgyan/aat/engine"
	"github.com/gburgyan/aat/plan"
)

// CLIProgressObserver implements engine.ProgressObserver for interactive CLI output.
// It prints each step result as it completes.
type CLIProgressObserver struct {
	out   io.Writer
	term  TerminalInfo
	total int // cached from OnRunStart for duration calculation
}

func (o *CLIProgressObserver) OnRunStart(total int) {
	o.total = total
}

func (o *CLIProgressObserver) OnStepStart(index, total int, step plan.Step) {
	// No-op for CLI v1. Exists for future WebSocket consumers.
}

func (o *CLIProgressObserver) OnStepComplete(index, total int, result engine.StepResult) {
	totalStr := fmt.Sprintf("%d", total)
	width := len(totalStr)
	color := o.term.IsTTY

	// Dynamic node column width (overhead=60 gives ncw=20 at 80-col terminal)
	ncw := nodeColWidth(o.term.Width, 60)
	nodeStr := formatNodeCol(result.Node, ncw, color)

	// indent = "  [" (3) + width + "/" (1) + len(totalStr) + "] " (2)
	indent := 6 + width + len(totalStr)
	prefix := fmt.Sprintf("  [%*d/%s] %s", width, index+1, totalStr, nodeStr)

	if result.Error != nil {
		errLabel := colorize("ERROR", colorRed, color)
		if result.RetryCount > 0 {
			cat := errorCategory(result)
			if o.term.Width > 100 && result.StatusCode > 0 {
				_, _ = fmt.Fprintf(o.out, "%s %s [%s] (retried %dx, last status %d)\n", prefix, errLabel, cat, result.RetryCount, result.StatusCode)
			} else {
				_, _ = fmt.Fprintf(o.out, "%s %s [%s] (after %d retries)\n", prefix, errLabel, cat, result.RetryCount)
			}
		} else {
			_, _ = fmt.Fprintf(o.out, "%s %s: %s\n", prefix, errLabel, result.Error)
		}
	} else if result.Response != nil {
		status := colorStatus(result.StatusCode, color)
		durStr := fmt.Sprintf("%dms", result.Duration.Milliseconds())
		if color {
			durStr = colorDim + durStr + colorReset
		}
		_, _ = fmt.Fprintf(o.out, "%s %s  %s%s\n", prefix, status, durStr, stepMarks(result, color))
		for _, do := range result.DisplayOutputs {
			_, _ = fmt.Fprintf(o.out, "%*s%s: %v\n", indent, "", do.Label, do.Value)
		}
		for _, msg := range failedAssertions(result.Validation) {
			_, _ = fmt.Fprintf(o.out, "%*s%s\n", indent, "", colorize(msg, colorYellow, color))
		}
	} else {
		_, _ = fmt.Fprintf(o.out, "%s (no response)\n", prefix)
	}
}

func (o *CLIProgressObserver) OnCleanupStart(total int) {
	if o.term.IsTTY {
		_, _ = fmt.Fprintf(o.out, "\n  %scleanup:%s\n", colorDim, colorReset)
	} else {
		_, _ = fmt.Fprintln(o.out, "\n  cleanup:")
	}
}

func (o *CLIProgressObserver) OnCleanupStepComplete(index, total int, result engine.StepResult) {
	color := o.term.IsTTY
	ncw := nodeColWidth(o.term.Width, 58)
	node := fmt.Sprintf("%-*s", ncw, truncateNode(result.Node, ncw))
	prefix := fmt.Sprintf("    %s", node)

	if result.Error != nil {
		errLabel := colorize("ERROR", colorRed, color)
		_, _ = fmt.Fprintf(o.out, "%s %s: %s\n", prefix, errLabel, result.Error)
	} else if result.Response != nil {
		status := colorStatus(result.StatusCode, color)
		durStr := fmt.Sprintf("%dms", result.Duration.Milliseconds())
		if color {
			durStr = colorDim + durStr + colorReset
		}
		_, _ = fmt.Fprintf(o.out, "%s %s  %s\n", prefix, status, durStr)
	} else {
		_, _ = fmt.Fprintf(o.out, "%s (no response)\n", prefix)
	}
}

func (o *CLIProgressObserver) OnRunComplete(result *engine.RunResult) {
	color := o.term.IsTTY
	_, _ = fmt.Fprintln(o.out)
	total := len(result.Steps)
	planned := max(o.total, total) // steps the run meant to execute, for ABORTED and STOPPED
	switch result.Outcome {
	case engine.OutcomePassed:
		_, _ = fmt.Fprintf(o.out, "%s (%d/%d steps, %s)\n", colorOutcome("PASSED", color), total, total, observerTotalDuration(result))
	case engine.OutcomeFailed:
		_, _ = fmt.Fprintf(o.out, "%s: %s\n", colorOutcome("FAILED", color), outcomeMessage(result))
	case engine.OutcomeError:
		_, _ = fmt.Fprintf(o.out, "%s: %s\n", colorOutcome("ERROR", color), outcomeMessage(result))
	case engine.OutcomeAborted:
		_, _ = fmt.Fprintf(o.out, "%s (%d/%d steps, %s)\n", colorOutcome("ABORTED", color), total, planned, observerTotalDuration(result))
	case engine.OutcomeStopped:
		_, _ = fmt.Fprintf(o.out, "%s at %q (%d/%d steps, %s)\n", colorOutcome("STOPPED", color), result.StoppedAt, total, planned, observerTotalDuration(result))
	}
	if n := oasWarningCount(result.Steps); n > 0 {
		_, _ = fmt.Fprintf(o.out, "OAS: %s\n", colorize(fmt.Sprintf("%d warning(s)", n), colorYellow, color))
	}
}

// stepMarks renders the notes after a step's status and duration: retries,
// failed assertions, and OpenAPI violations.
func stepMarks(result engine.StepResult, color bool) string {
	marks := ""
	if note := retryNote(result); note != "" {
		marks += "  " + colorize(note, colorYellow, color)
	}
	if result.Validation != nil && !result.Validation.Passed {
		marks += "  " + colorize("ASSERTIONS FAILED", colorYellow, color)
	}
	if result.OASValidation != nil && result.OASValidation.HasErrors() {
		marks += "  " + colorize(fmt.Sprintf("OAS: %d warning(s)", result.OASValidation.ErrorCount()), colorYellow, color)
	}
	return marks
}

// oasWarningCount totals the OpenAPI violations across steps.
func oasWarningCount(steps []engine.StepResult) int {
	n := 0
	for _, step := range steps {
		if step.OASValidation != nil {
			n += step.OASValidation.ErrorCount()
		}
	}
	return n
}

// OnRetryStart implements RetryNotifier for plan-level retries.
func (o *CLIProgressObserver) OnRetryStart(attempt, maxAttempts int) {
	color := o.term.IsTTY
	label := fmt.Sprintf("retry %d/%d", attempt, maxAttempts)
	if color {
		label = colorYellow + label + colorReset
	}
	_, _ = fmt.Fprintf(o.out, "\n%s\n", label)
}

// retryNote summarizes the retries behind a step that got a response, such as
// "retried 2x: transient". It is empty when the step did not retry.
func retryNote(result engine.StepResult) string {
	if result.RetryCount == 0 {
		return ""
	}
	var categories []string
	seen := make(map[string]bool)
	for _, c := range result.RetriedOn {
		if name := c.String(); !seen[name] {
			seen[name] = true
			categories = append(categories, name)
		}
	}
	note := fmt.Sprintf("retried %dx", result.RetryCount)
	if len(categories) > 0 {
		note += ": " + strings.Join(categories, ", ")
	}
	return note
}

// observerTotalDuration sums the duration of all steps and cleanup.
func observerTotalDuration(result *engine.RunResult) string {
	var total time.Duration
	for _, s := range result.Steps {
		total += s.Duration
	}
	for _, s := range result.CleanupResults {
		total += s.Duration
	}
	if total < time.Second {
		return fmt.Sprintf("%dms", total.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", total.Seconds())
}
