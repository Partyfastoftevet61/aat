package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	aat "github.com/gburgyan/aat"
	"github.com/gburgyan/aat/config"
	"github.com/gburgyan/aat/engine"
	"github.com/gburgyan/aat/internal/sandbox/shop"
)

// TestShopExample runs the embedded examples/shop project against the
// in-process sandbox: the checks the CI example-shop job runs with the real
// binaries (strict validation, every plan in both regions with strict OpenAPI
// validation, the layer matrix and its dedup counts, the declined-card overlay,
// a checkpoint handoff) plus the retry demo. Each subtest gets its own sandbox
// and project copy, so IDs and chaos counters are deterministic.
func TestShopExample(t *testing.T) {
	if testing.Short() {
		t.Skip("shop example end-to-end test skipped in -short mode")
	}

	t.Run("validate strict", func(t *testing.T) {
		t.Parallel()
		p := newShopProject(t)

		var out bytes.Buffer
		code := validateCommand(&validateArgs{ManifestPath: p.manifest, Strict: true, Vars: p.vars()}, &out)
		assert.Equal(t, 0, code, out.String())
	})

	for _, env := range []string{"us", "eu"} {
		t.Run("batch "+env, func(t *testing.T) {
			t.Parallel()
			p := newShopProject(t)

			res := batchCommand(context.Background(), &batchArgs{
				runArgs:  p.runArgs(t, env),
				PlanDirs: []string(p.m.PlanDirs),
			}, io.Discard)
			require.NoError(t, res.err)
			require.NotNil(t, res.summary)
			assert.Equal(t, "passed", res.summary.Outcome, failedRuns(res.summary))
			assert.Equal(t, 7, res.summary.Summary.TotalPlans)
			assert.Equal(t, 7, res.summary.Summary.PassedPlans)
		})
	}

	t.Run("layer matrix", func(t *testing.T) {
		t.Parallel()
		p := newShopProject(t)

		args := &batchArgs{runArgs: p.runArgs(t, "us"), PlanDirs: []string(p.m.PlanDirs), Parallel: 4}
		args.LayerGroups = [][]string{
			{"shipping-standard", "shipping-express"},
			{"basket-gear", "basket-apparel"},
		}
		res := batchCommand(context.Background(), args, io.Discard)
		require.NoError(t, res.err)
		require.NotNil(t, res.summary)
		stats := res.summary.Summary
		assert.Equal(t, "passed", res.summary.Outcome, failedRuns(res.summary))
		assert.Equal(t, 63, stats.TotalPlans, "7 plans x 9 permutations")
		assert.Equal(t, 36, stats.SkippedPlans, "duplicate permutations are skipped")
		assert.Equal(t, 27, stats.PassedPlans, failedRuns(res.summary))
	})

	t.Run("declined-card overlay", func(t *testing.T) {
		t.Parallel()
		p := newShopProject(t)

		args := p.runArgs(t, "us")
		args.PlanPath = p.plan("smoke")
		args.EnvOverlay = filepath.Join(p.dir, "overlays", "declined-card.yaml")
		res := runCommand(context.Background(), &args, io.Discard, TerminalInfo{})
		require.NoError(t, res.err)
		assert.Equal(t, engine.OutcomePassed, res.outcome)
		assert.Equal(t, 402, stepByNode(t, res.summary, "paymentCharge").Status)
	})

	t.Run("archive redaction", func(t *testing.T) {
		t.Parallel()
		p := newShopProject(t)

		args := p.runArgs(t, "us")
		args.PlanPath = p.plan("full-lifecycle")
		res := runCommand(context.Background(), &args, io.Discard, TerminalInfo{})
		require.NoError(t, res.err)
		require.NotEmpty(t, res.archivePath)
		data, err := os.ReadFile(res.archivePath)
		require.NoError(t, err)

		archived := string(data)
		assert.NotContains(t, archived, "pay-demo-key", "the payments API key is a secret")
		assert.NotContains(t, archived, "aat-shop-secret", "the OAuth2 client secret is a secret")
		assert.Contains(t, archived, "demo@example.com", "the short demo password does not mangle the email")
		assert.Contains(t, archived, `"Authorization": "[REDACTED]"`)
	})

	t.Run("resilience retries", func(t *testing.T) {
		t.Parallel()
		p := newShopProject(t)

		args := p.runArgs(t, "us")
		args.PlanPath = p.plan("resilience")
		res := runCommand(context.Background(), &args, io.Discard, TerminalInfo{})
		require.NoError(t, res.err)
		assert.Equal(t, engine.OutcomePassed, res.outcome)
		assert.Equal(t, 1, stepByNode(t, res.summary, "checkInventory").Retries, "one response_error retry")
		assert.Equal(t, 2, stepByNode(t, res.summary, "getShipment").Retries, "two transient retries")
		// Three backoff waits of at least 375, 375, and 750ms (500ms and 1s less
		// 25% jitter) count toward the steps and the run.
		assert.GreaterOrEqual(t, stepByNode(t, res.summary, "getShipment").DurationMs, int64(1125), "the step's duration includes its retry waits")
		assert.GreaterOrEqual(t, res.summary.Summary.DurationMs, int64(1500), "the run's duration is wall-clock time")
	})

	t.Run("checkpoint handoff", func(t *testing.T) {
		t.Parallel()
		p := newShopProject(t)

		// Stop after the payment, which runs on the payments host with its own
		// API key: the export must still carry the shop session at top level.
		args := p.runArgs(t, "us")
		args.PlanPath = p.plan("smoke")
		args.StopAfterStep = "paymentCharge"
		args.DumpStatePath = filepath.Join(t.TempDir(), "state.json")
		res := runCommand(context.Background(), &args, io.Discard, TerminalInfo{})
		require.NoError(t, res.err)
		assert.Equal(t, engine.OutcomeStopped, res.outcome)

		data, err := os.ReadFile(args.DumpStatePath)
		require.NoError(t, err)
		var state engine.StateExport
		require.NoError(t, json.Unmarshal(data, &state))
		orderID, ok := state.Values["checkout.orderId"].(string)
		require.True(t, ok, "state values: %v", state.Values)
		assert.Equal(t, p.apiURL+"/us/v1", state.BaseURL)
		assert.True(t, strings.HasPrefix(state.Auth.Headers["Authorization"], "Bearer "), "top-level auth is the shop bearer: %v", state.Auth.Headers)

		var payment *engine.StateStep
		for i := range state.Steps {
			if state.Steps[i].Node == "paymentCharge" {
				payment = &state.Steps[i]
			}
		}
		require.NotNil(t, payment, "the payment step is in the export")
		assert.Equal(t, p.payURL+"/us/v1", payment.BaseURL)
		assert.Equal(t, "pay-demo-key", payment.Headers["X-API-Key"])
		assert.Empty(t, payment.Headers["Authorization"], "the bearer token never went to the payments host")

		// Cleanup was skipped, so the dumped bearer token reads the live, paid order.
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, p.apiURL+"/us/v1/orders/"+orderID, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", state.Auth.Headers["Authorization"])
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var order struct {
			Status string `json:"status"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&order))
		assert.Equal(t, "paid", order.Status)
	})
}

// shopProject is a copy of the embedded examples/shop project and the
// in-process sandbox its runs target.
type shopProject struct {
	dir      string
	manifest string
	apiURL   string
	payURL   string
	m        *config.ProjectManifest
}

// newShopProject starts a sandbox on random ports and extracts the embedded
// example into a temporary directory. Runs reach the sandbox through --var
// overrides of the env.yaml apiHost and payHost vars (see vars).
func newShopProject(t *testing.T) *shopProject {
	t.Helper()

	srv := shop.New(shop.Options{Latency: 0, Seed: 1})
	api := httptest.NewServer(srv.APIHandler())
	t.Cleanup(api.Close)
	pay := httptest.NewServer(srv.PaymentsHandler())
	t.Cleanup(pay.Close)

	fsys, err := aat.ShopExampleFS()
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, fsys))

	manifest := filepath.Join(dir, "aat-project.yaml")
	m, err := config.LoadManifest(manifest)
	require.NoError(t, err)
	return &shopProject{dir: dir, manifest: manifest, apiURL: api.URL, payURL: pay.URL, m: m}
}

// vars points env.yaml's apiHost and payHost vars at this project's sandbox,
// as `--var apiHost=… --var payHost=…` would.
func (p *shopProject) vars() map[string]string {
	return map[string]string{
		"apiHost": strings.TrimPrefix(p.apiURL, "http://"),
		"payHost": strings.TrimPrefix(p.payURL, "http://"),
	}
}

// runArgs returns run arguments resolved from the project manifest, as `aat run`
// would, for the named environment with strict OpenAPI validation.
func (p *shopProject) runArgs(t *testing.T, env string) runArgs {
	return runArgs{
		EnvPath:         p.m.EnvPath,
		EnvName:         env,
		GraphPath:       p.m.GraphPath,
		TemplatesPath:   p.m.TemplatesPath,
		DomainPath:      p.m.DomainPath,
		LayersDir:       p.m.LayersDir,
		OutputDir:       filepath.Join(t.TempDir(), "runs"),
		Quiet:           true,
		NoAutoOverrides: true,
		OASValidateMode: "strict",
		Vars:            p.vars(),
	}
}

// plan returns the path of a plan in the project's plans directory.
func (p *shopProject) plan(name string) string {
	return filepath.Join(p.dir, "plans", name+".yaml")
}

// stepByNode returns the summary of the first step that ran node.
func stepByNode(t *testing.T, summary *RunSummary, node string) StepSummary {
	t.Helper()
	require.NotNil(t, summary)
	for _, s := range summary.Steps {
		if s.Node == node {
			return s
		}
	}
	require.Failf(t, "step not found", "no step ran node %q", node)
	return StepSummary{}
}

// failedRuns lists the batch runs that neither passed nor were skipped, for
// assertion messages.
func failedRuns(summary *BatchSummary) string {
	var lines []string
	for _, r := range summary.Runs {
		if r.Outcome != "passed" && r.Outcome != "skipped" {
			lines = append(lines, fmt.Sprintf("%s [%s]: %s %s", r.PlanName, r.Permutation, r.Outcome, r.Error))
		}
	}
	return strings.Join(lines, "\n")
}
