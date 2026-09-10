# Quickstart

Get from zero to a running API test in 5 minutes. This guide uses the [Petstore API](https://petstore.swagger.io/) as an example — substitute your own API to make it real.

## Prerequisites

- AAT installed — grab a release binary (see the Install section of the README) or build from source with `make build`
- An OpenAPI spec for your API (or use the Petstore spec)
- Optional but recommended: an AI coding assistant (Claude Code, Cursor, etc.) — see [Accelerating with AI Assistance](#accelerating-with-ai-assistance)

## Step 1: Create the Project

Create a project directory and scaffold from your OpenAPI spec:

```bash
mkdir petstore-tests && cd petstore-tests

aat generate --oas petstore.yaml \
  --output-graph graph.yaml \
  --output-templates templates/
```

This creates a `graph.yaml` with one node per API operation and a `templates/` directory with one YAML file per operation. The scaffold is a starting point — it includes every operation but has no ordering, cleanup, or workflows.

Now create the project manifest:

```yaml
# aat-project.yaml
name: petstore
description: Petstore API integration tests
graph: graph.yaml
templates: templates/
environment: env.yaml
plans: plans/
archives: runs/
```

Cross-ref: [Project Setup](project-setup.md)

## Step 2: Set Up the Environment

Create an environment file with the API base URL and auth settings:

```yaml
# env.yaml
environment: dev
apiBaseUrl: https://petstore.swagger.io/v2

auth:
  type: apikey
  headerName: api_key
  credentials:
    key:
      source: env
      var: PETSTORE_API_KEY
```

For a local Petstore instance with no auth, use `auth: { type: none }` instead.

Cross-ref: [Environments](environments.md)

## Step 3: Refine the Graph

The scaffold includes every operation — trim it to the nodes you need and add ordering. Replace the scaffolded `graph.yaml` with a focused version:

```yaml
# graph.yaml
nodes:
  addPet:
    description: "Add a new pet to the store"
    adapter: addPet
    inputs:
      - name: name
        type: string
        default: ["Buddy", "Luna", "Max", "Bella"]
      - name: status
        type: string
        default: "available"
    outputs:
      - name: petId
        type: integer
        path: id
      - name: petName
        type: string
        path: name
    satisfies: [pet]
    cleanup: deletePet

  getPetById:
    description: "Find a pet by ID"
    adapter: getPetById
    inputs:
      - name: petId
        type: integer
    outputs:
      - name: name
        type: string
        path: name
      - name: status
        type: string
        path: status
    requires: [pet]

  deletePet:
    description: "Delete a pet"
    adapter: deletePet
    inputs:
      - name: petId
        type: integer
    requires: [pet]
```

Key points:
- `satisfies: [pet]` on addPet means it provides the `pet` ordering token
- `requires: [pet]` on getPetById and deletePet means they must run after a node that satisfies `pet`
- `cleanup: deletePet` pairs addPet with its teardown operation
- `default: ["Buddy", "Luna", "Max", "Bella"]` is a value pool — AAT picks one per run

Cross-ref: [API Graphs](graphs.md)

## Step 4: Write a Plan

Create a plan that adds a pet, verifies it exists, then cleans up:

```yaml
# plans/create-and-verify.yaml
metadata:
  name: create-and-verify
  description: "Add a pet, verify it was created, clean up"
intent:
  summary: "Basic pet creation smoke test"
execution:
  steps:
    - id: add
      node: addPet
      values:
        name: "Buddy"
        status: "available"
      assertions:
        mechanical:
          - type: status
            expect: 200
      isGoal: true
    - id: verify
      node: getPetById
      values:
        petId:
          from: add.petId
      dependsOn: [add]
      assertions:
        mechanical:
          - type: status
            expect: 200
          - type: fieldEquals
            path: name
            value: "Buddy"
  cleanup:
    - node: deletePet
      runOn: always
```

The plan says: create a pet named "Buddy", then fetch it by the returned ID and verify the name matches, then delete it regardless of outcome. Cleanup steps take no `values:` — `deletePet` needs a `petId`, and AAT fills it by matching the input name against the outputs of the steps that ran (here, `add`'s `petId`).

Cross-ref: [Plans and Recipes](plans.md)

## Step 5: Run It

Validate the project structure, then execute:

```bash
aat validate
aat run plan create-and-verify
```

Expected output:

```
aat: loading environment...
aat: loaded environment "dev"
aat: loaded graph (3 nodes)
aat: loaded 3 templates
aat: authenticated via apikey
aat: executing plan (2 steps)...

  [1/2] addPet               200  145ms
  [2/2] getPetById           200  89ms

  cleanup:
    deletePet              200  52ms

PASSED (2/2 steps, 286ms)
Archive: runs/run-20260910-143052-a1b2c3d4/archive.json
```

Each step line shows the index, node name, HTTP status, and duration; the `cleanup:` block lists the teardown that ran afterwards. The `deletePet` cleanup appears once even though it is declared both in the plan and on the `addPet` node — a graph-level pairing whose node already ran as a plan-level cleanup step is skipped.

The full request/response archive is written to `runs/`. Use `aat web` to browse it visually (or `aat web view run-20260910-143052-a1b2c3d4` to jump straight to this run).

## Accelerating with AI Assistance

An AI coding assistant can dramatically speed up the authoring loop. Instead of writing YAML by hand, you describe what you want and the AI edits files, runs validation, reads output, and iterates.

The loop becomes:

1. Describe the test you want (natural language)
2. AI edits graph, templates, plans — directly in YAML
3. AI runs `aat validate` to check structure
4. AI runs `aat run plan` to execute
5. AI reads the output or archive, fixes issues, repeats

To give the AI structural context, point it at the [AI Assistant Primer](llms.md).

Example prompts to try:

- "Refine graph.yaml — add ordering and cleanup for the pet operations"
- "Create a workflow for adding and verifying a pet"
- "Write three recipe variations that test with different pet names and statuses"
- "Run aat validate and fix any errors"

The AI benefits from direct access to your API specification. Companion MCP servers make this seamless:

- **[exoas](https://github.com/gburgyan/exoas)** — serves OpenAPI specs so the AI can look up operations, schemas, and parameters
- **[expost](https://github.com/gburgyan/expost)** — serves Postman collections so the AI can browse requests and saved examples

Configure these alongside your coding assistant so it can query the API spec directly rather than guessing at request/response shapes.

## What Just Happened

AAT loaded the plan, resolved input values (literals and `{from: ...}` references), made HTTP calls in dependency order, checked assertions against the responses, and ran cleanup. The full archive with request/response pairs, timing, and status codes is in `runs/`.

## Next Steps

| Topic | Link |
|-------|------|
| Build a complete project from scratch | [Tutorial](tutorial.md) |
| AI-facing schema reference | [AI Assistant Primer](llms.md) |
| How values are resolved at runtime | [Value Resolution](value-flow.md) |
| Reusable test patterns | [Workflows](workflows.md) |
| Compact test format | [Plans: Recipes](plans.md#recipes) |
| Project validation | [Validation](validation.md) |
| CI/CD pipeline integration | [CI/CD Integration](ci-cd.md) |
| IDE AI integration | [MCP Server](mcp-server.md) |
