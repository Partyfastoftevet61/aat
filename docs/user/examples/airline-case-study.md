# Airline Case Study

AAT was built to test a production airline booking API, and that project remains its largest user. The project is private, so this page gives only its size and the AAT features that size relies on.

## The Project at a Glance

| Artifact | Count |
|----------|-------|
| Graph nodes (API operations) | 74 |
| Request templates | 69 |
| Workflows | 63 — 10 bases, 15 slot options, 38 addons |
| Layers | 6 |
| Recipes | 53 |
| Environments | 6 |

## What That Scale Relies On

**Long chains.** Booking flows cross many operations, each needing identifiers produced by earlier ones. Input defaults that take earlier outputs, named selections over search results, and `dependsOn` keep each plan a short list of steps. See [API Graphs](../graphs.md) and [Value Resolution](../value-flow.md).

**Composition.** The 53 recipes do not repeat 53 step lists. Each names a base workflow, picks slot options, adds addons, and states only the values and assertions that make it a distinct test, so a change to a base workflow reaches every recipe built on it. See [Workflows](../workflows.md) and [Plans and Recipes](../plans.md#recipes).

**Matrices across environments.** Layers vary test data without copying plans, and a batch with layer groups runs the same recipes across every combination in any of the environments, skipping permutations that would send identical requests. See [Matrix Testing](../batch-layers.md) and [Environments](../environments.md).

## Try the Same Patterns

The [shop example](shop.md) uses these patterns on an API you can run offline: a 17-operation graph, base workflows with slots and addons, layers, two environments, a Lua transform, and a visualizer.
