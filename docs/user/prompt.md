# LLM-Assisted Planning

The `aat prompt` command turns a natural-language description into an executable test plan. You describe what you want to test in plain English, and AAT's two-step LLM pipeline selects the right workflow, fills in values, and presents a ready-to-run plan for your approval.

For most AI-assisted authoring the [MCP server](mcp-server.md) is the better path: an assistant such as Claude Code reads the graph, writes recipes or plans, and validates and runs them in a loop. `aat prompt` is a single-shot convenience that needs an LLM configured in the environment.

## Prerequisites

Before using `aat prompt`, you need:

- **LLM configured** in your environment file — endpoint, API key, and model. See [Environments: LLM Configuration](environments.md#llm-configuration).
- **Graph, templates, and workflows** set up in your project. The LLM needs workflows to choose from and graph metadata to understand your API.
- **Domain knowledge** (optional but recommended) — improves value selection. See [Domain Knowledge](domain.md).

## Quick Start

```
$ aat prompt "create an order for express delivery to New York"
aat: loading environment...
aat: loaded environment "staging"
aat: authenticated via oauth2
aat: loaded graph (12 nodes)
aat: loaded domain knowledge
aat: analyzing prompt with LLM...
aat: plan generated (4 steps)

Plan: Create an order for express delivery to New York

Workflow: Order Lifecycle
  Addons: Express Shipping

Steps:
  1. search — Search for available products
  2. order — Create a new order
  3. confirm — Confirm the order
  4. check — Check the order status

LLM-provided values:
  order:
    shippingPriority: express
  confirm:
    shippingCity: New York

Assertions:
  check: status 200, fieldEquals orderStatus=confirmed

Cleanup: cancelOrder (always)

Execute this plan? [Y/n/a(djust)]
```

The summary shows what the model decided — the workflow, slot choices, addons, layers, the values it filled in, selection overrides, and assertions — rather than the wiring the workflow already provides. Press Enter (or `y`) to execute immediately. Press `n` to abort. Press `a` to edit the plan YAML before running.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--manifest` | path | auto-discovered | Explicit path to `aat-project.yaml` |
| `--env-config` | path | from manifest | Environment config file |
| `--env` | string | from manifest | Environment name (for multi-environment files); `AAT_ENV_NAME` sets it when the flag is absent |
| `--var` | `KEY=VALUE` | — | Set a var of a multi-environment file (repeatable; wins over the file's vars) |
| `--graph` | path | from manifest | API graph file |
| `--templates` | path | from manifest | Templates directory |
| `--domain` | path | from manifest | Domain knowledge file |
| `--output` | path | manifest `archives`, else `_output/runs` | Archive output directory |
| `--save` | string | — | Save the generated plan (name or path) |
| `--save-full` | bool | `false` | Save as full expanded plan instead of compact recipe |
| `--yes` | bool | `false` | Skip confirmation and execute immediately |
| `--trace` | bool | `false` | Capture planning pipeline trace for debugging |
| `--trace-dir` | path | manifest `traces` (or `traces/` next to the manifest), else `_output/traces` | Directory for plan trace output |
| `--layer` | string | — | Data layer to apply (repeatable) |
| `--no-auto-overrides` | bool | `false` | Disable auto-discovery of `.aat-overrides.yaml` |
| `--oas-validate` | string | `auto` | OAS validation mode for the executed plan: `auto`, `strict`, or `off` (see [Running Tests: OAS Validation](running.md#oas-validation)) |

When a manifest is discoverable, `--env-config`, `--graph`, `--templates`, and `--domain` are optional.

## Interactive Flow

### Plan Display

After the LLM generates a plan, AAT displays a summary of its decisions: the plan title, the workflow with any slot choices, addons, and layers, the numbered steps with their descriptions, the values the model provided (grouped by step), selection overrides, assertions, and cleanup. Wiring that the workflow already provides (`from` references, `dependsOn`) is left out. If `--save` is set, the plan is saved before you confirm.

### Confirmation

The confirmation prompt offers three choices:

| Key | Action |
|-----|--------|
| `y` or Enter | Execute the plan |
| `n`, EOF, or any other input | Abort without executing |
| `a` | Save the plan YAML to a temporary file for you to edit |

### Adjusting Plans

Pressing `a` writes the plan to a temporary YAML file and prints its path. AAT does not open an editor: edit the file in any editor, then press Enter. AAT reloads the file, re-validates it against the graph, shows the full plan, and asks for confirmation again. An edited plan that fails to load or validate ends the command with the error.

## Saving Plans

The `--save` flag writes the generated plan to disk. Saved plans can be run later with `aat run plan`.

```
aat prompt --save smoke-test "create an order and check status"
```

Name resolution for `--save`:

- Absolute paths are used as-is
- Names with `.yaml` or `.yml` extension are treated as literal paths
- Plain names are resolved through the manifest's plan directories, with `.yaml` appended automatically

### Recipe Format (default)

By default, `--save` writes a compact **recipe** — just the workflow selection and any value overrides. Recipes are small, reusable, and re-compose from the current workflow templates each time they run. This means recipes automatically pick up workflow improvements without being re-saved.

```yaml
kind: recipe
metadata:
  created: 2026-02-23T14:30:52Z
  prompt: "create an order for express delivery to New York"
  graphVersion: "1.0.0"
selection:
  workflow: order-lifecycle
  description: "Express delivery to New York"
  addons:
    - express-shipping
overrides:
  values:
    confirmOrder.shippingCity: "New York"
```

### Full Plan Format (`--save-full`)

The `--save-full` flag saves the fully expanded plan with every step and value frozen. Full plans are independent of workflow changes — they run exactly as generated, every time.

```
aat prompt --save smoke-test --save-full "create an order and check status"
```

Use recipes for most cases. Use full plans when you need exact reproducibility or the plan was manually adjusted.

## How the Pipeline Works

The `aat prompt` pipeline has five stages:

### Step 1: Workflow Selection

The LLM receives a list of available workflows, their descriptions, and your prompt. It selects the base workflow that best matches your intent, plus any addons that apply (e.g., an express-shipping addon for delivery-related prompts). It also selects slot options when the workflow has choice points, and layers when the project has a layers directory (layers given with `--layer` are always kept). Deprecated workflows are not offered, and a selection that names an unknown workflow, option, or layer is sent back to the model once with the errors.

### Step 2: Skeleton Composition

AAT composes the selected workflow with its addons into a plan skeleton. Addon steps are spliced into the correct position based on their `after` declarations. Placeholder values are auto-wired, and the list of unfed inputs (values the LLM needs to provide) is computed.

### Step 3: Value Fill

The LLM receives the plan skeleton, the list of unfed inputs with domain knowledge for each (type format, sample pool values, concepts), and your original prompt. It fills in values for every unfed input, choosing realistic data that matches your intent. Inputs marked `configurable` in the graph are offered as optional configuration. If the model reports that the selected workflow does not fit the request, AAT repeats workflow selection once with that feedback.

### Step 4: Post-Processing

AAT merges the LLM's values into the skeleton and performs mechanical fixes: repairing `dependsOn` references, normalizing selection configs, adding cleanup steps for nodes that declare cleanup counterparts, and setting plan metadata.

### Step 5: Validation

The completed plan is validated against the graph. When validation fails because of broken selection references, AAT clears them and repeats the value fill once with the validation errors in the prompt. Any other failure (e.g., the LLM referenced a node that doesn't exist) is reported. With `--trace` enabled, whatever was captured before the failure is still written to the trace directory.

The model is involved only here, while the plan is drafted. Once you confirm, the plan runs like any other: no LLM call happens during execution.

## Domain Knowledge

The quality of LLM-generated plans depends directly on the domain knowledge available. Domain knowledge teaches the LLM:

- What values are valid for each field type (value pools)
- How fields relate to each other (concepts)
- What constraints apply to specific inputs

A project with rich domain knowledge produces plans with realistic, varied data. A project without domain knowledge gets generic placeholder values.

See [Domain Knowledge](domain.md) for how to write a domain file.

## Debugging with Traces

When something goes wrong with plan generation — the LLM picks the wrong workflow, fills in bad values, or produces an invalid plan — traces are the primary debugging tool.

```
aat prompt --trace "create an order for express delivery"
```

The trace captures every pipeline stage as JSON:

- **Workflow selection call** — full system/user prompts sent to the LLM, raw response, token counts, timing
- **Skeleton** — the composed plan scaffold and YAML sent to the value-fill LLM call, with the list of unfed inputs
- **Value fill call** — full prompts, raw response, token counts, timing
- **Merge and post-processing** — snapshots of the plan after each transformation
- **Validation** — any validation errors

Traces are written to `trace-YYYYMMDD-HHMMSS-XXXXXXXX/plan-trace.json` under the trace directory: `--trace-dir`, else the manifest's `traces:` entry, else `traces/` next to the manifest, else `_output/traces`.

If the pipeline fails mid-way, whatever was captured so far is still written as a partial trace. This is invaluable for diagnosing LLM response parsing failures.

Browse traces visually with `aat web viewtrace`. See [Web UI: Viewing Traces](web-ui.md#viewing-traces) for details.

## Tips for Effective Prompts

**Be specific about what to test, not how.** Describe the scenario you want to verify, not the API calls. The LLM knows the workflows and will select the right one.

```
# Good — describes the scenario
"create an order with express shipping to Nashville"

# Less effective — describes the API calls
"call listProducts then createOrder then confirmOrder with express"
```

**Mention data you care about.** If you need a specific city, category, or shipping method, say so. The LLM will use your values and fill in everything else from domain knowledge.

```
"place an order for shoes with standard shipping to Chicago"
```

**Reference known domain concepts.** If your domain file defines concepts like "shipping priority" or "product category", use those terms. The LLM has access to your domain knowledge and will match your language to the right fields.

**Keep prompts concise.** One or two sentences is usually enough. Long, detailed prompts can confuse the workflow selection step.

**Use `--trace` to debug poor results.** If the LLM picks the wrong workflow or fills in bad values, the trace shows exactly what prompts were sent and what came back. Adjust your domain knowledge or workflow descriptions to guide better selections.

---

*Source: `cmd/aat/prompt.go`, `intent/interpret.go`, `intent/compose.go`.*
