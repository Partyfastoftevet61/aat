package main

import (
	"os"

	"golang.org/x/term"
)

// TerminalInfo describes the capabilities of the output terminal.
type TerminalInfo struct {
	IsTTY  bool
	Width  int
	Height int
}

// DetectTerminal checks whether f, the file progress output goes to, is a
// terminal and queries its size. Callers pass the file they write to: with
// --dump-state -, progress goes to stderr while stdout carries the state.
// Returns sensible defaults (80x24, non-TTY) on failure.
// Honors NO_COLOR (https://no-color.org/) by forcing non-TTY mode.
func DetectTerminal(f *os.File) TerminalInfo {
	if os.Getenv("NO_COLOR") != "" {
		return TerminalInfo{IsTTY: false, Width: 80, Height: 24}
	}
	fd := int(f.Fd())
	if !term.IsTerminal(fd) {
		return TerminalInfo{IsTTY: false, Width: 80, Height: 24}
	}
	w, h, err := term.GetSize(fd)
	if err != nil {
		return TerminalInfo{IsTTY: true, Width: 80, Height: 24}
	}
	return TerminalInfo{IsTTY: true, Width: w, Height: h}
}
