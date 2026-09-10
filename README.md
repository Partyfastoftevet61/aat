# AAT — Adaptive API Toolkit

[![CI](https://github.com/gburgyan/aat/actions/workflows/ci.yml/badge.svg)](https://github.com/gburgyan/aat/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/gburgyan/aat)](https://goreportcard.com/report/github.com/gburgyan/aat)
[![Go Reference](https://pkg.go.dev/badge/github.com/gburgyan/aat.svg)](https://pkg.go.dev/github.com/gburgyan/aat)
[![License](https://img.shields.io/github/license/gburgyan/aat)](https://github.com/gburgyan/aat/blob/main/LICENSE)

AAT is a CLI tool that tests API workflows end-to-end. You define your API as a graph of operations, write test plans that traverse it, and AAT handles the execution — resolving data dependencies between steps, running assertions, and producing detailed archives of every request and response.

With an LLM configured, AAT can also generate test plans from natural language prompts and intelligently select values when explicit data isn't available.

## Quick start

The fastest way to see AAT work is the offline shop: `aat-sandbox` serves a small e-commerce API on your machine, and `aat-sandbox init` extracts a complete AAT project for it (a 17-operation graph, workflows, layers, two regions, negative tests). No signup, no network.

```bash
aat-sandbox init shop && cd shop   # extract the example project
aat-sandbox serve &                # shop API on :8765, payments API on :8766
aat run plan full-lifecycle        # one order through every state, verified and cleaned up
aat run batch --layer-group shipping-standard,shipping-express --layer-group basket-gear,basket-apparel --parallel 4
aat web view                       # browse the latest run
aat run plan smoke --env eu        # the same plan with EU prices and VAT
```

From a source checkout, `make build` builds both binaries; run the same commands from `examples/shop` as `../../aat` and `../../aat-sandbox`. The [shop README](examples/shop/README.md) explains what each command shows.

For the smallest example, [examples/petstore](examples/petstore/README.md) runs two-step plans against the public Petstore API, and the [Petstore Walkthrough](docs/user/petstore-walkthrough.md) explains how the files fit together. To set AAT up for your own API, follow the [Quickstart guide](docs/user/quickstart.md).

## How it works

AAT uses four file types to describe and execute API tests:

| File | Purpose |
|------|---------|
| **Graph** (`graph.yaml`) | Defines API operations as nodes with typed inputs/outputs, connected by data-flow edges. This is the "map" of your API. |
| **Templates** (`templates/*.yaml`) | HTTP request/response templates for each operation. Define method, path, headers, body, and response extraction rules. |
| **Environment** (`env.yaml`) | Connection details: base URL, authentication, retry settings. Swap environments to test staging vs production. |
| **Plan** (`plan.yaml`) | Test scenario: which steps to run, input values, and assertions. Can be hand-written or generated from a prompt. |

Data flows automatically between steps via graph edges. When step A produces an output that step B needs, AAT resolves it without manual wiring in the plan.

## Commands

| Command | Description |
|---------|-------------|
| `aat run plan <file>` | Execute a single test plan against a live API |
| `aat run batch [dir]` | Execute all plans in a directory |
| `aat prompt "<text>"` | Generate a test plan from a natural language prompt (requires LLM config) |
| `aat validate` | Validate graph, plans, and workflows |
| `aat web` | Launch the web UI for browsing run archives |
| `aat generate --oas <spec>` | Scaffold a graph and templates from an OpenAPI spec |
| `aat docs generate --graph <file>` | Generate Markdown documentation from a graph |
| `aat mcp serve` | Start the MCP server for IDE integration |

### CI/CD mode

```bash
./aat run plan plan.yaml --env env.yaml --graph graph.yaml --templates tpl/ \
  --json --quiet
```

- Exit code 0 = all assertions passed
- Exit code 1 = test failure
- Exit code 2 = infrastructure error
- `--json` outputs a machine-readable summary to stdout
- `--quiet` suppresses progress output

## Building

Requires Go 1.25+, Node.js 18+, and Make:

```bash
make build    # Compiles Svelte frontend, then Go binary with version/commit/date
make test     # Runs go test ./...
make clean    # Removes binary and frontend artifacts
```

`make build` embeds the compiled web UI and injects version metadata via ldflags. A bare `go build ./cmd/aat/` works for development but skips the frontend and reports version as "dev".

## Documentation

- [Shop example](examples/shop/README.md) — the offline quick start: layers, regions, negative tests, checkpoints, MCP
- [Quickstart](docs/user/quickstart.md) — install AAT and set it up for your own API
- [Petstore Walkthrough](docs/user/petstore-walkthrough.md) — line-by-line tour of graph, templates, workflows, and recipes
- [Petstore Quickstart](examples/petstore/README.md) — runnable example with no setup
- [Graphs](docs/user/graphs.md) — nodes, edges, conditions, OAS linking
- [Templates](docs/user/templates.md) — HTTP request/response template format
- [Plans](docs/user/plans.md) — test plan YAML schema and assertions
- [Environments](docs/user/environments.md) — auth, headers, LLM configuration
- [Domain Knowledge](docs/user/domain.md) — concepts, types, value pools
- [Value Flow](docs/user/value-flow.md) — expressions, selections, constraints, resolution hierarchy
- [Running Tests](docs/user/running.md) — CLI usage, archives, CI/CD integration
- [LLM-Assisted Planning](docs/user/prompt.md) — generating plans from natural language
- [MCP Server](docs/user/mcp-server.md) — IDE integration for AI-assisted workflows

## Status

AAT is in active development and is used daily against a 74-node airline API. The core engine, graph model, plan execution, validation, archiving, LLM-assisted plan authoring, web UI, and MCP server are complete. See [ROADMAP.md](ROADMAP.md) for what is next.

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines. By submitting a pull request, you agree to the [Contributor License Agreement](CLA.md).

## License

Apache 2.0 — see [LICENSE](LICENSE).
