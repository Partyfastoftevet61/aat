# Changelog

All notable changes to AAT are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow semver with a 0.x caveat:
the graph and plan formats may still change before 1.0.

## [Unreleased]

### Added
- `make demos` regenerates the docs site's recordings and screenshots against a fresh `aat-sandbox`: VHS
  recordings of `aat run plan full-lifecycle` and a parallel layer-group batch, Playwright screenshots of
  the run timeline, a step's request with Copy as cURL, and the batch matrix, plus an MP4 of the plan run
  and the repository's social preview (`demos/`). It checks that both recorded runs passed and that each
  GIF stays within its size budget before writing anything.
- A documentation site built from `docs/user` with Material for MkDocs (`mkdocs.yml`), deployed to GitHub
  Pages by a new Docs workflow that fails on broken links, broken anchors, and pages missing from the
  navigation. `make docs` runs the same strict build locally.
- Documentation pages for installing, checkpoints, archives, `aat generate`, `aat docs generate`, the
  examples, and the airline case study; the Lua transforms page is written out. The quickstart (on the
  public Petstore API) and the tutorial (built by hand against `aat-sandbox`) are rewritten and were run
  verbatim; the previous versions did not load.
- Run output lists each failed assertion under its step (`status: expected status 200, got 201`), and the
  `--json` step summary includes them as `failed_assertions`.
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
- Release archives and the Homebrew cask include `aat-sandbox` next to `aat`. `make example-shop` runs
  `examples/shop` against a local sandbox the way the new CI `example-shop` job does, and `make cli`
  builds `aat` without rebuilding the web UI.
- Workflow slot options and addons can declare `verification:`. Composition merges it into the base
  workflow's, and a node they verify replaces the base's verification of that node.
- A step that succeeds after retrying shows `retried Nx: <category>` in run output, and archives record
  the category of each retried attempt in `retriedOn`.
- `status` assertions accept a status class such as `expect: 2xx` or `expect: 4xx`.
- `--var KEY=VALUE` (repeatable) on `aat run plan`, `aat run batch`, `aat prompt`, `aat validate`,
  `aat env list`, and `aat mcp serve` sets a var of a multi-environment file for one invocation, for
  example to point `examples/shop` at a sandbox on other ports or in a container. A key the file never
  declares or references is an error.
- `--host` on `aat web`, `aat web view`, `aat web viewtrace`, and `aat mcp serve` chooses the interface
  to bind (default `127.0.0.1`; the `AAT_HOST` environment variable sets it when the flag is absent).
  The Docker image sets `AAT_HOST=0.0.0.0`.
- `--dump-state` exports record each step's `baseUrl` and live request `headers`. The `--json` run
  summary adds `stopped_at` for a checkpoint and `retried_on` per step.
- Template conditional and iteration blocks accept keys with hyphens, such as
  `{{?X-Request-Id}}…{{/X-Request-Id}}` for a header parameter.
- `aat validate` checks the layers directory: parse errors, duplicate layer names, and layer input keys
  that match no node input (which layers silently ignored).
- `aat validate` checks the domain file and `visualizers.yaml` (Domain and Visualizers sections), and
  names each error's file relative to the working directory.
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
- **BREAKING:** project YAML is decoded strictly. A key that no field accepts — in the manifest,
  environment files and their includes, overlays, the graph, templates, the domain file, visualizers,
  workflows, layers, plans, recipes, and plan YAML given to the MCP plan tools — is an error naming the
  file, the line, and the likely intended key (`plans/smoke.yaml: line 12: unknown key "fromSelecton"
  in step value (did you mean "fromSelection"?)`). Such keys were silently ignored, so a typo produced a
  plan that loaded and did something else. `aat prompt` model output (JSON) is unaffected. To migrate,
  run `aat validate` and fix what it lists.
- **BREAKING:** a manifest that exists but fails to load is an error for every command that discovers
  it; it was skipped, so commands fell back to a lower-priority project or to none. A missing manifest
  is still skipped, and a higher-priority manifest that loads still wins.
- Run output names each step by its step ID, which `--stop-after`, `dependsOn`, the archive, and the web
  UI use, with the node in parentheses when the two differ and the column has room
  (`addProduct (addItem)`); it printed the node, so two steps on one node looked alike and the name shown
  was not the one `--stop-after` accepts. Engine errors do the same (`step "addSocks" (addItem) returned
  status 409`), and so do the parallel batch display and the MCP `execute_plan` table.
- Durations are wall-clock: a retried step's duration runs from its first attempt to the end of its last,
  so retry waits count, and a run's duration (the `PASSED` line, the `--json` `summary.duration_ms`,
  `batch.json` run entries, the web UI, and MCP) is the time the run took, recorded in the archive as
  `result.durationMs`. Both were sums of the last attempt of each step, so `full-lifecycle` printed `955ms`
  for a three-second run. Archives written before keep showing the sum. Step durations of a second or more
  read `1.4s`.
- A step that fails after retrying prints its error followed by the same `retried Nx: <category>` note as
  a step that recovers; the note used to take one of two other forms depending on the terminal width.
- `aat run` progress output marks OpenAPI violations on each step (`OAS: 1 warning(s)`) and totals them
  after the outcome, as documented; only an unused summary path printed them before.
- `aat validate` and `aat validate workflow` show OpenAPI and workflow-compatibility warnings as a `WARN`
  section without `--strict` instead of reporting `OK`, and counts read "1 file" rather than "1 files".
- The sequential batch header no longer prints `mode=strict`, a leftover of the runtime modes removed in
  0.0.2.
- `aat web` and `aat mcp serve --http` listen on `127.0.0.1` by default instead of every interface.
  Pass `--host 0.0.0.0` (or set `AAT_HOST`) to accept connections from other machines.
- Steps composed from workflow templates (recipes, `aat prompt`) get a default `status: 2xx`
  assertion instead of `status: 200`, and none when they declare `expectFailure`. On a step with
  `expectFailure` (including one added by an overlay), a `status` assertion that expects success is
  reported as skipped, since the expected-failure status list is the status check; one that agrees
  with it, such as `409` or `4xx`, is evaluated.
- `--dump-state`: the top-level `baseUrl` and `auth.headers` describe the environment's default route
  (the last request sent to `apiBaseUrl`), so a run whose last step went to another host still exports
  the main session.
- The `--json` step summary's `name` is the step ID, as documented, instead of the node name, so
  mutation siblings and repeated nodes are distinguishable.
- `${var}` substitution in multi-environment files covers every string of the environment (override
  auth header names and credentials, override values, LLM settings, `settings`), and so does the
  unresolved-variable check.
- `aat generate --oas` places optional query parameters, headers, and body properties in conditional
  blocks, orders body properties as the spec does, and writes integer, number, boolean, and array body
  values as JSON literals. With `--output-graph -` it writes no files unless `--output-templates` is
  given. A template header that resolves to an empty conditional is not sent.
- A layer that sets a value source (`value`, `pool`, `from`, `fromResolved`) replaces the graph
  default's source instead of merging with it, so a layer value is no longer shadowed by a default's
  `from`.
- libopenapi-validator v0.14.0 and libopenapi v0.38.7.
- An override that declares its own `auth` no longer sends the inherited credential header
  (`Authorization`, or the top-level API key header) to its host.
- Requesting layers (`--layer`, `--layer-group`, or a recipe's `selection.layers`) without a
  `layers:` directory in the manifest is an error; the layers were silently ignored before.
- Override precedence: among glob (and among exact) overrides the last registered match now wins, so
  `.aat-overrides.yaml`, `--overlay`, and `--override` take precedence over `env.yaml` overrides as
  documented. Exact names still beat globs.
- Cleanup input matching scans earlier steps in execution order (it was map order).
- CLI description and `--help` text describe AAT as graph-based API workflow testing; the LLM is
  optional and authoring-time only, and execution never calls one.
- Documentation: renamed flags (`--env`, `--env-config`, `--overlay`) corrected throughout; undocumented
  features documented (Ctrl+C `aborted` outcome, `--oas-validate`, batch matrix view, Copy as cURL,
  archive import/export, `aat run clean`, `aat run rebuild-summaries`).
- Minimum Go version is 1.25.7 (the OpenAPI libraries require it).

### Removed
- **BREAKING:** YAML keys that nothing read, which strict decoding now rejects: step `fallback`,
  `assertions.semantic`, the environment settings `maxRunDuration`, `defaultRetries`, and
  `archiveFormat` (retry with a step's `retry:` or `--retries`), template `response.validate`, the
  `prompt` field of selections and graph default `select`, and recipe `overrides.descriptions`.
- **BREAKING:** the `llm` selection strategy, which plan validation already rejected but `aat prompt`
  accepted from the model, and the `warn` OpenAPI validation mode, which behaved exactly like `auto`.
- The MCP `execute_plan` tool no longer accepts the obsolete `mode` parameter (the runtime
  strict/lean/adaptive modes were removed in 0.0.2).
- Airline-era repository leftovers (`setup.sh`, a root-level plan, IDE run configurations, the
  empty Airline case-study stub).

### Fixed
- Run archives redact known secrets from every string they hold: request URLs and query parameters,
  request and response bodies, outputs and display outputs, error, assertion, and OpenAPI messages,
  and plan step values, as well as headers, inputs, and resolved values; `batch.json` entries too. In
  JSON bodies only string values change. Before, a credential used as an input was redacted in `inputs`
  but kept in the body of the same request.
- Archive redaction no longer mangles ordinary data: the oauth2 `username` and `clientId` are not
  treated as secrets, and a secret shorter than eight characters is redacted only where a whole value
  equals it. The shop sandbox's `demo` credentials had turned `demo@example.com` into
  `[REDACTED]@example.com` in inputs. Overlapping secrets are redacted completely; map order could
  leave part of one visible.
- Run archives redact an API key sent under a custom `auth.headerName`, the credentials of host overrides
  and overlay overrides (they were never collected as secrets), and the literal credentials and credential
  headers of the plan stored in `metadata.plan` and `metadata.instantiatedPlan`; `aat prompt` archives also
  collect the credentials of `.aat-overrides.yaml`. `--verbose-auth` shows at most half of a short password
  or client secret.
- The `ABORTED` and `STOPPED` lines count the steps the plan meant to run (`ABORTED (2/5 steps, …)`);
  both numbers were the steps that ran.
- `aat generate --oas` marks a response property the schema does not list as `required` as an optional
  output with an optional extract rule, so a scaffolded step no longer fails when the API omits it.
- A `select` with a `filter` and no `strategy` works like `match` instead of failing at run time with
  `unknown selection strategy`.
- `--manifest` naming a file that does not exist is an error instead of silently falling back to manifest
  discovery.
- `aat run plan` no longer prints a failed or errored run's message a second time on stderr.
- A project's manifest no longer inherits fields it leaves out (such as `domain`, `layers`, or
  `defaultEnvironment`) from a lower-priority project named by `AAT_PROJECT` or the user config's
  `default_project`; the highest-priority manifest found describes the whole project.
- An override entry that sets only `values:` or `expectFailure:` no longer reroutes its node to the
  top-level base URL and auth; it keeps the route a broader match gives it.
- In multi-environment files, a child environment's overrides (`extends`, `include`) take precedence
  over inherited ones again; the last-match-wins change had inverted them.
- `aat validate`, `aat validate plan`, and the MCP plan tools resolve recipe layers, so a misspelled
  layer is reported and layer-supplied inputs no longer fail validation.
- MCP `execute_plan` applies override values, `expectFailure`, and recipe layers like `aat run plan`.
- `aat plan list` summarizes recipes instead of reporting a parse error for each.
- `--dump-state -` without `--json` writes only the state to stdout; progress and the summary line go to
  stderr, so the output pipes into `jq` as documented.
- Workflow compatibility checking (`aat validate`) accounts for slots: an addon `AUTOWIRE` input that every
  option of a slot produces is no longer reported as unfed, an addon that attaches after a slot option's
  node is checked instead of skipped, and slot options are no longer checked as bases of their own.
- The static OpenAPI output check looks each output up at its template extract path, through nested objects
  and array items, instead of requiring a top-level response property named after the output.
- `settings.oasValidation` and `--oas-validate` reject unknown values instead of treating them as `auto`.
- A workflow template whose `verification:` names a node missing from the graph fails to load, as cleanup
  entries already did.
- `aat run batch --parallel N` with runtime OpenAPI validation no longer has a data race: parallel runs share
  one loaded spec, and libopenapi-validator v0.13.1 wrote into the schema model while rendering a response
  schema behind a `$ref`. The upgrade to v0.14.0 removes the race, so validations still run concurrently.
- Workflow composition fills slots in declaration order, so merged cleanup, slot verification, and slot
  `inject` values no longer vary between runs, and neither can batch dedup fingerprints.
- `--stop-after` stops after a passing `expectFailure` step instead of running on to the end.
- Ctrl+C during a request or a retry wait ends the run as `aborted` (exit code `130`) with cleanup, as
  documented; it was reported as an `error` (exit code `2`), so an interrupt was aborted only when it landed
  between steps. Cleanup after an interrupt keeps its 30-second budget, which was lost on the way to the
  cleanup requests.
- `--stop-after` naming a node instead of a step ID says which step IDs run that node.
- A project YAML file with a second document (`---` followed by content) is an error naming its line;
  strict decoding read only the first document and silently ignored the rest.
- The sequential batch display prints a run's `OAS: N warning(s)` total, as the plan display does.
- With `--dump-state -`, run output is coloured when stderr, where it goes, is a terminal; colour
  followed stdout.
- Cleanup steps carry a step ID and a start time in archives, so the web UI places them on the timeline;
  their start time was empty.
- The web UI shows why a step retried (`retried 2x: transient`) on the run timeline and the step page; the
  server dropped the archive's `retriedOn`, and the badge read "2 RETRY".
- The batch By Test matrix no longer clips its rotated permutation labels: the header grows to fit the
  longest, and a wide matrix uses the space beside the page column. The run timeline shows a step's node
  only when it differs from the step ID.
- `--override NODE=URL` routes keep the environment headers, plan headers, overlay headers, and the
  credential, like an `overrides:` entry with that `match` and `baseUrl`; they used to send no headers.
- `aat prompt` rejects layers when the manifest sets no layers directory instead of silently running
  without them.
- A `--dump-state` file that cannot be written is reported on stderr even under `--quiet` or `--json`, and
  a dump that replaces an existing file ends up with mode `0600`.
- Lua transforms: `print()` writes to stderr instead of stdout (where it corrupted `--json` and
  `--dump-state -` output), `return {}` is a valid empty set of outputs, and a template with a transform
  but no `extract` rules runs its transform; outputs a transform computes no longer fail the adapter
  output check.
- `aat generate --oas` extracts an array property of an object response by its name instead of `@this`.
- The static OpenAPI check accepts a required parameter or body property that the template sends itself
  (such as a literal `"photoUrls": []`), which made `examples/petstore` fail `aat validate --strict`.
- `aat validate` checks workflow templates in subdirectories of the workflows directory (such as
  `workflows/slots/`), which it skipped.
- A layer's `fromResolved` applies over an existing graph default instead of being dropped.
- The plan summary resolves `intent.goal` as a step ID; `aat plan list` truncates long goals on character
  boundaries.
- Visualizers receive `--color-text-secondary`, `--color-danger`, and `--color-warning` as documented; the
  web UI sent variable names it does not define.
- "executing plan (N steps)" counts the mutation and verification steps the progress output numbers.
- `make clean` no longer deletes the tracked `server/web/dist/index.html`, which broke `go build`.
- `--verbose-auth` no longer prints the full access token in the logged token response.
- `aat validate`, `aat generate --oas`, and the MCP OpenAPI operation details include parameters
  declared on an OpenAPI path item (such as a shared `{cartId}`), not only those on the operation.

## [0.0.4] - 2026-03-04

See the [GitHub release](https://github.com/gburgyan/aat/releases/tag/v0.0.4). Earlier history will be
backfilled from tags when 0.1.0 is cut.

[Unreleased]: https://github.com/gburgyan/aat/compare/v0.0.4...HEAD
[0.0.4]: https://github.com/gburgyan/aat/releases/tag/v0.0.4
