package main

import (
	"fmt"
	"strings"

	"github.com/gburgyan/aat/adapter"
	"github.com/gburgyan/aat/engine"
	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/internal/grpcstatus"
	"github.com/gburgyan/aat/plan"
)

// ANSI color escape codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// colorize wraps text in an ANSI escape code when enabled.
func colorize(text, code string, enabled bool) string {
	if !enabled {
		return text
	}
	return code + text + colorReset
}

// colorStatus returns a status code string colored by HTTP range.
// 2xx → green, 4xx → yellow, 5xx → red.
func colorStatus(statusCode int, enabled bool) string {
	return colorStatusNamed(statusCode, "", enabled)
}

// colorStatusNamed is colorStatus for a status that may have a name of its own.
// A gRPC step shows its code — NOT_FOUND rather than the 404 it maps to — and
// is colored by that mapping, which is what the range checks below read.
func colorStatusNamed(statusCode int, name string, enabled bool) string {
	text := fmt.Sprintf("%d", statusCode)
	if name != "" {
		text = name
	}
	if !enabled {
		return text
	}
	switch {
	case statusCode >= 200 && statusCode < 300:
		return colorGreen + text + colorReset
	case statusCode >= 400 && statusCode < 500:
		return colorYellow + text + colorReset
	case statusCode >= 500:
		return colorRed + text + colorReset
	default:
		return text
	}
}

// colorOutcome returns an outcome word colored appropriately.
// PASSED → green+bold, FAILED/ERROR → red+bold, SKIPPED → cyan.
func colorOutcome(outcome string, enabled bool) string {
	if !enabled {
		return outcome
	}
	switch outcome {
	case "PASSED":
		return colorBold + colorGreen + outcome + colorReset
	case "FAILED", "ERROR":
		return colorBold + colorRed + outcome + colorReset
	case "ABORTED":
		return colorBold + colorYellow + outcome + colorReset
	case "SKIPPED", "STOPPED":
		return colorCyan + outcome + colorReset
	default:
		return outcome
	}
}

// nodeColWidth computes the node name column width for a given terminal width.
// overhead is the number of characters reserved for non-node content on the line.
// Returns a width between 15 (floor) and 40 (cap).
func nodeColWidth(termWidth, overhead int) int {
	if termWidth == 0 {
		termWidth = 80
	}
	w := termWidth - overhead
	if w < 15 {
		return 15
	}
	if w > 40 {
		return 40
	}
	return w
}

// truncateNode truncates a name to maxLen, adding a ~ suffix if truncated.
func truncateNode(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	if maxLen <= 1 {
		return "~"
	}
	return name[:maxLen-1] + "~"
}

// formatNodeCol formats a node name padded to the given width, optionally colored cyan.
// Handles ANSI-aware padding so visual alignment is preserved.
func formatNodeCol(name string, width int, color bool) string {
	truncated := truncateNode(name, width)
	if !color {
		return fmt.Sprintf("%-*s", width, truncated)
	}
	pad := width - len(truncated)
	if pad < 0 {
		pad = 0
	}
	return colorCyan + truncated + colorReset + strings.Repeat(" ", pad)
}

// grpcCodeName returns a response's gRPC status name, and "" for an HTTP one.
func grpcCodeName(resp *adapter.Response) string {
	if resp == nil || resp.GRPC == nil {
		return ""
	}
	return resp.GRPC.Name
}

// statusCol renders a step's status padded to width, for a column that stays
// aligned when a gRPC step reports a name where an HTTP one reports three
// digits. Padding goes outside the escape codes, which have no width.
func statusCol(result engine.StepResult, width int, color bool) string {
	text := colorStatusNamed(result.StatusCode, grpcCodeName(result.Response), color)
	plain := fmt.Sprintf("%d", result.StatusCode)
	if name := grpcCodeName(result.Response); name != "" {
		plain = name
	}
	if pad := width - len(plain); pad > 0 {
		return strings.Repeat(" ", pad) + text
	}
	return text
}

// planStatusWidth measures the status column a plan needs: three digits for an
// HTTP status, and for a gRPC step the longest name the plan actually writes —
// in expectFailure, in expectStatus, or in a status assertion — floored at OK,
// which is what a gRPC step that succeeds reports. A status nothing predicted
// can still overflow the column; it is one row, where every row used to jitter.
func planStatusWidth(p *plan.Plan, g *graph.Graph, registry *adapter.Registry) int {
	const httpWidth = 3
	if p == nil || g == nil || registry == nil {
		return httpWidth
	}
	if engine.ValidateNodeProtocols(g, registry).GRPCNodes == 0 {
		return httpWidth
	}

	width := max(httpWidth, len(grpcstatus.Name(grpcstatus.OK)))
	consider := func(v any) {
		if s, ok := v.(string); ok && grpcstatus.IsName(s) {
			width = max(width, len(s))
		}
	}
	for _, step := range p.Execution.Steps {
		if step.ExpectFailure != nil {
			for _, s := range step.ExpectFailure.Status.Strings() {
				consider(s)
			}
		}
		// A mutation becomes a sibling step whose expectFailure is its
		// expectStatus, so its names reach the column too.
		for _, m := range step.Mutations {
			for _, s := range m.ExpectStatus.Strings() {
				consider(s)
			}
		}
		if step.Assertions != nil {
			for _, a := range step.Assertions.Mechanical {
				if a.Type == "status" {
					consider(a.Expect)
				}
			}
		}
	}
	return width
}
