# Real APIs: Duffel, Stripe, and Shippo

Three projects run AAT against real, public APIs in their test modes. Each is a complete AAT project — a manifest, a graph, one template per operation, plans, layers, environments — in its own repository, which you clone and run against your own free test account. Every claim in their READMEs is something a run recorded. They are the proof behind this site: the same files that test an API describe it well enough for an AI coding tool to integrate with it.

The numbers count the same things for all three: the operations the graph covers, the plans, the layers, and the full batch's result and time.

| Project | API | Scale |
|---|---|---|
| [aat-duffel](https://github.com/gburgyan/aat-duffel) | Duffel flights, test mode | 66 operations, 47 plans, 14 layers; 47/47 in ~3½ min |
| [aat-stripe](https://github.com/gburgyan/aat-stripe) | Stripe payments, test mode | 82 operations, 53 plans, 14 layers; 53/53 in ~5 min |
| [aat-shippo](https://github.com/gburgyan/aat-shippo) | Shippo shipping, test mode | 46 of 70 operations, 28 plans, 9 layers; 28/28 in ~2½ min |

## aat-duffel: flights

Duffel sells flights from many airlines through one API: search, price, book, add seats and bags, change, cancel. Its test mode books nothing real.

The project covers 66 operations with 47 plans and 14 layers; the full batch passes 47/47 in about three and a half minutes. Duffel publishes no official OpenAPI spec, so the graph is the machine-readable description of the API, and everything the README says about Duffel came from runs. Layers vary the search on five axes — area, cabin, lead time, round trip, party — and a `--layer-group` batch crosses them, skipping any permutation that equals the default.

A team building a travel product on Duffel clones it, exports a test token, and has a search-to-booking suite and a reference for the call order on day one. Not covered: Stays, Cars, and card payments, which answer 403 on the account it was built against, and the features Duffel has deprecated or closed.

![The Order tab in the web UI: a family of four with an infant, their seats on the flight, and the seats and bags bought](https://raw.githubusercontent.com/gburgyan/aat-duffel/main/docs/images/order.png)

## aat-stripe: payments

Stripe moves money: customers, card payments, saved cards, ACH and SEPA debits, bank transfers into a cash balance, refunds. Its test mode moves none.

The project covers 82 operations with 53 plans and 14 layers; the full batch passes 53/53 in about five minutes, and every request and response is checked against Stripe's own 205,000-line spec as it runs. Where Stripe's API and Stripe's spec disagree, a plan records it. Each expected failure asserts the exact `code`, `decline_code`, and `param` Stripe answers with. Six small changes to AAT came out of building it, none Stripe-specific.

Anyone integrating Stripe can read the graph and `domain.yaml` as a reference whose every claim has a run behind it, or point the plans at their own test key and let a rerun say what changed. Not covered: Treasury, Sigma, and the legacy aliases; risk, Billing, Checkout, Connect, Terminal, Issuing, and Tax are still to come.

![The card brand matrix batch in the web UI's By Test view: one row per plan, one column per layer permutation](https://raw.githubusercontent.com/gburgyan/aat-stripe/main/docs/images/ui-batch-matrix.png)

## aat-shippo: shipping

Shippo rates a shipment across carriers, buys the label, and tracks the package. Its test mode buys real USPS labels that cost nothing.

The project covers 46 of Shippo's 70 operations with 28 plans and 9 layers; the full batch passes 28/28 in about two and a half minutes. Layers are the headline: a lane × parcel matrix and Shippo's six deterministic tracking fixtures, each from two plan files, each deduplicated, one command apiece ([why that matters](../why.md#the-proof-is-that-it-runs)). Every response is checked against Shippo's published spec, which reports rather than fails, because the spec is what is wrong. Five fixes to AAT came from it, three of them findable only by running `aat generate` on a real spec.

A shipping team clones it for the call order Shippo's docs don't give: rate, buy, refund, validate an address, track. Shippo's own MCP server lets an assistant ship a package for you; this project lets an assistant build and keep your integration. Not covered: customs, international lanes end to end, batches, manifests, pickups, and orders.

![A purchased label rendered in the web UI, with its tracking number and carrier link](https://raw.githubusercontent.com/gburgyan/aat-shippo/main/docs/images/ui-label.png)

Two smaller projects ship in this repository and need no account: the [shop](shop.md), which runs offline against `aat-sandbox`, and the [petstore](../petstore-walkthrough.md).
