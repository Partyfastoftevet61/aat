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
