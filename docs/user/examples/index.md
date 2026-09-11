# Examples

Two complete projects ship in the repository. CI checks both with `aat validate --strict`, and runs the shop's plans against the sandbox.

| Example | API | What it shows | Needs |
|---------|-----|---------------|-------|
| [Shop](shop.md) | An offline e-commerce API served by `aat-sandbox` | Everything: a 17-operation graph, workflows with slots and addons, a Lua transform, layers and batch matrices with dedup, `us`/`eu` environments, a separately hosted payments API, negative tests and mutations, retries, checkpoints, a visualizer, MCP configuration, and an [integration kit](../integration-kit.md) packaged from the project | Nothing — no network, no account |
| [Petstore](../petstore-walkthrough.md) | The public Swagger Petstore | The smallest working project: four operations, two workflows, two recipes, and graph cleanup pairing | Network access |

Examples against real APIs (Duffel flight booking, GitHub, and Stripe) are planned; see the [roadmap](https://github.com/gburgyan/aat/blob/main/ROADMAP.md).

The [Airline case study](airline-case-study.md) describes the private project AAT was built for — 74 operations and 53 recipes — and which features that scale relies on.

To build a project of your own step by step, follow the [Tutorial](../tutorial.md), which recreates a smaller version of the shop by hand.
