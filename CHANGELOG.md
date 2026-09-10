# Changelog

All notable changes to AAT are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow semver with a 0.x caveat:
the graph and plan formats may still change before 1.0.

## [Unreleased]

### Added
- `aat-sandbox`, a second binary that serves an offline e-commerce demo API (`aat-sandbox serve`:
  shop API on :8765 with OAuth2 tokens, payments API on :8766 with an API key, `us`/`eu` regions
  with their own currency, tax, tiers, and coupons, an order state machine, simulated latency, and
  two chaos hooks for retry demos) and extracts the `examples/shop` project (`aat-sandbox init`).
  The contract lives in `examples/shop/openapi.yaml`; the server tests validate every response
  against it. `make build` builds both binaries; `make sandbox` builds only the demo server.
  The sandbox binds `127.0.0.1` unless `--host` says otherwise.
- `examples/shop`, the offline quick-start project for `aat-sandbox`: a 17-operation graph, Quick
  Purchase and Checkout workflows with slots and addons, 12 layers, 7 plans (full order lifecycle,
  retries, a negative state-machine walk, `addItem` mutations), a declined-card overlay, a receipt
  visualizer, `us`/`eu` environments with payments routed to their own host and credential, and MCP
  configuration for AI coding tools.
- A step that succeeds after retrying shows `retried Nx: <category>` in run output, and archives record
  the category of each retried attempt in `retriedOn`.
- `status` assertions accept a status class such as `expect: 2xx` or `expect: 4xx`.
- `aat validate` checks the layers directory: parse errors, duplicate layer names, and layer input keys
  that match no node input (which layers silently ignored).
- Plan-level `execution.cleanup` steps now execute after the main flow, in declaration order and
  honoring `runOn: always|success|failure`, before graph-level cleanup pairings.
- Plan `execution.verification` steps now execute after the main flow and before cleanup, with their
  assertions counting toward the run outcome. They appear in archives as `verify_<node>` steps.
- `retry.on` and `retry.failOn` accept HTTP status codes (`on: [503]`) alongside category names;
  `aat validate plan` rejects unknown rules.
- `--oas-validate strict` (and `settings.oasValidation: strict`) now fails a step whose request or
  response violates the OpenAPI spec; it previously behaved like `auto`. `expectFailure` steps are
  exempt, and skipped validations or schema compilation warnings never fail a step.
- The web UI styles the `aborted` and `stopped` run outcomes.
- `aat web` reports a clear error (exit code 2) when the frontend bundle is not embedded, such as a
  plain `go install` build; the CLI, MCP server, and CI features still work in that build.
- `aat --version` reports the module version for `go install …@vX.Y.Z` builds.
- Release pipeline: versionless archive names for stable `releases/latest/download/…` URLs, a
  Homebrew cask in a tap (`brew install gburgyan/tap/aat`), and a multi-arch image at
  `ghcr.io/gburgyan/aat`.
- Repository scaffolding: issue and pull request templates, `SECURITY.md`, and `ROADMAP.md`.

### Changed
- Steps composed from workflow templates (recipes, `aat prompt`) get a default `status: 2xx`
  assertion instead of `status: 200`, and none when they declare `expectFailure`. On any step with
  `expectFailure` (including one added by an overlay) status assertions are reported as skipped:
  the expected-failure status list is the status check.
- An override that declares its own `auth` no longer sends the inherited credential header
  (`Authorization`, or the top-level API key header) to its host.
- Requesting layers (`--layer`, `--layer-group`, or a recipe's `selection.layers`) without a
  `layers:` directory in the manifest is an error; the layers were silently ignored before.
- Override precedence: among glob (and among exact) overrides the last registered match now wins, so
  `.aat-overrides.yaml`, `--overlay`, and `--override` take precedence over `env.yaml` overrides as
  documented. Exact names still beat globs.
- Cleanup input matching scans earlier steps in execution order (it was map order).
- CLI description and `--help` text describe AAT as graph-based API workflow testing; the LLM is
  optional and authoring-time only.
- Documentation: renamed flags (`--env`, `--env-config`, `--overlay`) corrected throughout; undocumented
  features documented (Ctrl+C `aborted` outcome, `--oas-validate`, batch matrix view, Copy as cURL,
  archive import/export, `aat run clean`, `aat run rebuild-summaries`).
- Minimum Go version is 1.25.

### Removed
- The MCP `execute_plan` tool no longer accepts the obsolete `mode` parameter (the runtime
  strict/lean/adaptive modes were removed in 0.0.2).
- Airline-era repository leftovers (`setup.sh`, a root-level plan, IDE run configurations, the
  empty Airline case-study stub).

### Fixed
- An override entry that sets only `values:` or `expectFailure:` no longer reroutes its node to the
  top-level base URL and auth; it keeps the route a broader match gives it.
- In multi-environment files, a child environment's overrides (`extends`, `include`) take precedence
  over inherited ones again; the last-match-wins change had inverted them.
- `aat validate`, `aat validate plan`, and the MCP plan tools resolve recipe layers, so a misspelled
  layer is reported and layer-supplied inputs no longer fail validation.
- MCP `execute_plan` applies override values, `expectFailure`, and recipe layers like `aat run plan`.
- `aat plan list` summarizes recipes instead of reporting a parse error for each.
- `--verbose-auth` no longer prints the full access token in the logged token response.
- `aat validate`, `aat generate --oas`, and the MCP OpenAPI operation details include parameters
  declared on an OpenAPI path item (such as a shared `{cartId}`), not only those on the operation.

## [0.0.4] - 2026-03-04

See the [GitHub release](https://github.com/gburgyan/aat/releases/tag/v0.0.4). Earlier history will be
backfilled from tags when 0.1.0 is cut.

[Unreleased]: https://github.com/gburgyan/aat/compare/v0.0.4...HEAD
[0.0.4]: https://github.com/gburgyan/aat/releases/tag/v0.0.4
