# Launch M1 — offline sandbox API + `examples/shop`

## 2026-09-10 — Phase A: sandbox server, OpenAPI contract, `aat-sandbox` binary

**What:** Built `internal/sandbox/shop` (stdlib-only leaf package), `examples/shop/openapi.yaml`
(OAS 3.0.3, 17 operationIds), `cmd/aat-sandbox` (`serve`, `init`), and a root `embed.go` that
embeds the example for `init`. Server tests run every request through a contract validator built
on `libopenapi-validator`, mirroring the engine's runtime `--oas-validate` path. Phases B (the AAT
project files) and C (e2e test, CI job, goreleaser, docs) follow; the full design and the findings
that shaped it are in `LAUNCH-PLAN.md` under M1.

**Decisions:**

- **Two base workflows instead of one.** After an `expectFailure` step passes, later steps still
  run, so a declined-card overlay on a workflow that ends at `shipOrder` would fail at the ship
  step. `Quick Purchase` (five steps, goal `paymentCharge`) backs `smoke` and the overlay demo;
  `Checkout` carries the slots, addons, and verification and ends at `shipOrder`.
- **Region lives in the OpenAPI `servers[].variables`, not in the paths.** The static graph↔spec
  validator flags any required OAS parameter missing from a node's inputs, and `--strict` makes
  that fatal; a `{region}` path parameter would have failed every node. Request and response
  bodies are flat for the same reason: the validator only sees top-level property names.
- **Chaos keyed per bearer token.** A batch shares one `AuthProvider` token, so "once per reset"
  would fire only for the first plan in a batch. Keying the `SKU-1004` stale read by token gives a
  standalone run exactly one `response_error` retry and keeps batches green. The shipment
  warm-up (503 twice) is keyed by shipment ID and is therefore deterministic per run. The engine
  does not honor `Retry-After`; the sandbox sends it anyway for realism.
- **`paymentRefund` is keyed by `orderId`.** Workflow-compat checking only sees a base template's
  own steps, so an addon `AUTOWIRE`ing `paymentId` from the payment slot would need explicit
  wiring in every base. Refund-by-order removes the dependency.
- **`shipOrder` returns a Shipment** (`status: in_transit`, `orderStatus: shipped`) rather than an
  order, so `getShipment`/`deliverShipment` share one schema; `addItem`/`applyCoupon` return the
  full cart so one `Cart` schema covers the cart operations.
- **Gift card balances are honest.** `GC-10-DEMO` = $10, `GC-100-DEMO` = $100, `GC-500-DEMO` =
  $500; the default one-item order is $103.40, so the Phase B Gift Card slot uses `GC-500-DEMO`.
- **`--seed` defaults to 1** (deterministic tokens and tracking numbers); `0` means time-based.
- **Embed patterns are explicit.** `all:examples/shop` would sweep a local `_output/`; the root
  `embed.go` lists each tracked subtree and names dotfiles individually. `examples/shop/.gitignore`
  must re-include `.mcp.json`, which the root ignore file excludes everywhere.
- **Contract tests map served URLs to spec paths by literal-segment matching** rather than relying
  on `http.Request.Pattern`: nested `ServeMux`es hand each handler a request copy, so the outer
  test middleware never sees the inner pattern.

**Open questions (for Phase B, answered in the entry below):**

- Whether layer values pass through the date-expression evaluator (`{{today + 3 days}}`); fall
  back to literal dates if not.
- `aat validate` reconstitutes recipes without a layers dir; Phase B adds `WithLayersDir` and a
  "Layers" section so recipe-embedded layers are validated.
- `aat generate --output-graph -` still writes `--output-templates`; the acceptance command must
  point it at a temp dir.

## 2026-09-10 — Phase A review and the engine bugs Phase B would have worked around

**What:** Reviewed the Phase A commit and designed Phase B against the engine. Before writing any
project YAML, fixed the bugs the shop demo would otherwise have routed around, plus the review
findings on the sandbox itself.

Engine and CLI:

- **Value-only overrides no longer reroute.** An overlay entry with only `values:`/`expectFailure:`
  registered a route with the top-level base URL and auth, and an exact match beats a glob, so
  `match: paymentCharge` pulled payments off the `payment*` route. `config.ResolvedOverride.Routes`
  now records whether an entry sets `baseUrl`, `auth`, `headers`, or `pathRewrite`; the new
  `engine.ExecutorRouter.AddResolvedOverride` registers a route only then. The CLI, `aat prompt`,
  and MCP `execute_plan` share it (MCP previously ignored override values entirely).
- **Override auth no longer leaks the inherited credential.** An override with its own `auth` kept
  the top-level `Authorization` header, so the shop bearer token went to the payments host. The
  inherited header (or top-level API key header) is dropped first; explicit override `headers`
  survive `auth: {type: none}`.
- **`extends`/`include` override order.** `mergeOverrideSlices` still put child entries first, which
  M0's last-match-wins routing (P3) turned into "parent wins". Child entries now append.
- **Layers are validated and never silently dropped.** `aat validate` gains a Layers section (parse
  errors, duplicate names, keys matching no node input) and reconstitutes recipes with the layers
  directory; `aat validate plan` and the MCP plan tools do the same. Naming layers without a layers
  directory is now an error in `run plan`, `run batch`, validate, and MCP instead of a different test.
- **Status defaults.** `status` assertions accept a class (`2xx`); recipe post-processing injects
  `2xx` rather than `200` (the sandbox, like most REST APIs, answers 201/204) and nothing on
  `expectFailure` steps; the engine reports status assertions on `expectFailure` steps as skipped,
  which is what lets an overlay turn a recipe step into a negative test.
- `aat plan list` parses recipes (every recipe, petstore's included, showed `(parse error)`).
- `--verbose-auth` redacts `access_token`/`refresh_token`/`id_token` in the logged token response;
  the old 20-character preview printed the sandbox's 18-character tokens whole.

Sandbox (Phase A review):

- Root `.gitignore` anchors `/.mcp.json`, so examples can ship one without a negation rule.
- `embed_test.go` asserts that `ShopExampleFS` embeds exactly the files under `examples/shop`.
- `openapi.yaml` inlines the payments Server Object; `$ref` is not allowed under `servers`.
- Refunds: a non-positive `amount` is 400; omitting it refunds the remaining balance; partial
  refunds leave the payment `partially_refunded`.
- `addItem` returns 409 `OUT_OF_STOCK` when the cart would hold more than is available.
- `aat-sandbox serve --host` defaults to `127.0.0.1` (credentials are printed and `/admin/reset` is
  unauthenticated); pass `--host 0.0.0.0` for Docker.

**Decisions:**

- Class syntax over dropping the default assertion: a visible `status 201 is 2xx` result in the
  archive is more useful than no assertion, and `4xx` is handy in hand-written plans.
- Skipping status assertions under `expectFailure` rather than rejecting them at runtime: the
  plan validator still rejects a contradiction written into the plan itself; an overlay applied at
  run time cannot be validated in advance.
- Stock stays in the read-only catalog: nothing decrements it, so the per-region copy considered
  during review would add state without behavior.

**Open questions:** none new; verified-but-deferred engine issues are listed under "Follow-ups
found during M1" in `LAUNCH-PLAN.md`.

## 2026-09-10 — Phase B: the `examples/shop` project

**What:** The AAT project for the sandbox: manifest, 17-node graph, 17 templates, domain, Quick Purchase
and Checkout workflows (customer and payment slots, four addons), 12 layers, 7 plans, the declined-card
overlay, a receipt visualizer, `us`/`eu` environments, MCP configuration, and a README. `aat validate
--strict` is clean; the plans pass 7/7 with `--oas-validate strict` in both regions; the two-group matrix
runs 63 permutations with 36 deduplicated.

**Decisions:**

- **Names come from the contract.** Nodes are named by operationId and outputs by response property
  (`status`, not `cartStatus`): the static validator warns on output names the 2xx schema lacks, and
  `--strict` turns warnings into failures. The Lua join in `getCart` therefore enriches `lines` in place
  instead of adding an `items` output.
- **Data flow lives in the graph.** `cartId`, `orderId`, `amount`, `currency`, and `shipmentId` default
  to `from:` references, and `addItem.sku` selects the first in-stock product. Plans that skip
  `listProducts` pin `sku`, because a graph default that references a missing step fails validation.
- **AUTOWIRE only where it shows something.** Apply Coupon wires `cartId` explicitly (the cart comes
  from a slot, which compatibility checks cannot see) and Track Shipment/Return After Delivery
  AUTOWIRE `shipmentId`; `orderId` for return and refund comes from graph defaults.
- **The negative state machine is region-neutral.** It uses an unknown coupon (404) rather than
  `EU-ONLY` in us (422), because the CI job also runs every plan with `--env eu`, where `EU-ONLY` succeeds.
- **Mutations assume nothing about order.** The over-stock mutation asks for 43 of a 42-unit SKU, which
  fails whether or not the parent step's unit is already in the cart.
- **Postal codes come from the environment.** `checkoutCart.postalCode` defaults to `{{env.postalCode}}`,
  and each environment sets it through `vars` and `values`.
- **Two more root causes, not workarounds.** The validator, scaffolder, and MCP operation details read only
  operation-level OAS parameters, so every path with a shared `{cartId}` warned; they now merge path-item
  parameters. Successful retries were invisible (the engine discarded the failed attempt's category), so
  the resilience demo looked like it never retried; steps now carry `RetriedOn`, archives `retriedOn`, and
  run output `retried Nx: <category>`.

**Open questions:** none for Phase B. Phase C adds the Go e2e test, the CI job, the goreleaser build, and
the root README/CLAUDE.md quick start. The web UI visual pass (timeline bars, matrix, receipt tab) and a
Claude Code MCP session are author checks.

## 2026-09-10 — Review of Phases A and B: root causes the example worked around

**What:** Before Phase C, reviewed the three committed M1 commits and fixed six issues where they
originate rather than in the example; a seventh, a data race, surfaced once the Phase C end-to-end test ran.
Each fix has tests:

- **Addon compatibility ignored slots.** `intent.ValidateWorkflowCompat` checked every addon against the
  raw base template with its slot markers unfilled, while `Compose` fills slots before it splices addons.
  An input now counts as fed when the base, or every option of one of its slots, produces it; an addon
  whose `after:` node comes from a slot option is checked instead of skipped; slot options are no longer
  checked as bases of their own. The tests had re-implemented the function in memory and now call it.
- **Slot and addon `verification:` was never composed.** Composition merged cleanup only, so the Checkout
  base had to accept `shipped || returned`. `mergeVerification` merges option and addon verification, and
  a node they verify replaces the base's check of that node.
- **The static OAS output check compared names, not paths.** Rule 7 wanted a top-level response property
  named after each output and ignored the template's extract path. With templates loaded (`aat validate`,
  `aat validate graph --templates`) it walks the 2xx schema along the extract path.
- **`--dump-state -` without `--json` mixed progress and the summary line into stdout,** which running.md
  said it did not. Stdout now carries only the state.
- **The silent layer drop lived in `intent.Reconstitute`,** but the previous commit guarded three of its
  callers. `Reconstitute` now returns the error itself, and `graph.LayeredDefaults` replaces the two
  copies of load-and-apply in `cmd/aat` and `mcp`.
- **`settings.oasValidation` and `--oas-validate` accepted any string;** a typo silently meant `auto`.
- **Parallel batches raced during runtime OAS validation** (found once the Phase C end-to-end test ran under
  `-race`). `aat run batch --parallel N` shares one `oas.SpecCache` entry across runs, and
  libopenapi-validator v0.13.1 writes into the schema model while it renders a response schema. Per its
  source it renders on every validation for a schema behind a `$ref`: the warm cache is stored under the
  media type's schema hash but looked up under the resolved schema's hash. The shop spec declares every
  response that way. Validations against one spec entry are now serialized; a `graph/oas` test with a
  `$ref` response schema fails under `-race` without the lock (the same test with an inline schema never
  raced, which is how the cache miss was confirmed).

Example changes: Apply Coupon drops its `wire:`; Checkout verifies `shipped`; Return After Delivery
verifies `returned` and `refunded`; `registered-paypal-coupon` adds Return After Delivery so that path runs
in every batch; `env.yaml` drops `oasValidation: warn`, which behaved like the default. `validate --strict`
stays clean, both regions pass 7/7 with strict OAS checks, and the matrix is unchanged (63 runs, 36 skipped,
27 passed).

**Decisions:**

- **The Apply Coupon `wire:` was redundant, not a workaround.** Checkout's own `addItem` and `getCart`
  produce `cartId`, so the unfixed checker never flagged it; a copy of the example without the `wire:`
  validated and ran green on the Phase B binary. The compat bug shaped only the stated reason for keying
  refunds by order.
- **`paymentRefund` stays keyed by `orderId`** and the sandbox bodies stay flat. Both are reasonable API
  shapes; reshaping the contract would churn the example for no demo value.
- **Verification merges replace by node.** The synthetic `verify_<node>` IDs would collide otherwise, and
  the later composition knows the plan's end state. All of a sub-workflow's entries for a node replace all
  of the base's entries for it.
- **The path-aware output check is conservative.** It keeps the old gate (a 2xx schema that declares
  properties), and schemas without declared properties, `additionalProperties`, composition branches, and
  GJSON queries or modifiers all count as present, so it reports only definite mismatches. Outputs a Lua
  transform computes are skipped. `graph/oas` still imports nothing from `adapter`: `engine` builds the
  path map and passes it in.
- **Template placeholder escaping is its own item** (P13, needed before M5): bodies get no JSON escaping
  and paths no URL encoding, which the M7 MCP demo would expose.
- **Serialize validations per spec instead of loading the spec per run.** A mutex on the cache entry is
  correct whatever libopenapi does internally, and a validation takes milliseconds, so parallel runs lose
  little. The upstream cache miss is logged as a follow-up.

**Corrections to earlier entries:** "compat checking only sees base steps" and "verification survives only
in the base template" described bugs that are now fixed; bodies no longer need to be flat for `--strict`.
