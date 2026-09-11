# Share Your API with Integrators

The project you build to test an API holds what anyone integrating with it needs. It records what every operation sends and returns, how one call's output feeds the next, and which sequences reach a goal, and your own test runs keep all of it honest. Package part of it as an **integration kit** and give it to the teams who integrate with you. Their AI coding tool learns your API from the kit through `aat mcp serve`. They can also run your reference flows against your sandbox and see real exchanges before their own client sends a request.

One framework pays off twice: first when you test your API, then when others integrate with it.

## What Goes in a Kit

A kit is a package you build from your project, typically in CI. It doesn't have to mirror the project; it only has to be cheap to rebuild.

| Ship | Keep internal |
|------|---------------|
| The graph, request templates, OpenAPI spec, and domain file | Negative, chaos, and regression plans |
| Workflows, which integrators see as integration flows | Layers and overlays |
| Reference plans for the main journeys | Environments with internal hosts or credentials |
| An environment for your public sandbox or test API, with credentials read from the integrator's environment variables | Run archives, visualizers, and `.aat-overrides.yaml` |
| A README for integrators, and per-node docs if you write them | |

## Layout: One Project, Two Manifests

Keep a single project. Put the suites you don't ship in a directory of their own, and add a second manifest that names what you do ship:

```
my-api-tests/
  aat-project.yaml     your manifest: everything
  aat-kit.yaml         the kit manifest: what integrators get
  graph.yaml  openapi.yaml  domain.yaml  env.yaml
  templates/  workflows/
  plans/               reference plans (shipped)
  internal/plans/      negative, resilience, regression (not shipped)
  layers/  overlays/  visualizers/
  KIT-README.md        becomes the kit's README.md
  package-kit.sh       builds the kit
```

Your manifest lists both plan directories. A plan's name is its path inside its directory, so `internal/plans/negative/state-machine.yaml` is still called `negative/state-machine`:

```yaml
# aat-project.yaml (excerpt)
plans: [plans/, internal/plans/]
layers: layers/
visualizers: visualizers/
```

The kit manifest uses the paths its files will have inside the kit, so packaging copies it unchanged as `aat-project.yaml`:

```yaml
# aat-kit.yaml
name: shop
description: Integration kit for the shop API
graph: graph.yaml
templates: templates/
domain: domain.yaml
workflows: workflows/
plans: plans/
environment: env.yaml
defaultEnvironment: us
archives: _output/runs
```

Before you package anything, `aat validate --strict --manifest aat-kit.yaml` checks the kit, and `aat mcp serve --manifest aat-kit.yaml --persona api` shows it the way an integrator's tool will see it.

### Environments You Don't Ship

Keep internal environments in a second file that includes the shipped one, and point your manifest at it with `environment: env.internal.yaml`:

```yaml
# env.internal.yaml
include: [env.yaml]
environments:
  staging:
    extends: _base
    vars:
      region: us
      postalCode: "78701"
      apiHost: shop.staging.internal
      payHost: pay.staging.internal
```

`staging` extends the `_base` environment from the included file. When both files define the same environment or `shared` key, the included file wins, so give internal environments names of their own. See [Environments: File Splitting with `include`](environments.md#file-splitting-with-include).

## Package It in CI

The shop's `package-kit.sh` is the whole packaging step:

```bash
sh package-kit.sh _output/shop-kit
```

It copies the files the kit manifest names into `_output/shop-kit/`, with `aat-kit.yaml` as `aat-project.yaml` and `KIT-README.md` as `README.md`. Then it writes `_output/shop-kit.tar.gz`. When the kit needs a new file, such as an environment include or a docs directory, add it to the script's copy line.

Check the package the way an integrator will use it, then publish it. In GitHub Actions, with your sandbox reachable from the job:

```yaml
- name: Package the integration kit
  run: sh package-kit.sh _output/shop-kit
- name: Validate and run the packaged kit
  working-directory: _output/shop-kit
  run: |
    aat validate --strict
    aat run batch --oas-validate strict
- uses: actions/upload-artifact@v7
  with:
    name: shop-kit
    path: _output/shop-kit.tar.gz
```

The reference plans run in your pipeline next to your internal suites. A change to the API that breaks an integration flow fails your build before an integrator runs into it.

## What an Integrator's AI Tool Sees

`aat mcp serve --persona api` is read-only. From the manifest it loads:

- the graph, and the workflow templates it names (resolved next to the graph file)
- every template in the templates directory
- the domain file, the OpenAPI specs, and the per-node docs directory
- `README.md` next to the graph file, served as `aat://readme`
- run archives, for `get_sample_response`
- the manifest's name, description, and tags

No `api` tool reads plan directories, layers, or overlays, and none exposes the environment file, although the server still loads it at startup when `environment:` is set. Everything else in those files can reach the integrator's tool. Keep internal-only templates out of the kit's templates directory, and internal notes out of its docs and README.

The `test` persona, and a server started without `--persona`, can also list and run plans and read every archive; see [MCP Server](mcp-server.md#personas).

## The Integrator's Side

Unpack the kit into the client's repository and register the server in `.mcp.json`:

```
their-app/
  .mcp.json
  vendor/shop-kit/     the unpacked kit
```

```json
{
  "mcpServers": {
    "shop-api": {
      "command": "aat",
      "args": ["mcp", "serve", "--manifest", "vendor/shop-kit/aat-project.yaml", "--persona", "api"]
    }
  }
}
```

Then:

- **Ask for a client.** The tool looks up operations, request templates, and integration flows instead of guessing at request and response shapes.
- **Run a reference flow** against the sandbox to see real exchanges: `cd vendor/shop-kit && aat run plan full-lifecycle`, then `aat web view latest`. The run's archive also gives `get_sample_response` real responses to return.
- **Start from live state.** `aat run plan smoke --stop-after paymentCharge --dump-state state.json` stops with a paid order still live, and the state file holds its IDs and credentials for the new client to pick up; see [Checkpoints](checkpoints.md).

## Current Limits

- A manifest names one graph, one templates directory, and one workflows directory. An operation or workflow that must stay internal cannot be added on top of a kit, so keep it out of the shipped files.
- No tool renders the concrete request for an operation from input values. `inspect_request_template` shows the template, and a reference run shows the request it produced.
- `package-kit.sh` copies a fixed list of files. Nothing derives the list from the kit manifest yet.

## The Shop Does This

[`examples/shop`](examples/shop.md) uses this layout. Its `shop-api` MCP server reads `aat-kit.yaml`, and `shop-test` reads the whole project. `make example-shop` packages the kit, unpacks it into an empty directory, and validates and runs it there against the sandbox.
