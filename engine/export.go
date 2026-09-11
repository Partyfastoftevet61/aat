package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// StateExport is a self-contained snapshot of the live state accumulated by a
// run, intended for consumption by an external test harness.
//
// SECURITY: Auth headers are deliberately NOT redacted — the whole point is to
// let an external harness replay calls against the same live session. Files
// written via WriteStateExport use mode 0600 for this reason. Do not commit or
// share these files.
type StateExport struct {
	Version   string         `json:"version"`
	Outcome   string         `json:"outcome"`
	StoppedAt string         `json:"stoppedAt,omitempty"`
	BaseURL   string         `json:"baseUrl"`
	Auth      StateAuth      `json:"auth"`
	Steps     []StateStep    `json:"steps"`
	Values    map[string]any `json:"values"`
}

// StateAuth holds the live request headers (including auth tokens) of the
// environment's default route, UNREDACTED.
type StateAuth struct {
	Headers map[string]string `json:"headers"`
}

// StateStep captures one executed step: where its request went, the live
// headers it sent (UNREDACTED), its outputs, and its resolved inputs.
type StateStep struct {
	StepID  string            `json:"stepId"`
	Node    string            `json:"node"`
	BaseURL string            `json:"baseUrl,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Outputs map[string]any    `json:"outputs,omitempty"`
	Inputs  map[string]any    `json:"inputs,omitempty"`
}

// BuildStateExport assembles a StateExport from a completed (or checkpointed)
// run. It includes every successfully executed step's outputs and resolved
// inputs, flattens outputs into a "stepID.outputName" → value convenience map,
// and records each step's base URL and live (unredacted) request headers.
//
// The top-level baseUrl and auth describe the environment's default route: they
// come from the last request sent to defaultBaseURL, so a plan whose last step
// was routed to another host (such as a payments API with its own API key)
// still exports the main session. When no request went to defaultBaseURL, they
// come from the last request issued.
func BuildStateExport(result *RunResult, defaultBaseURL string) *StateExport {
	exp := &StateExport{
		Version:   "1",
		Outcome:   result.Outcome.String(),
		StoppedAt: result.StoppedAt,
		Auth:      StateAuth{Headers: map[string]string{}},
		Steps:     []StateStep{},
		Values:    map[string]any{},
	}

	var last, lastDefault *StateStep
	for _, s := range result.Steps {
		if s.Error != nil {
			continue
		}
		step := StateStep{
			StepID:  s.StepID,
			Node:    s.Node,
			Outputs: s.Outputs,
			Inputs:  s.Inputs,
		}
		if s.Request != nil {
			step.BaseURL = s.ActualBaseURL
			step.Headers = make(map[string]string, len(s.Request.Headers))
			for k, v := range s.Request.Headers {
				step.Headers[k] = v
			}
		}
		exp.Steps = append(exp.Steps, step)
		for name, val := range s.Outputs {
			exp.Values[s.StepID+"."+name] = val
		}
	}

	for i := range exp.Steps {
		if exp.Steps[i].Headers == nil {
			continue
		}
		last = &exp.Steps[i]
		if defaultBaseURL != "" && exp.Steps[i].BaseURL == defaultBaseURL {
			lastDefault = &exp.Steps[i]
		}
	}
	session := lastDefault
	if session == nil {
		session = last
	}
	if session != nil {
		exp.BaseURL = session.BaseURL
		for k, v := range session.Headers {
			exp.Auth.Headers[k] = v
		}
	}

	return exp
}

// WriteStateExport writes the export as indented JSON to path with mode 0600.
// The restrictive mode reflects that the file contains plaintext credentials.
// The file is written to a temporary file in the same directory and renamed
// into place, so an existing file at path also ends up with mode 0600.
func WriteStateExport(exp *StateExport, path string) error {
	data, err := json.MarshalIndent(exp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state export: %w", err)
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating state export dir: %w", err)
		}
	}
	tmp, err := os.CreateTemp(dir, ".state-*.json")
	if err != nil {
		return fmt.Errorf("writing state export: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing state export: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing state export: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing state export: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("writing state export: %w", err)
	}
	return nil
}
