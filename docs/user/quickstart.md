# Quickstart

Get from an OpenAPI spec to a passing, self-cleaning API test in about five minutes. This guide uses the public [Swagger Petstore](https://petstore.swagger.io/) API, which needs no account; substitute your own spec and base URL to make it real.

> **Want to watch AAT work before setting anything up?** The [shop example](examples/shop.md) needs no API key or network:
>
> ```bash
> aat-sandbox init shop && cd shop
> aat-sandbox serve &
> aat run plan full-lifecycle
> ```

## Prerequisites

- AAT installed — see [Install](install.md)
- `curl`, and network access to `petstore.swagger.io`

## Step 1: Scaffold from the Spec

Create a project directory, download the Petstore spec, and let `aat generate` scaffold a graph and templates from it:

```bash
mkdir petstore-tests && cd petstore-tests
curl -sSLO https://raw.githubusercontent.com/gburgyan/aat/main/examples/petstore/petstore-spec.yaml

aat generate --oas petstore-spec.yaml \
  --output-graph graph.yaml \
  --output-templates templates/
```

```text
Generated 4 nodes, 4 templates written to templates/
```

`graph.yaml` now has one node per operation (`addPet`, `getPetById`, `deletePet`, `findPetsByStatus`) and `templates/` has one request template per node. The scaffold knows each operation's inputs and outputs, but not the order operations must run in, which operation undoes which, or what you want to call things. The next steps add that. See [Scaffolding from OpenAPI](generate.md) for everything the generator writes.

## Step 2: Add a Manifest and an Environment

The manifest tells every `aat` command where the project's files are. Create `aat-project.yaml`:

```yaml
name: petstore-tests
graph: graph.yaml
templates: templates/
environment: env.yaml
plans: plans/
archives: runs/
```

The environment says where the API lives and how to authenticate. The Petstore is public, so create `env.yaml` with no auth:

```yaml
environment: petstore
apiBaseUrl: https://petstore.swagger.io/v2
auth:
  type: none
```

Real APIs use `oauth2`, `apikey`, or `bearer` auth, with secrets read from environment variables. See [Project Setup](project-setup.md) and [Environments](environments.md).

## Step 3: Shape the Graph

Replace the scaffolded `graph.yaml` with the three operations this test needs, plus the two facts the spec cannot express — `addPet` must run before the others, and `deletePet` undoes it:

```yaml
version: 1.0.0
oas: petstore-spec.yaml

nodes:
  addPet:
    description: Add a new pet to the store
    adapter: addPet
    oas:
      operationId: addPet
    satisfies: [pet]
    cleanup: deletePet
    inputs:
      - name: name
        type: string
      - name: status
        type: string
        optional: true
        default: available
    outputs:
      - name: petId
        type: integer
      - name: name
        type: string

  getPetById:
    description: Find pet by ID
    adapter: getPetById
    oas:
      operationId: getPetById
    requires: [pet]
    inputs:
      - name: petId
        type: integer
    outputs:
      - name: name
        type: string
      - name: status
        type: string

  deletePet:
    description: Deletes a pet
    adapter: deletePet
    oas:
      operationId: deletePet
    requires: [pet]
    inputs:
      - name: petId
        type: integer
```

- `satisfies: [pet]` and `requires: [pet]` record that `addPet` has to come first. A plan still orders its steps with `dependsOn`, as in Step 4; AAT turns the tokens into `dependsOn` when it composes a plan from a workflow, and the MCP tools use them to trace which operations a test needs.
- `cleanup: deletePet` pairs creation with teardown. Whenever a plan runs `addPet`, AAT runs `deletePet` afterwards — even when a later step fails.
- `addPet`'s output is called `petId` rather than the API's `id`, so it matches `deletePet`'s input name. That is how the cleanup step finds the ID to delete.
- `status` defaults to `available`, so plans only have to name the pet.

Templates translate between these names and HTTP. Update `templates/addPet.yaml` to send an empty `photoUrls` list (the API requires the field; this test does not care about photos) and to extract the new ID as `petId`:

```yaml
adapter: addPet
protocol: http
request:
  method: POST
  path: /pet
  headers:
    Content-Type: application/json
  body: |-
    {
      "name": "{{name}}",
      "photoUrls": []{{?status}},
      "status": "{{status}}"{{/status}}
    }
response:
  extract:
    petId: id
    name: name
```

`{{?status}}…{{/status}}` is a conditional block: its contents are sent only when `status` has a value. Trim `templates/getPetById.yaml` to the outputs the graph declares, and remove the template for the node you dropped:

```yaml
adapter: getPetById
protocol: http
request:
  method: GET
  path: /pet/{{petId}}
response:
  extract:
    name: name
    status: status
```

```bash
rm templates/findPetsByStatus.yaml
```

The scaffolded `templates/deletePet.yaml` is already right. See [API Graphs](graphs.md) and [Templates](templates.md).

## Step 4: Write a Plan

A plan is the test itself. Create `plans/create-and-verify.yaml`:

```yaml
intent:
  goal: verify
  description: Add a pet, read it back, and delete it

execution:
  steps:
    - id: add
      node: addPet
      values:
        name: Buddy

    - id: verify
      node: getPetById
      dependsOn: [add]
      isGoal: true
      values:
        petId:
          from: add.petId
      assertions:
        mechanical:
          - type: status
            expect: 200
          - type: fieldEquals
            path: name
            value: Buddy
```

The `add` step creates a pet named Buddy. The `verify` step fetches it by the ID `add` returned (`from: add.petId`) and asserts that the API answers `200` with the same name. There is no cleanup in the plan: the graph's `cleanup: deletePet` pairing supplies it. See [Plans and Recipes](plans.md).

## Step 5: Validate and Run

```bash
aat validate --strict
```

```text
Manifest:        OK (project: petstore-tests)
Environment:     OK (single environment)
Graph structure: OK (3 nodes)
OAS validation:  OK
Adapter outputs: OK (3 templates)
Template inputs: OK
Plans:           OK (1 file)

Project validation: PASSED
```

`aat validate` checks every file against the others and against the spec — an output the template never extracts, a required body field the template never sends, a misspelled key in any YAML file. Then run the plan:

```bash
aat run plan create-and-verify
```

```text
aat: loading environment...
aat: loaded environment "petstore"
aat: loaded graph (3 nodes)
aat: loaded 3 templates
aat: loaded 1 OAS spec(s) for runtime validation
aat: authenticated via none
aat: executing plan (2 steps)...

  [1/2] add (addPet)         200  275ms
  [2/2] verify (getPetById)  200  39ms

  cleanup:
    deletePet              200  39ms

PASSED (2/2 steps, 355ms)
Archive: /home/you/petstore-tests/runs/run-20260911-123519-a6db0d9b/archive.json
```

Each line shows the step ID with its node in parentheses, the HTTP status, and the duration. Because the graph references the spec, every request and response was also checked against it; a mismatch would show as an `OAS: N warning(s)` marker on the step.

> **About the public Petstore:** it gives every pet created without an `id` the same ID, and anyone on the internet can write to it. If another client creates a pet between your two steps, `verify` can read their pet and fail. Rerun, or point the quickstart at your own API.

## What Just Happened

AAT ordered the steps, resolved each input (the literal `Buddy`, the `status` default, the `petId` from the first response), sent the requests, checked the assertions, and deleted the pet. The archive in `runs/` records every request and response, how each value was resolved, and each assertion's result. Browse it with `aat web view latest` (the web UI needs a release or `make build` binary), or see [Archives](archives.md).

## Let an AI Assistant Take It from Here

AAT's MCP server gives AI coding assistants such as Claude Code the graph, the templates, and tools to validate and run plans, so you can ask for tests in plain language and review the YAML they write:

```bash
aat mcp serve
```

Try prompts such as "add a test that finds pets by status and reads one back" or "write a negative test for getting a pet that does not exist". [MCP Server](mcp-server.md) shows how to register the server with your assistant, and the [AI Assistant Primer](llms.md) is the reference it reads.

## Next Steps

| Topic | Link |
|-------|------|
| A complete project built step by step, offline | [Tutorial](tutorial.md) |
| Every file of a finished example explained | [Petstore Walkthrough](petstore-walkthrough.md) |
| Everything AAT can do, in one project | [Shop example](examples/shop.md) |
| How inputs get their values | [Value Resolution](value-flow.md) |
| Reusable test patterns and compact recipes | [Workflows](workflows.md), [Plans: Recipes](plans.md#recipes) |
| Running in CI | [CI/CD Integration](ci-cd.md) |
