# Running Tests

AAT has two run commands: `aat run plan` executes a single plan, and `aat run batch` executes all plans in a directory. Both produce JSON archives that capture every request, response, and assertion for later inspection.

## Running a Single Plan

```
aat run plan <name-or-path>
```

The positional argument is either a file path or a plan name. AAT resolves names by searching the plan directories declared in your [manifest](project-setup.md), then falling back to the literal file path. A `.yaml` or `.yml` extension is optional when using names.

```
$ aat run plan smoke-test
  [1/3] listProducts            200    45ms
  [2/3] createOrder             201   312ms
  [3/3] getOrderStatus          200    28ms

PASSED (3/3 steps, 385ms)
```

Each line shows the step index, node name, HTTP status code, and duration. Display outputs defined in the plan appear indented below their step.

## Checkpoints: Stopping Early and Handing Off State

Sometimes you want AAT to drive a system into a particular state and then hand off to a different, specialized test harness — for example, run a booking plan only as far as the step that creates a itinerary, then let an external script take over with that live itinerary and its auth token.

Two single-plan flags support this:

| Flag | Description |
|------|-------------|
| `--stop-after STEP` | Stop execution after the step whose ID equals `STEP` completes. Cleanup is **skipped**, so resources created up to that point stay alive for the handoff. |
| `--dump-state FILE` | Write the accumulated run state to `FILE`. Use `-` for stdout instead of a file; stdout then carries only the state and progress goes to stderr. Usable with or without `--stop-after`. |

```
aat run plan roundtrip-booking \
  --stop-after createItinerary \
  --dump-state itinerary-state.json
```

The run reports a `stopped` outcome (exit code `0`), and the regular archive is still written. The dump file is JSON:

```json
{
  "version": "1",
  "outcome": "stopped",
  "stoppedAt": "createItinerary",
  "baseUrl": "https://api.pp.example.com",
  "auth": { "headers": { "Authorization": "Bearer ..." } },
  "steps": [ { "stepId": "createItinerary", "node": "createItinerary", "outputs": { "itineraryId": "wb-42" } } ],
  "values": { "createItinerary.itineraryId": "wb-42" }
}
```

- `baseUrl` and `auth.headers` come from the last request issued, capturing the live session.
- `values` flattens every completed step's outputs as `stepId.outputName` for easy lookup.

### Dumping to stdout (no file)

Pass `-` as the path to emit the state on stdout instead of writing a file. Combined with `--json`, the state is nested under a top-level `state` key in the summary, so a wrapping harness can capture everything from a single JSON object on stdout:

```
aat run plan roundtrip-booking --stop-after createItinerary --json --dump-state -
```

```json
{
  "outcome": "stopped",
  "steps": [ ... ],
  "summary": { ... },
  "state": { "baseUrl": "...", "auth": { "headers": { ... } }, "values": { ... } }
}
```

Without `--json`, stdout carries only the state object: progress and the summary line go to stderr, so `aat run plan … --dump-state - | jq .values` works with or without `--quiet`.

> **Security:** unlike run archives, auth headers in the dump are **not redacted** — that is the point, so the external harness can replay calls. To a file it is written with mode `0600`; to stdout it lands in your terminal/pipe. Either way, treat it as a secret: do not commit, log, or share it.

`STEP` is matched against the step's ID (its `id:` if set, otherwise the node name). An unknown step name fails the run with a clear error rather than silently running to completion.

## Running Batches

```
aat run batch [directory]
```

Without arguments, AAT discovers all `.yaml` and `.yml` files in the plan directories declared in your manifest. With a directory argument, it scopes discovery to that subdirectory.

### Subdirectory Filtering

A relative path filters within the configured plan directories. An absolute path is used as a standalone directory.

```
# Run only plans under plans/orders/
aat run batch orders/

# Run plans from an absolute path
aat run batch /tmp/smoke-tests/
```

### Parallel Execution

By default, plans run sequentially. Use `--parallel` to run multiple plans concurrently.

```
aat run batch --parallel 4
```

In parallel mode, AAT displays a compact progress renderer that tracks all active plans. Sequential mode shows step-by-step output for each plan.

### Layer Expansion

Layers provide alternate test data for the same plan structure. The `--layer` flag adds a layer to every plan in the batch. The `--layer-group` flag creates a cartesian product — each plan runs once per combination of layer groups.

```
# Every plan runs with the "premium" layer applied
aat run batch --layer premium

# 2 plans x 2 groups = 4 runs
aat run batch --layer-group "premium,standard" --layer-group "us,eu"
```

With two plans (`smoke-test.yaml`, `full-checkout.yaml`) and two layer groups of two values each, AAT runs eight total executions: each plan with each combination of (`premium`/`standard`) x (`us`/`eu`).

See [Plans: Layers](plans.md#layers) for how layers are defined and how they override step values.

## Shared Flags

These flags apply to both `run plan` and `run batch`.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--manifest` | path | auto-discovered | Explicit path to `aat-project.yaml` |
| `--env-config` | path | from manifest | Environment config file |
| `--env` | string | from manifest | Environment name (for multi-environment files) |
| `--graph` | path | from manifest | API graph file |
| `--templates` | path | from manifest | Templates directory |
| `--domain` | path | from manifest | Domain knowledge file |
| `--output` | path | `_output/runs` | Archive output directory |
| `--override` | `NODE=URL` | — | Route a node to a different URL (repeatable) |
| `--overlay` | path | — | Overlay YAML with additional environment overrides |
| `--retries` | int | `0` | Max plan-level retries on failure |
| `--layer` | string | — | Data layer to apply (repeatable) |
| `--no-auto-overrides` | bool | `false` | Disable auto-discovery of `.aat-overrides.yaml` |
| `--oas-validate` | string | `auto` | OAS validation mode: `auto`, `warn`, `strict`, or `off` (see [OAS Validation](#oas-validation)) |
| `--no-mutations` | bool | `false` | Skip mutation-expanded sibling steps; run only the happy path (smoke-test mode) |
| `--verbose-auth` | bool | `false` | Log auth request/response details to stderr for debugging |
| `--json` | bool | `false` | Machine-readable JSON summary to stdout |
| `--quiet` | bool | `false` | Suppress progress, show final line only |

The `run plan` command adds:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--stop-after` | string | — | Stop after the named step ID and skip cleanup (see [Checkpoints](#checkpoints-stopping-early-and-handing-off-state)) |
| `--dump-state` | path | — | Write accumulated run state to a file (`-` for stdout, which then carries only the state); mode `0600` |

The `run batch` command adds:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--parallel` | int | `1` | Concurrency limit (1 = sequential) |
| `--layer-group` | string | — | Comma-separated layer names for permutations (repeatable) |
| `--no-dedup` | bool | `false` | Disable duplicate plan detection across permutations |
| `--shuffle` | bool | `false` | Randomize plan execution order |
| `--seed` | int | `0` | Random seed for `--shuffle` (`0` = current time; the seed used is logged) |

See [Matrix Testing: Controlling Behavior](batch-layers.md#controlling-behavior) for how dedup, shuffle, and seed interact with layer groups.

When a manifest is discoverable, `--env-config`, `--graph`, `--templates`, and `--domain` are optional. Explicit flags always override manifest paths. See [Project Setup: Auto-Discovery](project-setup.md#auto-discovery) for how manifest resolution works.

AAT also auto-discovers a `.aat-overrides.yaml` dotfile for personal, per-project routing overrides. This is especially useful for [local development](local-dev.md) — drop the file once and every run picks it up without extra flags. Use `--no-auto-overrides` to disable this for CI or clean runs.

## Output Modes

### Default (Progress)

Without flags, AAT prints step-by-step progress as each step completes.

```
$ aat run plan full-checkout
  [1/5] listProducts            200    52ms
  [2/5] createCart               201    98ms
  [3/5] addToCart                200    45ms
  [4/5] createOrder             201   287ms
  [5/5] getOrderStatus          200    31ms

  cleanup:
    cancelOrder                  200    64ms

PASSED (5/5 steps, 513ms)
```

Assertion failures append a marker to the step line. Display outputs appear indented below their step.

### Quiet (`--quiet`)

Suppresses all progress output. Prints a single summary line when the run finishes.

```
$ aat run plan smoke-test --quiet
PASSED (3/3 steps, 385ms)
```

For batches, each plan gets one summary line:

```
$ aat run batch --quiet
smoke-test          PASSED  3 steps   385ms
full-checkout       PASSED  5 steps   513ms
return-flow         FAILED  4 steps   892ms

FAILED (2/3 passed)
```

### JSON (`--json`)

Writes a machine-readable JSON summary to stdout. Implies `--quiet` — no progress output is mixed with the JSON. See [CI/CD Integration](ci-cd.md) for the full JSON schema and pipeline integration patterns.

```
$ aat run plan smoke-test --json
{"outcome":"passed","steps":[...],"summary":{"total_steps":3,...},...}
```

## Exit Codes

| Code | Meaning | When |
|------|---------|------|
| `0` | Passed / Stopped | All steps succeeded, or a `--stop-after` checkpoint was reached |
| `1` | Failed | One or more steps or assertions failed |
| `2` | Error | Infrastructure or setup error (bad config, network failure, invalid plan) |
| `130` | Aborted | The run was interrupted by Ctrl+C or `SIGTERM` (see [Interrupting a Run](#interrupting-a-run-ctrlc)) |

For batches, the exit code reflects the worst outcome across all plans: any aborted plan gives `130`, otherwise any error gives `2`, otherwise any failure gives `1`.

These codes are deterministic and designed for CI/CD pipelines. See [CI/CD Integration: Exit Codes](ci-cd.md#exit-codes) for detailed scenarios.

## Interrupting a Run (Ctrl+C)

Pressing Ctrl+C (or sending `SIGTERM`) during a run does not simply kill the process. AAT stops issuing new requests, runs cleanup for the resources created so far, and writes a partial archive:

```
$ aat run plan full-checkout
  [1/5] listProducts            200  52ms
  [2/5] createCart              201  98ms
^C
aat: interrupted, writing partial results...

  cleanup:
    deleteCart                  204  41ms

ABORTED (2/5 steps, 191ms)
```

The archive records the outcome as `aborted` with the steps that completed, and the process exits with code `130`. Cleanup for an aborted run executes under its own 30-second budget so a hung API cannot keep the process alive indefinitely. In a batch, the plan that was running is marked `aborted`; plans that had not yet started still get an entry, but each stops before issuing a request and is recorded as `aborted` too. The batch outcome is `aborted` and the process exits `130`.

## What Happens During Execution

When you run a plan, AAT performs these steps in order:

1. **Load and validate** — parse the plan YAML, validate it against the graph
2. **Authenticate** — obtain credentials using the environment's auth config
3. **Resolve and execute** — for each step in topological order: resolve input values, execute the HTTP request, extract outputs, run assertions
4. **Cleanup** — run plan-level cleanup steps, then graph-level cleanup pairings (newest resource first), even if main steps failed
5. **Archive** — write the full execution trace to the output directory

### Step Execution Order

Steps run in topological order based on `dependsOn` declarations. Steps with no dependencies run first. Steps that depend on earlier steps wait until their dependencies complete. Within a dependency level, steps run in plan declaration order.

### Value Resolution at Runtime

Each step input is resolved through a priority chain: plan-provided values, then references to earlier step outputs, then domain value pools, then graph defaults. Expressions like `{{today + 7 days}}` and environment variable references like `{{env.API_REGION}}` are evaluated at resolution time.

See [Value Resolution](value-flow.md) for the full priority chain and resolution strategies.

### Cleanup

Cleanup runs after the main steps finish — whether the plan passed, failed, errored, or was interrupted with Ctrl+C. The only time cleanup is skipped is a `--stop-after` checkpoint, where the whole point is to leave resources alive.

Two sources of cleanup work combine, in this order:

1. **Plan-level cleanup steps** (`execution.cleanup:` in the plan) run first, in declaration order. Each step's `runOn` (`always`, `success`, `failure`; empty means `always`) is checked against the outcome — `success` runs only when the plan passed, `failure` runs when it failed, errored, or was aborted.
2. **Graph-level cleanup pairings** (`cleanup: deleteX` on a node) run next from a stack: every main step whose node declares a cleanup partner pushes that partner when the step succeeds, and the stack unwinds last-in-first-out, so the most recently created resource is torn down first. A pairing whose node already ran as a plan-level cleanup step is skipped rather than run twice.

Cleanup steps do not carry `values:`. Their inputs are filled by matching input names against the outputs of earlier steps — the step that registered the cleanup is consulted first, then any other executed step. A `deleteOrder` cleanup with an `orderId` input picks up `orderId` from the `createOrder` step that created it.

Cleanup results are recorded in the archive (and in the `cleanup` array of `--json` output) with the same detail as main steps, but a failed cleanup step never changes the run outcome — the outcome is determined by the main steps alone.

See [Plans: Cleanup Steps](plans.md#cleanup-steps) and [API Graphs: Cleanup](graphs.md#cleanup) for how each kind is declared.

## Retries

The `--retries` flag sets the maximum number of plan-level retries on failure. When a plan fails and retries remain, AAT re-executes the entire plan from scratch.

```
aat run plan flaky-test --retries 2
```

Each failed attempt is saved as `attempt-01.json`, `attempt-02.json`, etc. in the run directory. The final attempt (whether it passed or not) is saved as `archive.json`. Setup errors (invalid plan, missing config) are not retried.

A short delay separates retry attempts to avoid hammering the API.

## Archives

Every execution produces a JSON archive in the output directory.

### Single Run

```
_output/runs/
  run-20260223-143052-a1b2c3d4/
    archive.json
```

### Batch

```
_output/runs/
  batch-20260223-150000-e5f6g7h8/
    batch.json
    run-20260223-150001-i9j0k1l2/
      archive.json
    run-20260223-150003-m3n4o5p6/
      archive.json
```

The batch directory contains a `batch.json` with aggregate results and a subdirectory per plan with its individual archive.

### What Archives Contain

Archives capture the full execution trace: per-step request/response pairs (method, URL, headers, body), HTTP status codes, timing, extracted outputs, value resolution decisions, selection decisions, assertion results, and error classifications. Sensitive headers (`Authorization`, API keys) are automatically redacted.

Archives are safe to store as CI artifacts or share with teammates. See [Web UI and Archives](web-ui.md) for browsing and debugging with the archive viewer.

### Pruning Old Runs (`aat run clean`)

The output directory grows by one directory per run. `aat run clean` deletes auto-generated run and batch directories older than a cutoff:

```
aat run clean                # delete runs older than 7 days
aat run clean --days 30      # keep a month
aat run clean --dry-run      # list what would be deleted, remove nothing
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--days` | int | `7` | Delete runs older than this many days |
| `--dry-run` | bool | `false` | Preview without deleting |
| `--output` | path | from manifest | Archive directory to clean (defaults to the manifest's `archives`) |

Only directories with the auto-generated `run-`/`batch-` prefix are candidates. Runs you have named or saved in the web UI (their directory no longer starts with `run-`/`batch-`, or starts with `!`) are never deleted, and the summary line reports how many were skipped. See [Web UI: Naming and Saving Runs](web-ui.md#naming-and-saving-runs).

### Rebuilding Summaries (`aat run rebuild-summaries`)

Each run directory carries a cached `summary.json` that the web UI reads for its list views (step counts, issue counts, and similar). After upgrading AAT, older summaries may lack fields the new UI expects. Rebuild them from the full archives without re-running anything:

```
aat run rebuild-summaries
```

It walks the output directory (or `--output DIR`), recomputes every summary, and reports how many were rebuilt. The next web UI listing picks them up immediately.

## OAS Validation

When the graph references an OpenAPI spec (a graph-level `oas:` or per-node `oas` references — see [API Graphs: OAS Integration](graphs.md#oas-integration)), AAT validates each step's request and response against the spec as it runs. Violations show up in three places: an `OAS: N warning(s)` marker on the step line and a total after the summary, an `issues` map (`{"oas": N}`) in the `--json` summary and archive summary, and the per-step detail in the archive.

The `--oas-validate` flag controls the mode:

| Mode | Behavior |
|------|----------|
| `auto` | Default. Validate when specs are present; report violations as warnings |
| `warn` | Same reporting as `auto` |
| `strict` | Like `auto`, but a request or response that violates the spec fails the step (outcome `failed`, cleanup still runs). Skipped validations and schema compilation warnings never fail a step; `expectFailure` steps are exempt. Use a `schema` assertion instead when only specific steps should be strict (see [Plans: Assertions](plans.md#assertions)) |
| `off` | Do not load specs or validate |

The default comes from `settings.oasValidation` in the environment file; the flag overrides it for one run. See [Environments: Runtime Settings](environments.md#runtime-settings).

## Debugging Authentication

The `--verbose-auth` flag prints the full authentication exchange to stderr, so you can see exactly what AAT sends and receives when obtaining tokens.

```
$ aat run plan smoke-test --verbose-auth
[auth] authenticating with type=oauth2
[auth] POST https://auth.example.com/oauth/token
[auth]   client_id = IeWY...
[auth]   client_secret = IzlP...
[auth]   grant_type = password
[auth]   password = RVfN...
[auth]   username = testuser
[auth] response status: 200
[auth] response body: {"access_token":"eyJhb...","token_type":"Bearer","expires_in":86400}
[auth] token type=Bearer expires_in=86400 access_token=eyJhbGci...
```

Key details:

- **Output goes to stderr** — it won't interfere with `--json` output on stdout.
- **Passwords and client secrets are truncated** to the first 4 characters for readability (you already have access to the full values in your config).
- **All auth types are covered** — OAuth2 shows the full token exchange; API key and bearer show the resolved credential values.
- **Works with overlay auth** — if an overlay file overrides auth, the verbose output reflects the effective auth being used.

This flag is available on both `run plan` and `run batch`.

## Installing and Building AAT

The easiest path is a prebuilt release binary — see the Install section of the README. The binary is self-contained: no runtime dependencies, no external files needed beyond your project's YAML configuration.

To build from source with version information and the web UI embedded:

```
make build
```

This compiles the Go binary with version, commit hash, and build date injected via linker flags, and builds the embedded web UI frontend.

For a quick build without the frontend:

```
go build -o aat ./cmd/aat/
```

A build without the frontend (including `go install github.com/gburgyan/aat/cmd/aat@latest`) runs every CLI, MCP, and CI feature, but `aat web` exits with code `2` and an install hint because there is no UI bundle to serve. Use a release build or `make build` when you want the web UI.

---

*Source: `cmd/aat/run_plan_cmd.go`, `cmd/aat/run_batch_cmd.go`, `cmd/aat/run_shared.go`, `cmd/aat/progress.go`.*
