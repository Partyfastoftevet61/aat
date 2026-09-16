package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gburgyan/aat/archive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const formRunID = "run-20260912-110000-aaaa0002"

// formTestArchive is a run whose request body is form-encoded, as an API that
// takes application/x-www-form-urlencoded receives it, archived as the one
// string it was sent as.
func formTestArchive(t *testing.T) *archive.Archive {
	t.Helper()
	body, err := json.Marshal("amount=2000&currency=usd&metadata[source]=aat-stripe&expand[]=latest_charge&name=AAT+Stripe")
	require.NoError(t, err)
	return &archive.Archive{
		Metadata: archive.ArchiveMetadata{
			Version:   "1.0.0",
			RunID:     formRunID,
			Timestamp: time.Date(2026, 9, 12, 11, 0, 0, 0, time.UTC),
		},
		Steps: []archive.StepRecord{{
			StepID: "createPayment", Node: "createPaymentIntent", DurationMs: 5,
			Request: &archive.RequestRecord{
				Method:  "POST",
				URL:     "https://api.example.com/v1/payment_intents",
				Headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
				Body:    body,
			},
			Response: &archive.ResponseRecord{
				Status:  200,
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    json.RawMessage(`{"id": "pi_1", "status": "succeeded"}`),
			},
			Outputs: map[string]any{"paymentIntentId": "pi_1"},
		}},
		Result: archive.ArchiveResult{Outcome: "passed", DurationMs: 5},
	}
}

func TestRunShow_FormRequestBody(t *testing.T) {
	dir := t.TempDir()
	writeShowArchive(t, dir, formRunID, formTestArchive(t))

	out, _, err := runShow(t, dir, "latest", showOptions{Step: "createPayment", Part: "request"})
	require.NoError(t, err)
	assert.Equal(t, "amount=2000\ncurrency=usd\nmetadata[source]=aat-stripe\nexpand[]=latest_charge\nname=AAT Stripe\n", out,
		"one decoded field per line, in the order they were sent")

	out, _, err = runShow(t, dir, "latest", showOptions{Step: "createPayment", Part: "request", Compact: true})
	require.NoError(t, err)
	assert.Equal(t, "amount=2000&currency=usd&metadata[source]=aat-stripe&expand[]=latest_charge&name=AAT+Stripe\n", out,
		"compact prints the body as it was sent")
}

func TestRunShow_FormBodyPathAndShape(t *testing.T) {
	dir := t.TempDir()
	writeShowArchive(t, dir, formRunID, formTestArchive(t))

	tests := []struct {
		name string
		opts showOptions
		want string
	}{
		{name: "a bracketed key", opts: showOptions{Part: "request", Path: "metadata.source"}, want: `"aat-stripe"`},
		{name: "a repeated key", opts: showOptions{Part: "request", Path: "expand"}, want: `["latest_charge"]`},
		{name: "a field", opts: showOptions{Part: "request", Path: "amount"}, want: `"2000"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.Step = "createPayment"
			out, _, err := runShow(t, dir, "latest", tt.opts)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, out)
		})
	}

	out, _, err := runShow(t, dir, "latest", showOptions{Step: "createPayment", Part: "request", Shape: true})
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^metadata\.source\s+string\s+"aat-stripe"$`, out)
	assert.Regexp(t, `(?m)^expand\s+array\s+1 item$`, out)
}

func TestRunShow_FormBodyInStepAndList(t *testing.T) {
	dir := t.TempDir()
	writeShowArchive(t, dir, formRunID, formTestArchive(t))

	out, _, err := runShow(t, dir, "latest", showOptions{Step: "createPayment"})
	require.NoError(t, err)
	assert.Contains(t, out, "request body: 91 bytes (form, 5 fields)\n", "the size is the body's own text, not its archived quoting")

	out, _, err = runShow(t, dir, "latest", showOptions{Step: "createPayment", JSON: true})
	require.NoError(t, err)
	var step shownStep
	require.NoError(t, json.Unmarshal([]byte(out), &step), out)
	assert.True(t, step.RequestBodyForm)
	assert.Equal(t, 5, step.RequestFormFields)

	// Without --step, a part flag prints that part of every step that has it.
	out, _, err = runShow(t, dir, "latest", showOptions{Part: "request", Path: "metadata.source"})
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^createPayment\s+createPaymentIntent\s+"aat-stripe"$`, out)
}
