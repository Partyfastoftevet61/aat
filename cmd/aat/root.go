package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/gburgyan/aat/internal/version"
	"github.com/spf13/cobra"
)

// exitError wraps an error with a specific process exit code.
type exitError struct {
	Code int
	Err  error
}

func (e *exitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ""
}

func (e *exitError) Unwrap() error { return e.Err }

var rootCmd = &cobra.Command{
	Use:   "aat",
	Short: "Adaptive API Toolkit: API workflow testing from a graph",
	Long: `Model your API as a graph once. Get long-chain integration tests, layer × environment
matrices, CI-ready runs, and an MCP server for AI coding tools — all from the same YAML.

Execution never calls an LLM. AI coding tools author plans through the MCP server;
aat prompt can draft one.`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version.Effective(), version.GitCommit, version.BuildDate),
}

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(promptCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(envCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(docsCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(webCmd)
	rootCmd.AddCommand(importCmd)
}

func main() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		var exitErr *exitError
		if errors.As(err, &exitErr) {
			if exitErr.Code != 0 && exitErr.Err != nil {
				fmt.Fprintf(os.Stderr, "aat: %s\n", exitErr.Err)
			}
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "aat: %s\n", err)
		os.Exit(1)
	}
}
