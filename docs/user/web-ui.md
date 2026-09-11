# Web UI and Archives

Every AAT run produces a JSON archive that captures the full execution trace — requests, responses, timing, value resolutions, selections, and assertions. The web UI provides a visual interface for browsing and debugging these archives.

## Archives

### What's in an Archive

| Content | Description |
|---------|-------------|
| Metadata | Plan name, graph version, start/end timestamps, outcome, layers applied |
| Steps | Per-step records: HTTP method, URL, request headers/body, response headers/body, status code, duration |
| Value resolutions | How each input was resolved — source (literal, reference, pool, expression), value used, constraint pass/fail |
| Selection decisions | Array element selection: source size, filter applied, strategy used, selected index, LLM call details |
| Assertions | Per-step assertion results: type, path, expected value, actual value, pass/fail |
| Expect-failure records | Negative test outcomes: expected status codes, actual status, pass/fail |
| Extractions | Output values extracted from responses via gjson paths |
| Cleanup | Cleanup step results with the same detail as main steps |

### Directory Structure

**Single run:**

```
_output/runs/
  run-20260223-143052-a1b2c3d4/
    archive.json
```

**Batch run:**

```
_output/runs/
  batch-20260223-150000-e5f6g7h8/
    batch.json
    run-20260223-150001-i9j0k1l2/
      archive.json
    run-20260223-150003-m3n4o5p6/
      archive.json
```

**Retries:**

When a plan is retried (via `--retries`), failed attempts are preserved alongside the final result:

```
run-20260223-143052-a1b2c3d4/
  attempt-01.json
  attempt-02.json
  archive.json        # final attempt
```

### Header Redaction

Sensitive headers (`Authorization`, API keys, bearer tokens) are automatically redacted in archives. The header name is preserved but the value is replaced with `[REDACTED]`. Archives are safe to store as CI artifacts, share with teammates, or commit to repositories.

### Export and Import

A run or batch can travel as a single file. Every run has an **Export** button (in the run list and on the run detail header) that downloads a `.aar` file — a zip of `archive.json` plus any `attempt-NN.json` retry files. Batches export as `.aab`, which bundles `batch.json` and every member run. Both keep the redacted headers, so they are safe to attach to a ticket or a pull request.

To bring a file back in:

- **Web UI** — the **Import** button on the run list accepts `.aar` and `.aab` files and adds them to the served archive directory.
- **CLI** — `aat import file.aar` (or `.aab`) extracts into the project's archive directory. The directory name is derived from the filename; pass `--name` to choose one, or `--output DIR` to import somewhere else.
- **API** — `POST /api/import` with a multipart form whose `file` field is the archive (100 MB limit). The response carries the new `ref`, its display `name`, and `type` (`run` or `batch`).
- **Just look at it** — `aat web view file.aar` serves the file from memory without importing it (see [Viewing Runs](#viewing-runs)).

An import refuses to overwrite: if a directory with the target name already exists, it fails rather than merging.

### Naming and Saving Runs

Auto-generated directories are named `run-<timestamp>-<id>` and are fair game for `aat run clean`. From the run or batch detail header you can:

- **Save** — keeps the run under its original ID, but prefixes the directory with `!` so it is treated as named.
- **Rename** (the pencil icon) — renames the directory to the name you type (a name that itself starts with `run-` or `batch-` gets the `!` prefix so it is still recognizable as saved).
- **Unname** — restores the original auto-generated directory name.

Anything that no longer matches the `run-`/`batch-` prefix is *named*, shows under the **Saved** filter on the landing page, and is never deleted by `aat run clean`. The equivalent REST calls are `PUT`/`DELETE` on `/api/runs/{id}/name` and `/api/batches/{id}/name`; the response returns the new `ref` to use in URLs. Naming is only available when serving a directory — a file opened with `aat web view file.aar` is read-only.

## Starting the Web UI

```
aat web
```

Opens a web server on `http://localhost:9119` serving the archive viewer.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--host` | string | `127.0.0.1` | Interface to bind; `0.0.0.0` accepts connections from other machines or containers. The `AAT_HOST` environment variable sets it when the flag is absent |
| `--port` | int | `9119` | Listen port |
| `--open` | bool | `false` | Open browser automatically after starting |
| `--dev` | bool | `false` | Development mode (request logging) |
| `--manifest` | path | auto-discovered | Explicit path to `aat-project.yaml` |
| `--output` | path | `_output/runs` | Archive directory to serve |

The server runs until interrupted (Ctrl-C).

By default the server listens on loopback only: run archives, and the rename and import routes, are reachable from this machine alone. `--host 0.0.0.0` makes them reachable by anyone on your network, so use it only where that is acceptable (see `SECURITY.md`). The Docker image sets `AAT_HOST=0.0.0.0`, because inside a container loopback cannot be reached through a published port.

The web UI is compiled into the binary at build time. A build without the frontend bundle — a plain `go install github.com/gburgyan/aat/cmd/aat@latest`, or `go build` without running `make frontend` first — has every CLI, MCP, and CI feature, but `aat web` exits with code `2` and a hint to install a release build (or `brew install gburgyan/tap/aat`, or `make build`). `aat web --dev` is exempt because it proxies to the Vite dev server instead.

## Viewing Runs

```
aat web view [ref]
```

Opens a specific run in the browser. If a server is already running on the configured port, AAT opens the URL directly. If not, it starts an ephemeral server that serves until you interrupt it.

Without a `ref` argument, opens the run list. `latest` opens the newest run or batch.

```
# Open the newest run or batch
aat web view latest

# Open a specific run
aat web view run-20260223-143052-a1b2c3d4

# Open the run list
aat web view

# Open a batch
aat web view batch-20260223-150000-e5f6g7h8
```

AAT auto-detects whether a reference is a single run or a batch by checking for a `batch.json` file.

### Viewing a File

`ref` can also be a file. An exported `.aar`/`.aab`, or a bare `archive.json`/`batch.json` from a CI artifact, is loaded straight into memory and served by an ephemeral server — no project, manifest, or archive directory needed, and nothing is written to disk:

```
aat web view exported-run.aar
aat web view downloaded-artifacts/run-20260223-143105-b2c3d4e5/archive.json
aat web view nightly.aab
```

For a run directory, sibling `attempt-NN.json` files are picked up so the attempt selector works; for a batch directory, member run subdirectories are loaded too. Rename and import are disabled in this mode.

## Viewing Traces

```
aat web viewtrace [id]
```

Opens the trace viewer for a specific planning trace, or lists all traces if no ID is given. Traces are produced by `aat prompt --trace`.

```
# Open a specific trace
aat web viewtrace trace-20260223-141000-f1g2h3i4

# Browse all traces
aat web viewtrace
```

See [LLM-Assisted Planning: Debugging with Traces](prompt.md#debugging-with-traces) for what traces contain.

## Web UI Features

### Run and Batch List

The landing page shows all runs and batches in reverse chronological order. Each entry shows the run ID (or its saved name), timestamp, outcome badge, duration, and step count. Batches show aggregate counts. The **All** / **Saved** filter narrows the list to named runs, and the **Import** button accepts `.aar`/`.aab` files.

Entries with recorded issues carry an **N issues** badge; hover it to see the count broken down by category. Issues are counted per category in the archive summary (`issues` in `summary.json` and in `--json` output) — today the category is `oas`, the number of OpenAPI request/response violations found by [OAS validation](running.md#oas-validation). Batch detail shows the same badge for the batch as a whole and for each member run.

### Run Detail

Clicking a run opens the detail view:

- **Step timeline** — a Gantt chart showing step durations and execution order, making parallelism and bottlenecks visible at a glance
- **Metadata** — plan name, outcome, timing, layers applied, graph version
- **Attempt selector** — when a run has retries, switch between attempt archives to compare what changed

### Step Detail

Clicking a step in the timeline opens the step detail with tabbed panels:

| Tab | Contents |
|-----|----------|
| Request | HTTP method, URL, headers, request body (formatted JSON), and a **Copy as cURL** button |
| Response | Status code, response headers, response body (formatted JSON with expand/collapse) |
| Assertions | Per-assertion results: type, path, expected vs actual, pass/fail |
| Resolutions | Per-input value resolution chain: source, strategy, value, constraint satisfaction |
| Selections | Array element selection details: source array size, filter, strategy, selected index |
| Extractions | Output values extracted from the response |
| Error | Error classification, message, and details (when step failed) |

**Copy as cURL** builds a `curl -X <method> '<url>' -H ... --data '...'` command from the request exactly as the archive recorded it and copies it to the clipboard, so a failing call can be replayed from a terminal or pasted into a bug report. Because archives redact sensitive headers, an `Authorization` header comes through as `[REDACTED]` — substitute a live token before running it. If the step was routed by an override, the URL is the one actually called; the original is shown beneath it.

### Visualizer Tabs

When [visualizer plugins](visualizers.md) are configured, matching steps show additional tabs in the step detail view. Each tab renders the response data through a custom HTML visualizer in a sandboxed iframe. Visualizers are matched by response body content or node name — see the [visualizers documentation](visualizers.md) for how matching works.

### Batch Detail

Batch detail shows an aggregate view with outcome counts, total duration, an issues badge, and the per-plan results. Click any plan to drill down to its individual run detail.

When the batch was run with `--layer-group`, a **By Layers** / **By Test** toggle switches between two layouts of the same permutation matrix (the choice is remembered per browser):

- **By Layers** groups runs by permutation — one section per layer combination, listing each plan's outcome, duration, and any extra `--layer` values applied on top.
- **By Test** pivots the data into a table with one row per plan and one column per permutation, plus an **Overall** column, so you can scan a single test across every configuration. Above the table, one drop-down per layer group filters the columns: **All**, **(none)** (permutations where that dimension is unset), or a specific layer value. A **Clear filters** button and a `N of M permutations` counter appear whenever a filter is active.

Duplicate permutations skipped by [dedup](batch-layers.md#duplicate-detection) are shown as skipped with a pointer to the canonical run; a **hide skipped** toggle removes them from both views. See [Matrix Testing: Reading the matrix in the web UI](batch-layers.md#reading-the-matrix-in-the-web-ui).

### Trace Viewer

The trace viewer shows the LLM planning pipeline step by step:

- Workflow selection call — prompts sent, raw response, token counts, timing
- Skeleton composition — the composed plan scaffold
- Value fill call — prompts, response, tokens, timing
- Post-processing snapshots
- Validation results or errors

## Debugging Patterns

### Failed Steps

Start with the **Response** tab to see the status code and response body. Common patterns:

- `400` — bad request, check the **Request** tab for malformed input
- `401`/`403` — authentication issue, check your environment config
- `404` — resource not found, check the **Resolutions** tab for incorrect references
- `500` — server error, the response body usually contains diagnostic details

### Value Resolution Issues

Open the **Resolutions** tab to see how each input was resolved:

- **Source** — where the value came from (literal, from-reference, pool, expression, LLM)
- **Value** — the resolved value
- **Constraint** — whether constraints were satisfied, relaxed, or failed

If a value looks wrong, trace it back through its source. A `from` reference points to an earlier step's output — check that step's **Extractions** tab to see what was actually extracted.

### Selection Problems

The **Selections** tab shows:

- **Source array size** — how many elements were available
- **Filter** — what predicate was applied (and how many elements passed)
- **Strategy** — which selection strategy was used (first, min, match, etc.)
- **Selected index** — which element was picked

Common issues: an empty source array (the search returned no results), a filter that eliminates all elements, or a sort field that doesn't differentiate elements well.

### Assertion Failures

The **Assertions** tab shows each assertion with its expected and actual values. For predicate assertions, the full expression and evaluation result are shown. Compare expected vs actual to understand what diverged.

## API Routes

The web server exposes a REST API that you can use programmatically.

| Method | Route | Description |
|--------|-------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/api/runs` | List all runs |
| `GET` | `/api/runs/latest` | Get the latest run |
| `GET` | `/api/runs/{id}` | Get run detail |
| `PUT` | `/api/runs/{id}/name` | Rename a run |
| `DELETE` | `/api/runs/{id}/name` | Remove custom name |
| `GET` | `/api/runs/{id}/steps/{stepId}` | Get step detail |
| `GET` | `/api/runs/{id}/attempts/{attempt}` | Get a retry attempt |
| `GET` | `/api/runs/{id}/attempts/{attempt}/steps/{stepId}` | Get step from a specific attempt |
| `GET` | `/api/runs/{id}/export` | Download the run as a `.aar` zip |
| `GET` | `/api/batches` | List all batches |
| `GET` | `/api/batches/{id}` | Get batch detail |
| `PUT` | `/api/batches/{id}/name` | Rename a batch |
| `DELETE` | `/api/batches/{id}/name` | Remove custom name |
| `GET` | `/api/batches/{id}/export` | Download the batch as a `.aab` zip |
| `POST` | `/api/import` | Import a `.aar`/`.aab` (multipart `file` field, 100 MB max) |
| `GET` | `/api/traces` | List plan traces |
| `GET` | `/api/traces/{id}` | Get trace detail |
| `GET` | `/api/visualizers/{id}` | Get visualizer HTML file |

## Development Mode

For frontend development, run the Vite dev server alongside AAT's Go server:

```bash
# Terminal 1: Vite dev server with hot reload
cd server/web && npm run dev

# Terminal 2: Go server proxying to Vite
aat web --dev
```

In `--dev` mode, the Go server proxies frontend requests to Vite on port 5173 and enables request logging. The API routes (`/api/*`) are served directly by the Go server. This gives you hot reload for frontend changes while using the real API backend.

For production, `make build` compiles the Svelte frontend and embeds it into the Go binary via `//go:embed`. Release binaries are built this way; see the note under [Starting the Web UI](#starting-the-web-ui) about builds that lack the bundle.

---

*Source: `cmd/aat/web_cmd.go`, `cmd/aat/import_cmd.go`, `server/server.go`, `server/handlers.go`, `server/service.go`, `archive/transfer.go`, `archive/naming.go`, `server/web/src/routes/*.svelte`.*
