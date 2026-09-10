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

**Open questions (for Phase B):**

- Whether layer values pass through the date-expression evaluator (`{{today + 3 days}}`); fall
  back to literal dates if not.
- `aat validate` reconstitutes recipes without a layers dir; Phase B adds `WithLayersDir` and a
  "Layers" section so recipe-embedded layers are validated.
- `aat generate --output-graph -` still writes `--output-templates`; the acceptance command must
  point it at a temp dir.
