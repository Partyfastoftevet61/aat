# Shop API integration kit

This kit describes the shop API for integrators and their AI coding tools. It has the API's 17 operations
with their request templates, the OpenAPI contract, the data flow between calls, integration flows, and
three reference plans that run against the API. `package-kit.sh` builds it from the shop's AAT test
project.

## What you need

- `aat`: see [Install](https://gburgyan.github.io/aat/install/).
- The API. This kit targets the offline sandbox that `aat-sandbox serve` starts (shop API on port 8765,
  payments API on port 8766). The sandbox's demo credentials are in `env.yaml`.

## Give your AI coding tool the API

Unpack the kit into your repository, for example as `vendor/shop-kit/`, and register the MCP server in your
`.mcp.json`:

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

The `api` persona is read-only. It serves the operations, request templates, data flow, integration flows,
OpenAPI schemas, domain values, and sample responses.

## See the real exchanges

From the kit directory, with the sandbox running:

```bash
aat run plan full-lifecycle   # one order through every state, then cleanup
aat web view latest           # every request and response, with Copy as cURL
```

After a run, `get_sample_response` returns the responses it recorded. To continue from live state in your
own client, `aat run plan smoke --stop-after paymentCharge --dump-state state.json` leaves a paid order in
place and writes its IDs and the bearer token to `state.json`.

| Plan | Flow |
|------|------|
| `smoke` | Browse, open a cart, add an in-stock item, check out, and pay by card |
| `full-lifecycle` | One order from browsing through payment, shipment, delivery, return, and refund |
| `registered-paypal-coupon` | A registered customer applies a coupon, pays with PayPal, and returns the order |

## Not included

The shop's negative tests, resilience tests, test-data layers, and any internal environments stay in the
producer's project.
