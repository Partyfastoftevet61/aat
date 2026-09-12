# Changelog

All notable changes to AAT are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow semver with a 0.x caveat:
the graph and plan formats may still change before 1.0.

## [Unreleased]

### Added
- The shop example ships an integration kit, the part of an AAT project that an API's integrators get.
  `aat-kit.yaml` names the graph, templates, OpenAPI spec, domain file, workflows, sandbox environment,
  and three reference plans. `sh package-kit.sh` copies them into a directory and a tarball, with the kit
  manifest as its `aat-project.yaml`. The `shop-api` server in `.mcp.json` reads the kit manifest, so it
  shows what an integrator's AI tool would see. `make example-shop` packages the kit, then validates and
  runs the unpacked copy.
- Docs: *Share Your API with Integrators* describes an integration kit for any API: what to ship and what
  to keep, a kit manifest beside the project manifest, internal environments that `include:` the shipped
  file, packaging and checking the kit in CI, and what an integrator's AI tool can read through the `api`
  persona. The docs home page, the examples index, Project Setup, and MCP Server link to it.
- `make demos` regenerates the docs site's recordings and screenshots against a fresh `aat-sandbox`: VHS
  recordings of `aat run plan full-lifecycle` and a parallel layer-group batch, Playwright screenshots of
  the run timeline, a step's request with Copy as cURL, and the batch matrix, plus an MP4 of the plan run
  and the repository's social preview (`demos/`). It checks that both recorded runs passed and that each
  GIF stays within its size budget before writing anything. The docs site's home, matrix-testing, and web UI
  pages show the results.
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
- **BREAKING:** a recipe's `overrides` must name steps of the composed plan. An override for any other step ID,
  such as an addon step without its `inc0_` prefix, used to be ignored; now the recipe fails to load, and the
  error lists the plan's steps. A value override on an input that the workflow template wires with `from`,
  `fromSelection`, or `fromInput` now replaces that wiring and sends the override. It used to have no effect.
- **BREAKING:** Lua transforms can no longer load code or reach the host process. The `package` library is gone,
  and so are the base library's `dofile`, `loadfile`, `load`, `loadstring`, `require`, `module`, `getfenv`,
  `setfenv`, `collectgarbage`, and `newproxy`. An integration kit's templates run on its users' machines, so a
  transform must not read their files. Also, `dofile()` and `loadfile()` with no argument read stdin, which under
  `aat mcp serve` is the MCP connection.
- Template headers no longer replace the auth credential, an override's own headers, or overlay headers. They
  still replace environment and plan headers, such as a per-operation `Content-Type`. On nodes that an
  override routes, overlay headers now replace the credential, as they already did on the default route.
  Header names compare case-insensitively when headers merge.
- **BREAKING:** request templates escape each substituted value for where it lands. Values used to go in
  raw.
  - **Path:** a value is URL-encoded as one path segment before the first `?`, and as a query component after
    it. `a/b` stays one segment, and `&` or `#` in a value can no longer add a parameter or cut the URL.
  - **JSON body:** a value inside quotes is JSON-escaped, so a quote, backslash, or newline in a field such
    as `notes` no longer breaks the body. A value outside quotes is written as JSON: arrays and objects as
    JSON, numbers in plain digits, and `null`.
  - **Form body:** values are URL-encoded.
  - **Headers and other bodies:** unchanged.

  To send a malformed payload on purpose, use a step's `rawBody`.
- **BREAKING:** JSON keys follow the convention of the document they are in:
  - The `aat run batch --json` summary's `batchId` is now `batch_id`. It is left out when the batch stopped
    before it started.
  - In run archives, a step's `duration_ms` is now `durationMs`. AAT and `tools/aat-to-junit.py` still read
    archives that use the old key.
  - A plan's `auth` in `metadata.plan` and `metadata.instantiatedPlan` uses camelCase keys (`tokenUrl`,
    `credentials`) instead of Go field names (`TokenURL`, `Credentials`).

  The `state` object that `--json` nests with `--dump-state -` keeps the camelCase keys of the `--dump-state`
  file.
- **BREAKING:** `aat run batch <filter>` selects plans by whole path segments. `orders` selects `orders.yaml`
  and every plan under `orders/`, and `orders/refund` selects one plan. The filter matched the start of each
  plan's path, so `smoke` also ran `smoke-eu.yaml`.
- **BREAKING:** a batch that finds no plans exits `2`, and the message names the filter and the plan
  directories. It used to pass with nothing run, so a mistyped filter passed in CI. An absolute path that does
  not exist gets the same error.
- **BREAKING:** exit codes follow one rule on every command:
  - `0` passed.
  - `1` a test or validation ran and found a failure.
  - `2` AAT could not do what was asked. That covers an unknown flag, argument, or subcommand, and a project,
    environment, or `--var` error. Most commands exited `1` for these.
  - `130` aborted.

  Command by command:
  - `aat validate` exits `1` when it finds a problem, and `2` when there is no manifest to validate or
    `--var` is bad.
  - `aat prompt` exits with its run's outcome code, instead of `1` for any run that did not pass.
  - `aat import`, `aat generate`, and `aat docs generate` exit `2` on an error.
- **BREAKING:** `aat run batch --json` reports an error that stops the batch before any plan runs in a top-level
  `error` field, with an empty `runs` array. It used a run entry with no plan name.
- **BREAKING:** only `aat run plan` and `aat run batch` take the execution flags: `--env`, `--env-config`,
  `--graph`, `--templates`, `--domain`, `--override`, `--overlay`, `--var`, `--retries`, `--layer`,
  `--no-auto-overrides`, `--oas-validate`, `--verbose-auth`, and `--no-mutations`. `aat run clean` and
  `aat run rebuild-summaries` accepted and ignored them; they now reject them.
- Docs: the README and shop quick starts run `aat web view latest` last, since it holds the terminal until
  Ctrl+C, and then stop the sandbox. The Petstore page is now *Petstore Quickstart*, so "quick start" means
  the offline shop. The MCP server page describes `--persona` with `--http` as the code behaves, and the
  workflow and plan guides use generic airline operation names in their examples. The README, roadmap,
  and airline case study say AAT was built and proven against the airline API.
- MCP operation details and `explain_field` label a graph input's default "Test default": it is data AAT
  sends in tests, not a value the API fills in. Read as a plain default, the shop's `quantity: 1` and
  `method: card` made required fields look optional.
- A plan-level cleanup step that names a node a step's graph node pairs with (`cleanup:` on the node)
  runs from the graph pairings: once per created resource, newest first, and not at all when the creating
  step failed. Its `runOn` still decides whether it runs. Recipes and `aat prompt` plans list every
  pairing in step order, so they deleted a cart before its order, and sent cleanup requests with
  unresolved placeholders when the create step had failed.
- The shop example describes how its API works, for the AI tools that read it through MCP. Each operation
  lists its error codes in the order they are checked, and each input and output says what it must be,
  where it comes from, and what it means. New domain concepts cover authentication, the payments host, the
  error envelope, transient failures, the cart and order lifecycles, stock, regional pricing, and money.
  The kit's README adds connection details, a flow map, and the rules that matter.
- Docs: the README, *Share Your API with Integrators*, and the airline case study describe what an
  integration kit tells an AI tool beyond an OpenAPI spec: the whole workflow, from call order and data
  hand-offs to the fields that matter and what a failure looks like. With it, a working client in any
  language takes a single prompt, as it did on the airline API in Java, C#, Go, Python, Perl, and Lisp.
  The integration-kit page publishes the prompt, the setup, and the results of the shop's Python and Go
  runs, so the claim can be checked; the shop README uses the same prompt.
- `examples/shop` keeps the suites its integration kit does not ship in `internal/plans/`: the negative
  tests, `resilience`, and `giftcard-express`. The manifest lists `plans/` and `internal/plans/`, so plan
  names (`negative/state-machine`) and batch results are unchanged; only the files' paths moved.
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
- Repository leftovers from the private airline project (`setup.sh`, a root-level plan, IDE run
  configurations, an empty case-study stub).

### Fixed
- An override `values:` entry (from `env.yaml`, `.aat-overrides.yaml`, or `--overlay`) is recorded in the
  archive as the input's resolution, with the source `override_value`. The decision trail used to show the plan
  value that the override replaced.
- An `errorDetection` `equals` rule with a number (`value: 0`) matches the JSON number. The YAML integer and the
  JSON number used to compare as different types, so the rule never matched. A map or list `value` is now a
  validation error; at run time it crashed `aat run` and the MCP server.
- `aat prompt` keeps the headers of `.aat-overrides.yaml` when the plan sets its own auth or headers; it used to
  drop them.
- A request path value with an encoded `/` (`%2F`) keeps it inside its segment; the executor used to decode it
  into a real `/`. The archive records the URL as the executor joins it, instead of concatenating the base URL
  and the path.
- A number of a million or more fills a placeholder in plain digits instead of exponent form (`1.2e+06`), and a
  map fills one as JSON instead of Go syntax (`map[k:v]`).
- Archive references stay inside the archive directory:
  - `aat import --name` must be a single directory name. A name that starts with `run-` or `batch-` gets the
    `!` prefix, as a name derived from the file does. `--name ../x` used to import outside the archive
    directory.
  - The web server answers `404` for a run, batch, or trace ID that is not a single directory name. An ID of
    `..` read the parent directory's `archive.json`. `PUT /api/runs/{id}/name` renamed any directory in the
    archive directory; it now renames only runs and batches.
  - The MCP archive tools reject a `run_id` that is not a directory name. The plan tools reject absolute plan
    names and names that climb out of the plans directory.
- An environment file given with `--env-config` no longer takes the manifest's `defaultEnvironment`. A
  single-environment file therefore loads in a project whose manifest sets one; it failed with "--env is not
  applicable". A single-environment file also ignores `AAT_ENV_NAME` and an overlay's `environment:`, and only
  an explicit `--env` is an error for it. `aat run plan`, `aat run batch`, and `aat prompt` now choose the
  environment in one place.
- An unknown subcommand is an error (exit `2`), with a suggestion when the name is close. This covers
  `aat run bogus` and unknown subcommands of `aat plan`, `aat env`, `aat mcp`, and `aat docs`. They printed
  help and exited `0`, so a mistyped subcommand passed in CI. Commands that take no arguments, such as
  `aat validate` and `aat web`, now reject stray arguments instead of ignoring them.
- `aat mcp serve` reports a manifest that fails to load with the load error; it said the manifest was not
  found. `aat import` fails on such a manifest instead of importing into `_output/runs`.
- `aat run plan --json` and `aat run batch --json` print the error document for every error that stops them
  before a plan runs. Before, a manifest that failed to load, a bad `--var`, or an overlay environment that
  could not be resolved left stdout empty.
- The shop kit's descriptions match the sandbox. `applyCoupon` lists its errors in the order they are
  checked; `paymentCharge` and `shipOrder` say they check the order again after their delay;
  `createReturn` and `deliverShipment` list their 404s; `addItem.quantity` and `paymentCharge.method` say
  they are required. Order lines have no `*Display` fields, `GC-100-DEMO` covers orders up to 10000 minor
  units, and the stale inventory read happens once per token in each region. `openapi.yaml` declares the
  400 responses of `createCart` and `applyCoupon`.
- Graph-level cleanup deletes the resource each step created. Cleanup looked up the creating node's
  outputs by node name, but outputs are stored by step ID, so a step with its own `id` fell through to the
  first step with an output of that name: two `createCart` steps with their own IDs deleted the first cart
  twice and left the second. Cleanup now reads the registering step's outputs, then the most recent step
  with a matching output.
- `aat run plan` and `aat run batch` exit with code 2 when the project manifest cannot be found or loaded,
  or an overlay's environment cannot be resolved. They exited 1, the code for a failed test, so CI read a
  broken manifest as a test failure.
- A request that fails before any response reports `executing HTTP request: …` once, not
  `executing request: executing HTTP request: …`.
- `aat mcp serve` starts when a relative `--manifest` names an `oas:` spec. The spec path was joined onto
  the graph's directory a second time (`examples/shop/examples/shop/openapi.yaml`), so a project loaded
  from another directory failed with "no such file or directory".
- The MCP `get_sample_response` tool returns the newest successful response for an operation, and a
  failed one, marked as such, only when no run succeeded. It took the newest response of any status, so a
  negative test's `409` could pass for the sample. It also searches the runs inside batch directories,
  and a manifest without `archives` gets the expected output shape instead of an error. The archive tools
  accept the ID of a run inside a batch.
- A run archive that cannot be redacted is not written. `aat run`, batch runs, and the MCP `execute_plan`
  tool report the error; before, the error was ignored and the archive was written with its secrets in
  place. Redaction fails only on a value JSON cannot hold, such as a NaN from a Lua transform.
- `aat mcp serve` without `--persona` registers `get_data_flow`, `get_response_shape`, and `explain_field`,
  which only the `api` persona had, so it has every tool: 39 with an OpenAPI spec, 32 without.
- The web UI's run timeline shows a step's assertion count only when the step has assertions, not
  `0 / 0` on every step.
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
