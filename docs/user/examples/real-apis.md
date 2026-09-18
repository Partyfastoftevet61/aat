# Real APIs: Duffel, Stripe, Shippo, and Qdrant

Four projects run AAT against real, public APIs. Three run in an API's test mode, against your own free test account; the fourth runs against the database itself, a pinned Qdrant in a local container. Each is a complete AAT project — a manifest, a graph, one template per operation, plans, layers, environments — in its own repository, which you clone and run. Every claim in their READMEs is something a run recorded. They are the proof behind this site: the same files that test an API describe it well enough for an AI coding tool to integrate with it.

The numbers count the same things for all four: the operations the graph covers, the plans, the layers, and the full batch's result and time.

| Project | API | Scale |
|---|---|---|
| [aat-duffel](https://github.com/gburgyan/aat-duffel) | Duffel flights, test mode | 66 operations, 47 plans, 14 layers; 47/47 in ~3½ min |
| [aat-stripe](https://github.com/gburgyan/aat-stripe) | Stripe payments, test mode | 82 operations, 53 plans, 14 layers; 53/53 in ~5 min |
| [aat-shippo](https://github.com/gburgyan/aat-shippo) | Shippo shipping, test mode | 46 of 70 operations, 28 plans, 9 layers; 28/28 in ~2½ min |
| [aat-qdrant](https://github.com/gburgyan/aat-qdrant) | Qdrant vector database over gRPC, local container | all 52 public unary gRPC methods as 77 operations, 38 plans, 6 layers; 38/38 in ~70 s |

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

## aat-qdrant: a vector database, over gRPC

Qdrant stores vectors — the lists of numbers an embedding model turns a document or an image into — and answers which stored items are nearest to a given one: what semantic search, recommendations, and retrieval for LLM prompts run on. It has no test mode and needs none: the project runs against Qdrant v1.19.1 in a local container, pinned by image digest, and nothing outlives the container.

The project covers all 52 public unary gRPC methods Qdrant serves, as 77 operations with 38 plans and 6 layers; the full batch passes 38/38 in about 70 seconds, 60 of them one deliberate wait for a rate limit's `retry-after`. It is the one project here that speaks [gRPC](../grpc.md), and it was built to stress-test AAT's support for it against protobuf nobody on the AAT side wrote: oneofs for ids, vectors, and query kinds, payloads that are `map<string, Value>`, 64-bit counts that arrive as strings, and a pagination cursor that is a message. Every node is checked offline against Qdrant's 17 published `.proto` files, vendored unchanged and compiled to a descriptor set. Each expected failure names its gRPC status and the exact message, which is how the plans can say that a read-only key on a write is `PERMISSION_DENIED` where a missing key is `UNAUTHENTICATED`, and that a rate-limited read puts `retry-after` in the trailers, not the headers. A few nodes read the same data back over REST, checked against Qdrant's OpenAPI spec, and one plan outside the batch pins an endpoint the v1.19 spec dropped and the server still answers. Layers cross the four distance metrics with both kinds of point id. Twelve changes to AAT came out of building it, each made as the gap turned up.

A team building on Qdrant clones it for working requests in proto3 JSON, the text `grpcurl -d` takes, and for what the protos don't say: which zero values must still be sent, and what each refusal says. A team with a gRPC API of its own reads it as the worked example; its README maps each gRPC concern — the contract, oneofs, maps, errors, auth as metadata, retry hints, pagination — to how the project handles it and what carries over. It needs Docker and aat 0.3.0 or later, the first release with gRPC. Not covered: Qdrant Cloud, the internal and streaming services, snapshot download and recovery (REST-only), cloud inference, and clusters of more than one node.

![A rate-limited read in the web UI: the gRPC status RESOURCE_EXHAUSTED with Qdrant's message, and retry-after: 60 among the trailers](https://raw.githubusercontent.com/gburgyan/aat-qdrant/main/docs/images/ui-grpc-refusal.png)

Three smaller projects ship in this repository and need no account: the [shop](shop.md) and [gRPC payments](../grpc.md#the-60-second-version), which run offline against `aat-sandbox`, and the [petstore](../petstore-walkthrough.md).
