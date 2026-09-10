# Changelog

All notable changes to AAT are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow semver with a 0.x caveat:
the graph and plan formats may still change before 1.0.

## [Unreleased]

### Added
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

## [0.0.4] - 2026-03-04

See the [GitHub release](https://github.com/gburgyan/aat/releases/tag/v0.0.4). Earlier history will be
backfilled from tags when 0.1.0 is cut.

[Unreleased]: https://github.com/gburgyan/aat/compare/v0.0.4...HEAD
[0.0.4]: https://github.com/gburgyan/aat/releases/tag/v0.0.4
