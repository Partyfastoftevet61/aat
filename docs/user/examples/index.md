# Examples

Every example is the same idea at a different scale: a graph precise enough to run, and the things that read it. Three complete projects ship in the repository. CI checks each with `aat validate --strict`, and runs the shop's and the gRPC project's plans against the sandbox.

| Example | API | What it shows | Needs |
|---------|-----|---------------|-------|
| [Shop](shop.md) | An offline e-commerce API served by `aat-sandbox` | Everything: an 18-operation graph, workflows with slots and addons, a Lua transform, layers and batch matrices with dedup, `us`/`eu` environments, a separately hosted payments API, negative tests and mutations, retries, checkpoints, a visualizer, MCP configuration, and an [integration kit](../integration-kit.md) packaged from the project | Nothing — no network, no account |
| [Petstore](../petstore-walkthrough.md) | The public Swagger Petstore | The smallest working project: four operations, two workflows, two recipes, and graph cleanup pairing | Network access |
| [gRPC payments](../grpc.md#the-60-second-version) | The same `aat-sandbox`, over gRPC and HTTP | One plan across two protocols: a cart opened and checked out over HTTP, then charged and refunded over [gRPC](../grpc.md), with the order id crossing the boundary untouched | Nothing — no network, no account |

Three more projects run against real, public APIs and live in their own repositories; [Real APIs](real-apis.md) describes each. Each was built against the API's live test mode, and every claim in its README is something a run recorded:

| Project | API | Scale | What it shows |
|---------|-----|-------|---------------|
| [aat-duffel](https://github.com/gburgyan/aat-duffel) | Duffel flights, test mode | 66 operations, 47 plans, 14 layers; 47/47 in ~3½ min | An API with **no official OpenAPI spec** — everything in the README came from runs |
| [aat-stripe](https://github.com/gburgyan/aat-stripe) | Stripe, test mode | 82 operations, 53 plans, 14 layers; 53/53 in ~5 min | ~6,300 lines of graph and templates against a **205,000-line** vendored spec, with every exchange checked against it |
| [aat-shippo](https://github.com/gburgyan/aat-shippo) | Shippo shipping, test mode | 46 of 70 operations, 28 plans, 9 layers; 28/28 in ~2½ min | Layers as the headline — a lane × parcel matrix and six deterministic tracking fixtures — with real shipping labels rendered in the web UI |

Each needs a free test-mode account and its token; the README of each says which.

The [Airline case study](airline-case-study.md) describes the project AAT was built for — a private 74-node airline booking API with 63 workflows, 53 recipes, and 6 environments — and which features that scale relies on.

To build a project of your own step by step, follow the [Tutorial](../tutorial.md), which recreates a smaller version of the shop by hand.
