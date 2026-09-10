# AAT — Adaptive API Toolkit

AAT is a Go CLI that models an API as a graph and executes long, multi-step test plans against it: automatic data wiring between steps, layers for matrix testing, multi-environment routing, run archives with a full decision trail, an embedded web UI, and an MCP server so AI coding tools can integrate with the API. Execution is deterministic; LLMs are optional and only draft plans (`aat prompt`).

## Module

`github.com/gburgyan/aat` — Go 1.25+

## Build System

The project uses a Makefile for builds:

```bash
make build      # Build frontend, then Go binary with version/commit/date ldflags
make frontend   # cd server/web && npm install && npm run build
make test       # go test ./...
make check      # fmt + test-race + lint — mirrors CI pipeline
make clean      # Remove binary and frontend artifacts (node_modules, dist)
```

`make build` injects `VERSION`, `COMMIT`, and `BUILD_DATE` into `internal/version` via `-ldflags`.

**Before committing or pushing**, run `make check` to catch issues that CI would flag. This runs `gofmt -s` formatting, tests with the race detector, and linting — the same checks the CI/CD pipeline performs.

## Package Structure

| Package | Responsibility |
|---------|---------------|
| `cmd/aat/` | CLI binary — thin wrapper, wires packages together |
| `graph/` | API graph model, YAML parsing, traversal, backward chaining, diffing |
| `adapter/` | Adapter interface, HTTP executor, Tier 1/3 loaders |
| `domain/` | Domain knowledge: concepts, types, value pools |
| `plan/` | Plan model, expression evaluator, validation, persistence |
| `intent/` | LLM-powered prompt → plan transformation |
| `engine/` | Execution engine: scheduling, value resolution, retry, cleanup, verification |
| `validate/` | Mechanical, semantic, and intent validation |
| `archive/` | Run archives: capture, inspection, diffing, reports |
| `llm/` | Provider-agnostic LLM client |
| `config/` | Configuration, environments, local storage |
| `server/` | Local web API server (chi), embedded Svelte SPA frontend, archive viewer |
| `mcp/` | MCP server: API lifecycle platform for IDE-based AI tools (stdio transport) |
| `internal/testutil/` | Shared test helpers and fixtures |
| `internal/version/` | Build version info |

## Dependency Rules

Dependencies flow in one direction. No cycles. No lateral imports within a tier.

**Leaf packages** (zero aat imports): `config`, `graph`, `domain`, `llm`
**Mid-tier**: `adapter` → config; `plan` → graph, config; `archive` → plan; `validate` → llm
**Orchestrators**: `engine` → graph, adapter, plan, domain, llm, validate, archive, config
**Entry points**: `intent` → graph, domain, plan, llm; `mcp` → all packages; `server` → engine, archive, plan, config
**Binaries**: `cmd/aat` → engine, server, intent, mcp, archive, config

Data flows down, decisions flow up. No business logic in `cmd/`.

## Dependency Injection Patterns

Cross-package dependencies use direct struct fields and function parameters — explicit, no framework. No global mutable state.

```go
// Production: construct with NewEngine + builder methods
router := engine.NewExecutorRouter(executor, envConfig)
eng := engine.NewEngine(g, registry, router).
    WithDomain(kb)

// Usage: collaborators are struct fields or function params
func (e *Engine) Run(ctx context.Context, p *plan.Plan) *RunResult {
    // use e.graph, e.KB, etc.
}

// Tests: construct with test implementations
func TestRun(t *testing.T) {
    router := engine.NewExecutorRouter(executor, envConfig)
    eng := engine.NewEngine(g, &fakeRegistry{}, router)
    result := eng.Run(ctx, testPlan)
    // assert
}
```

## Project Manifest

Each AAT project has an `aat-project.yaml` manifest that declares project artifacts for CLI auto-discovery. The CLI walks up from `cwd` to find it (`config.FindManifest()`), or it can be passed explicitly via `--manifest`.

```yaml
# airline/aat-project.yaml
name: airline
description: Airline JSON API integration testing
tags: [travel, api, booking]
graph: graph.yaml
templates: templates/
domain: domain.yaml
workflows: workflows/
plans: plans/
archives: runs/
environment: env.yaml
defaultEnvironment: pp
```

Key type: `config.ProjectManifest`. Fields: `Name` (required), `GraphPath` (required), `TemplatesPath` (required), `DomainPath`, `WorkflowsDir`, `PlanDirs`, `ArchiveDir`, `TracesDir`, `EnvPath`, `DefaultEnvironment`.

Used by: `aat validate`, `aat web`, `aat mcp serve`, `aat plan list`, `aat env list`, and as defaults for `aat run`/`aat prompt` when explicit flags are omitted.

### Multi-Environment Files

The environment file supports two formats: **single-environment** (legacy, one `apiBaseUrl` per file) and **multi-environment** (multiple named environments with shared config, inheritance via `extends`, variable substitution via `vars`, and file splitting via `include`). Format is auto-detected by presence of the `environments` top-level key. Select an environment with `--env` flag, `AAT_ENV_NAME` env var, or `defaultEnvironment` in the manifest. Key types: `config.MultiEnvironmentFile`, `config.EnvironmentPartial`. Key functions: `config.LoadNamedEnvironment(path, envName)`, `config.ListEnvironments(path)`.

## Plans vs Workflows

- **Workflows** (`workflows/` dir) — pre-written reusable templates defined in the graph YAML. Composed at runtime via `ComposeWithAddons`. Referenced by name in `WorkflowSelection`.
- **Plans** (`plans/` dir) — user-generated execution instances, typically saved from `aat prompt --save`. These are concrete, ready-to-run YAML files.

## Testing Philosophy

- Each package has its own `_test.go` files; tests run with `go test ./...`
- Use `github.com/stretchr/testify` — `assert` for non-fatal checks, `require` for fatal preconditions
- Tests construct types with test implementations directly (no DI framework)
- Table-driven tests preferred for systematic coverage
- Closed-loop: tests verify observable behavior, not internal state
- `internal/testutil/` for shared helpers only — no test logic there

## Go Conventions

- `context.Context` as first parameter to functions that do I/O or cross package boundaries
- Errors as values; wrap with `fmt.Errorf("...: %w", err)` for context
- No `init()` functions; no global mutable state
- Exported types and functions get doc comments
- Use `errors.Is` / `errors.As` for error inspection
- Prefer returning concrete types; accept interfaces

## Documentation

- **`docs/internal/`** — progress tracker, architecture notes (for contributors)
- **`docs/worklog/`** — decision log entries per stage (date, decisions, rationale)
- **`docs/user/`** — user-facing docs (created when there's something to document)

Record user-visible changes in `CHANGELOG.md` under *Unreleased* as tasks complete. Add worklog entries for non-trivial decisions. The public launch roadmap lives in `LAUNCH-PLAN.md`.

## Worklogs

When making design decisions or completing stage milestones, add entries to `docs/worklog/stage-N.md`. Format:

```
## YYYY-MM-DD — Summary

**What:** What was done
**Decisions:** Key choices and their rationale
**Open questions:** Anything deferred
```

## Running AAT

Build the binary and run one of the example projects under `examples/`:

```bash
# Build (injects version/commit/date automatically)
make build

# Or without make (no web UI bundle; `aat web` will refuse to start):
# go build -o aat ./cmd/aat/

# Petstore example (public API, no credentials)
cd examples/petstore/
../../aat run plan plans/create-and-verify.yaml
../../aat run batch
../../aat web view

# TODO(M1): replace with the shop sandbox quick start (aat-sandbox init/serve).

# Explicit paths instead of manifest auto-discovery:
./aat run plan examples/petstore/plans/create-and-verify.yaml \
  --env-config examples/petstore/env.yaml \
  --graph examples/petstore/graph.yaml \
  --templates examples/petstore/templates/

# Optional prompt flags:
#   --yes              skip interactive confirmation (auto-execute)
#   --save FILE        save generated plan to a YAML file
#   --trace            capture planning pipeline trace for debugging
#   --trace-dir DIR    trace output directory (default: traces/)
#   --output DIR       archive output directory (default: _output/runs/)

# Shared run flags (apply to both plan and batch):
#   --output DIR       archive output directory (default: _output/runs/)
#   --env NAME         environment name (for multi-environment files)
#   --env-config FILE  environment file (overrides the manifest)
#   --json             machine-readable JSON summary to stdout
#   --quiet            suppress progress, show final line only
#   --override NODE=URL  route a node to a different URL (repeatable)
#   --overlay FILE       path to overlay YAML with additional overrides
#   --no-auto-overrides  disable auto-discovery of .aat-overrides.yaml
#   --retries N        max plan-level retries on failure (0 = no retries)
#   --oas-validate MODE  runtime OpenAPI validation: auto|warn|strict|off
#   --stop-after STEP  stop after a step, skip cleanup, keep resources alive
#   --dump-state FILE  write live state (base URL, auth headers, outputs) for external harnesses
```

The author's production-grade project (a 74-node airline API graph) lives in a separate private
repository; it is cited by numbers only in public docs. LLM config (endpoint, API key, model) comes
from the `llm:` section in the env YAML. The API key resolves from an OS environment variable via
`SecretRef`.

## Observability & Debugging

AAT has two layers of observability: **run archives** capture execution, **plan traces** capture planning.

### Run Archives (`archive/`)

Every execution writes a JSON archive to the output directory (default `runs/`). Archives contain per-step request/response pairs, timing, status codes, and overall outcome. Sensitive headers are redacted.

```bash
aat run plan plan.yaml --output runs/
# produces: runs/run-YYYYMMDD-HHMMSS-XXXXXXXX/archive.json

aat run batch --output runs/
# produces: runs/batch-YYYYMMDD-HHMMSS-XXXXXXXX/batch.json
#           runs/batch-.../run-YYYYMMDD-HHMMSS-XXXXXXXX/archive.json (per plan)
```

Key types: `archive.Archive`, `archive.Write`, `archive.Read`, `archive.GenerateRunID`, `archive.BatchArchive`, `archive.WriteBatch`, `archive.ReadBatch`, `archive.GenerateBatchID`.

### Plan Traces (`intent/`)

When `--trace` is passed to `aat prompt`, the `intent.Interpret()` pipeline captures every intermediate step as a JSON trace file. This is the primary tool for debugging LLM prompt engineering and plan generation issues.

```bash
aat prompt --trace --trace-dir traces/ "book a flight"
# produces: traces/trace-YYYYMMDD-HHMMSS-XXXXXXXX/plan-trace.json
```

The trace captures:
- **Workflow selection call**: full system/user prompts, raw LLM response, token counts, timing (selects workflow + addons)
- **Skeleton**: the composed plan scaffold + YAML sent to the LLM, unfed inputs list
- **Plan call (value fill)**: full prompts, raw LLM response, token counts, timing
- **Merge/post-process**: snapshots of the plan after merge and after post-processing
- **Validation**: any validation errors
- **Partial traces on error**: if the pipeline fails mid-way, whatever was captured so far is still written

Opt-in via `InterpretRequest.EnableTrace = true`. Zero overhead when disabled. Key types: `intent.PlanTrace`, `intent.WritePlanTrace`.

## CLI Commands

Beyond `prompt` (shown above), the CLI provides:

```bash
# Execute plans
aat run plan <name-or-path>            # single plan (positional arg)
aat run batch [directory]              # all plans, or filtered by subdirectory

# Validation (unified — bare validates everything, subcommands focus on one scope)
aat validate [--manifest FILE] [--strict]
aat validate graph [--graph FILE] [--oas FILE] [--templates DIR] [--strict]
aat validate plan [--graph FILE] [--plan FILE] [--unfed]
aat validate workflow [--graph FILE] [--strict]

# Web UI
aat web [--port 9119] [--open] [--dev] [--manifest FILE]
aat web view [ref] [--port 9119]        # open a specific run in the browser

# MCP server (stdio or HTTP transport, for IDE-based AI tools)
aat mcp serve [--manifest FILE] [--persona PERSONA] [--http] [--port PORT] [--log]

# Plan management
aat plan list [--manifest FILE]

# Environment management
aat env list [--manifest FILE] [--env-config FILE]

# Scaffold from OpenAPI spec
aat generate --oas FILE [--output-graph graph.yaml] [--output-templates templates/]

# Documentation generation
aat docs generate --graph FILE [--domain FILE] [--output FILE] [--title TEXT] [--split]
```

All `aat validate` subcommands support manifest auto-discovery: when `--graph` is omitted and a manifest is discoverable, the graph path resolves from the manifest. Explicit flags always override.

## Web UI

Svelte 5 + Vite 6 + TypeScript SPA in `server/web/`. Embedded via `//go:embed` in production builds.

- **Default port**: 9119
- **Router**: chi/v5
- **API routes**: `/api/runs`, `/api/runs/latest`, `/api/runs/{id}`, `/api/runs/{id}/steps/{stepId}`
- **Features**: run list, run detail with Gantt timeline, step detail with audit tabs
- **Dev mode**: `cd server/web && npm run dev` (Vite on :5173) + `aat web --dev` (Go server proxies to Vite)
- **Production**: `make build` embeds compiled frontend into the Go binary

## Task planning

If a task seems too aggressive to do in one operation, push back and offer to break it down into sub-tasks. When doing this, update the implementation plan with the new information so we can have clearly defined work items.

## Current Stage

Public launch work is tracked milestone by milestone in `LAUNCH-PLAN.md`; feature status is in `ROADMAP.md` and `CHANGELOG.md`. The historical stage tracker is archived at `docs/worklog/progress-archive-2026-02.md`.
