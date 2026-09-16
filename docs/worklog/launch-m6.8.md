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
