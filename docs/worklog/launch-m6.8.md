# M6.8 — the `aat-shippo` package

The third and last example package, and the first built green-field. Its three audiences are a
running test suite for Shippo, a worked MCP example, and launch marketing that leads on layers.

## 2026-09-15 — Phase 1: foundation, and three AAT gaps the package found

**What:**

- **PR #28 merged** (`valuePrefix` on apikey auth), so an environment can write
  `Authorization: ShippoToken <token>` without the scheme becoming part of the secret. Verified live
  on the first request.
- **Three more AAT gaps, each found by building the package, each its own PR:**
  - **#29, merged.** A block key may end in `[]`. Block tags matched letters, digits, underscores
    and hyphens only, so a conditional keyed on an input named for an array query parameter was
    never recognized: `aat validate --strict` reported the placeholder inside as required, and a
    request failed with `unresolved placeholders: ?status[], /status[]`. `aat generate` writes
    exactly that shape, so **its own output failed validation** for any spec with such a parameter —
    Shippo's `order_status[]` on `GET /orders` is how it surfaced.
  - **#30, merged.** The static OAS check now knows the inputs of a `oneOf` or `anyOf` request body.
    It read a body's own properties and its `allOf` only, so for a composed body it found none and
    reported every input of the node. `aat generate` declines to write such a body and says so, so
    the operations it hands over were exactly the ones that then warned — six in Shippo's spec.
  - **#31, open.** An input sent as a JSON body property the spec names otherwise now counts as that
    property, as one sent as a form field, a query parameter, or a path segment already did. Until
    now only form-bodied APIs could name inputs for the package rather than the spec.
- **The package's Phase 1 landed** (`aat-shippo` d512b66): manifest, environments, domain, twelve
  nodes with their templates, eight plans, the live-label guard, and a drift plan. `aat validate
  --strict` clean; 8/8 plans pass on `test-ci`; the staged-diff token scan counts 0.
- **The carrier × lane coverage map** (`docs/carrier-lane-coverage.md`) was probed live and
  committed: which of the fifteen test carrier accounts answer on which of ten lanes, with the
  reason each silent one gives, in Shippo's own words.

**Decisions:**

- **`aat generate` is the accelerator, not the deliverable.** The whole spec scaffolds in one command
  — 70 nodes, 70 templates, a 3,681-line graph, `aat validate --strict` clean, six bodies flagged for
  hand-writing. But the generated graph carries spec prose (one input holds a 60-row carrier table)
  where the sibling packages carry what runs proved. Each family is generated into a scratch
  directory and curated in by hand, in the `aat-duffel`/`aat-stripe` house style. The author chose
  this over committing the scaffold.
- **`oasValidation: auto` is the default, the reverse of the Stripe package.** Shippo's spec marks
  nothing nullable and the API sends null constantly — `next` and `previous` on every list,
  `is_residential`, `latitude`, `longitude` on an address, `template` on a parcel. Under `strict`
  every list read fails for a reason that is the spec's. `test-strict` exists to show it, and
  `drift/nullable-pagination.yaml` is run both ways: it passes under `test` and fails under
  `test-strict` with `$.next: got null, want string`.
- **No `incompatibleWith` layer primitive.** The M6.8 plan held it as a candidate for Phase 9. AAT
  has no such key, and a layer group already picks at most one of its options, so a per-region group
  is the mechanism. Dropped unless Phase 9 finds something a group cannot express.
- **Australia is not a lane.** `couriersplease` is the only carrier with an AU service area and it
  answers nothing, even once the company name it asks for is supplied. FedEx, Canada Post and LSO
  are unusable too, for reasons the API states; ten of the fifteen accounts remain.
- **The parcel is a layer axis, not a constant.** It changes *which carriers answer*, not just the
  price: a 15 g letter reaches Deutsche Post where a 2 lb box reaches only DPD DE, and a 30 cm box
  reaches Chronopost where a 20 cm one does not. Proven on two independent lanes.
- **Correction to the M6.8 design:** `object_created_gte` is declared on `ListShipments` and
  `ListTransactions` only, not on every list endpoint. Both guards page transactions, so the guard
  design holds, but the README must not generalise it.

**Open questions:**

- Whether `minRequestInterval: 1200ms` absorbs the ten-list-reads-a-minute limit across a full
  batch. Phase 1's eight plans never hit a 429, but they are small. If a larger batch does, per-method
  pacing becomes the next PR. Observed separately: UPS rate-limits *itself* during a fast sweep and
  simply returns no rates, which is why a rating plan must not assert that a given carrier is present
  unless it pins that carrier.

## 2026-09-15 — Phases 2 and 3: rating and labels

**What:**

- **Phase 2, rating** (`aat-shippo` 191939a): six nodes, three plans, and the `rates` visualizer,
  which draws the carrier × service table with the carriers' own logos and the CHEAPEST / FASTEST /
  BESTVALUE chips. The same file serves a rates listing, a currency-converted listing, and the rates
  a shipment carries inline — and for a shipment it also lists the reason every silent carrier gave.
- **Phase 3, labels** (`aat-shippo` 75054b7): six nodes, three plans, the `label` visualizer, and
  the second guard. The visualizer draws a PNG label as the label itself: the real USPS sheet with
  its barcode and QR code, rendered in the web UI, because the visualizer CSP allows `img-src
  https:`. A PDF or ZPL gets a link instead.
- **PR #32 opened:** an input can name a property nested inside the request body. Follows #31.
- 15 plans, `aat validate --strict` clean, 15/15 green on `test-ci`, 0 cleanup failures.

**Decisions:**

- **Every label node is paired with `createRefund` under `when: status == "SUCCESS"`.** A label
  cannot be deleted, so cleanup is a refund. The batch proves the pairing: three plans buy four
  labels between them and the account ends with none of them bought. When a plan refunds explicitly,
  AAT reports the pairing as `skipped: released by refund`.
- **Both guards assert `ourCount > 0`,** so neither can pass by finding nothing.
  `no-unrefunded-labels` checks that no label the package bought is still in `SUCCESS`, since a
  refunded one reads `REFUNDPENDING` and then `REFUNDED`.
- **Label plans pin USPS rather than buying the cheapest rate.** Rating and buying are separate
  permissions on this account, and pinning is the honest way to say so in the plan.
- **A rating plan never asserts that a named carrier answered** unless the shipment pinned that
  carrier, because a carrier's absence is normal and explained in `messages`.

**What the phases found, each now pinned by a plan:**

- **A 201 does not mean a label was bought.** Buying a UPS rate answers `201` with
  `status: ERROR` and `ups_registration_error`; the account can rate with UPS but not purchase. The
  status, not the HTTP code, is the evidence.
- **Shippo does not echo `label_file_type`.** The format delivered is readable only from the label
  URL's extension, so the graph exposes `labelFormat`, parsed from the URL, rather than the type
  that was asked for.
- **A refund does not settle quickly in test mode** — one stayed `PENDING` for at least 90 seconds,
  which is what a carrier taking days to accept a refund looks like through an API. The label's own
  status becomes `REFUNDPENDING` at once, so that is what a plan waits for.
- **`GET /refunds/` needs its trailing slash,** and the spec declares no `page` or `results`
  parameters for it, alone among the listings.
- **Rating is synchronous in test mode** despite the documented async default: every lane probed
  answered `SUCCESS` with its rates already attached. The poll stays because it is what the API asks
  for and a slower lane would need it.
- **`GET /carrier_accounts/{id}` omits `service_levels` entirely;** the listing with
  `service_levels=true` is the only source.
- **An address created with `validate: true` is corrected and loses its `metadata`,** so a validated
  address carries no tag.

**Open questions:**

- Whether other carriers besides USPS can buy. Only USPS was needed for Phases 1–3; the answer
  shapes the per-region purchase matrix in Phase 9, and the coverage map will need a *can rate* /
  *can buy* distinction.

## 2026-09-15 — Layers and the README, pulled forward (author request)

**What:**

- **The parcel layer axis and a matrix** (`aat-shippo` f947d30), pulled forward from Phase 9 because the
  parcel is the finding the package leads on and it needs no graph restructuring: every parcel layer
  sets inputs of `createParcel` and nothing else. Two matrix plans pin their lane and say nothing about
  the parcel. `createParcel`'s dimensions gained defaults so the no-layer permutation is a real parcel.
- **The README and its assets** (`aat-shippo` b90c0b5), with `demos/` — three VHS tapes, a Playwright
  screenshot script, and `demos/run.sh` that regenerates all of it against the live test API. 1.3 MB of
  images: the hero label run, the matrix, `aat validate --strict`, the rendered label, the rates
  visualizer with its refusal panel, and the run timeline.
- **PR #32 merged.** All five AAT PRs from this milestone are in.

**Decisions:**

- **The README does three jobs, because the files do:** a test suite Shippo could run, a Rosetta stone
  for anyone integrating with Shippo, and a demonstration of AAT. The author framed it that way and it
  turned out to need no compromise — the same graph, templates and plans serve all three.
- **The matrix table is the lead**, not the feature list. `parcel-large` gains France Chronopost;
  `parcel-letter` gains Germany Deutsche Post at 1.10 EUR against 4.85 EUR by parcel. One axis, two
  lanes, opposite directions, one command.
- **The upstream PRs are in the README.** Three of the five were only findable by pointing the tool at a
  real API and running what came out — including `aat generate` producing a project its own validator
  rejected. That is the dogfooding claim with evidence attached.
- **Every asset is regenerated by a committed script.** `demos/run.sh` needs only a test token.
- **A lane layer axis is deferred** to the layer phase: it needs the from and to addresses as separate
  graph nodes, which is a restructuring the parcel axis did not.

**Open questions:**

- The README will need a pass per phase as tracking, customs and batches land; its *Not covered yet*
  section is the list to work down.
