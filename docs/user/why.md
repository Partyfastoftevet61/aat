# Why AAT Exists

AAT started as a pile of Postman collections.

They worked, at first. Then the team grew. Everyone had their own copy with their own tweaks, and none of them were reliable. Nothing was in source control, so there was no diff, no review, and no way to tell whose version was right. Every new test case meant editing a collection in place, so the case it replaced was gone. The chaining lived in pre-request scripts, which put the interesting part of a flow — what depends on what — inside JavaScript instead of in front of you. And none of it was legible to an AI coding tool: the export was one file too large to read, in a shape nothing else consumes.

The underlying problem is that real integrations are not one call. Buying something means browse, cart, checkout, pay, ship, and maybe return and refund: 8 to 20 calls, each needing IDs from the calls before it, leaving state behind that someone has to clean up. A collection is a folder of single requests. Everything that makes those requests a *flow* has to live somewhere else, and that somewhere was scripts.

Postman is good at what it is for: exploring an API by hand, one request at a time. It is a poor place to *keep* the knowledge of how an API works. That knowledge ends up in a format only Postman reads, in a workspace rather than your repository, and it scales by copying.

So the knowledge moved into the repository. **API knowledge** is a [graph](graphs.md) of operations and [request templates](templates.md), written once. **Test intent** is a [plan](plans.md) that lists steps, not wiring. **Variation** is [layers](batch-layers.md) and [environments](environments.md) that turn one plan into a matrix. Small files, reviewed like code, that an AI coding tool can read one at a time and a person can follow without opening a debugger.

The point is not that the files are tidy. It is that they run.

## What changed, item by item

| What went wrong with a pile of collections | What AAT does instead |
|---|---|
| Everyone had a copy, with their own tweaks | One graph in the repository. Plans name steps, not wiring, so fixing an operation fixes every test that uses it |
| None of them were reliable | The project runs. [`aat validate --strict`](validation.md) catches broken wiring before a request is sent, [`--oas-validate`](running.md) checks every exchange against the spec, and the [archive](archives.md) keeps the request and the response |
| Every new case meant mutating the collection | [Layers](batch-layers.md) turn one plan into a matrix, and nothing is edited in place |
| No source control | One small YAML file per operation, reviewed in a pull request and diffed like code |
| An AI tool could not use it | A 21 MB export does not fit in a context window; a 23-line template does. The [MCP server](mcp-server.md) hands an assistant the flows as tools rather than prose |
| The scripting hid the lede | Wiring is [declared, not scripted](value-flow.md). The archive shows every value, where it came from, and every decision |

The two numbers in that last row are measured, not rhetorical. The private airline API this tool was built against ships a 40,000-line OpenAPI spec, and the team's Postman collections for it were 21 MB and 23 MB. The AAT project that replaced them is a 3,708-line graph covering 74 operations, with a median request template of 23 lines. An assistant — or a reviewer — loads the one operation the task needs, and edits one file.

## What else could read it

Once the API was described well enough for the engine to run it — typed operations, where each value comes from, what has to happen first, what undoes what — the description turned out to be worth more than the tests it was written for. The question stopped being *what else should this run?* and became *what else can read this?*

**AI coding tools.** [`aat mcp serve`](mcp-server.md) hands an assistant the same graph the engine runs: each operation's exact request, the order calls go in, what each one needs from the calls before it, the composed flows, and sample responses from real runs. It is a form a machine can act on, rather than prose it has to interpret. Package a subset as an [integration kit](integration-kit.md) and your integrators' assistants read it too — on a 74-node airline API, that was enough for a working client in a single prompt, in six languages.

**Evidence you can send someone.** Every run writes an [archive](archives.md): each request and response, how every input got its value, every retry, every assertion, and the cleanup, with secrets redacted. The [web UI](web-ui.md) exports a run as a single `.aar` file, and whoever you send it to opens it in the same viewer with `aat web view` or `aat import`. It is how you show that something works — or that it doesn't — with the actual exchange instead of a screenshot of one pane, and it is the difference between "the sandbox rejects this" and a file the other team can open and step through.

**Reference documentation.** [`aat docs generate`](docs-generate.md) writes Markdown for every operation from the graph, so the description that runs the tests is also the page someone reads.

**Whatever comes next in your pipeline.** [`--stop-after`](checkpoints.md) stops a run at a named step and leaves the resources it created alive; `--dump-state` writes their IDs, base URLs, and headers for a pytest suite, a load test, or a `curl` session to pick up. For [CI](ci-cd.md) there are exit codes 0/1/2/130, `--json`, `--quiet`, JUnit XML, and a Docker image.

None of that was a roadmap. It is what one good description of an API turned out to be good for, and it is what the *toolkit* in the name means.

## It is legible both ways

The same property that makes the files fit an agent's context window makes them fit a reviewer's head: one operation per file, one plan per scenario, no hidden scripting between them. When you want detail rather than summary, the [web UI](web-ui.md) turns a run into a timeline of every step, with the resolved value behind every input, every retry and assertion, and Copy-as-cURL on any step.

## The proof is that it runs

Three complete projects against real, public APIs, each built openly and each run against the live test API. Every claim in their READMEs is something a run recorded.

| Project | Scale | What it shows |
|---|---|---|
| [aat-duffel](https://github.com/gburgyan/aat-duffel) | 66 endpoints, 47 plans, 14 layers; the full batch passes 47/47 in about 3½ minutes | An API with **no official OpenAPI spec**. Everything the README says about Duffel came from runs |
| [aat-stripe](https://github.com/gburgyan/aat-stripe) | 82 operations, 53 plans, 14 layers; 53/53 in about 5 minutes | About 6,300 lines of graph and templates against Stripe's **205,000-line** vendored spec, with every request and response checked against it as it goes |
| [aat-shippo](https://github.com/gburgyan/aat-shippo) | 46 of 70 operations, 28 plans, 9 layers; 28/28 in about 2½ minutes | Layers as the headline — a lane × parcel matrix and six deterministic tracking fixtures — with real shipping labels rendered in the web UI |

`aat-shippo` makes the argument on this page in one command. Its lane × parcel matrix runs eight combinations from two plan files. A second matrix, over an axis those plans never read, expands to fourteen runs — seven execute, seven are skipped as duplicates, ten seconds, nothing bought. A layer only multiplies the plans it actually reaches. With collections, every one of those combinations is a copy you maintain by hand.

Two smaller projects ship in this repository and need no account at all: the [shop](examples/shop.md), which runs offline against `aat-sandbox`, and the [petstore](petstore-walkthrough.md). See [Examples](examples/index.md).

## If you already have API tooling

- **An OpenAPI spec** is the best starting point. [`aat generate --oas`](generate.md) scaffolds the graph and one template per operation — roughly the mechanical 70% — and you add the part a spec cannot describe: which calls reach a goal, in what order, and what undoes what.
- **A Postman collection** has no importer, and this page will not pretend otherwise. What works today is to point your AI coding assistant at the collection through a reader such as [expost](https://github.com/gburgyan/expost) and have it author the graph, then close the loop with `aat validate --strict` and a real run. The [AI assistant primer](llms.md) covers that workflow.
- **Nothing yet** is fine too. The [Tutorial](tutorial.md) builds a project by hand against the offline sandbox in about 45 minutes.

---

I built AAT because working with someone else's API should cost less than working around it.
