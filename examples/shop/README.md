# AAT Shop Example

An offline e-commerce API (`aat-sandbox`) and the AAT project that tests it.
The sandbox server and its OpenAPI contract (`openapi.yaml`) are in place;
the AAT project files (graph, templates, workflows, layers, plans, environments)
land next. See `LAUNCH-PLAN.md` M1 at the repository root for the design.

```bash
aat-sandbox serve          # shop API on :8765, payments on :8766
curl -s localhost:8765/healthz
```
