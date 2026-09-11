# Tutorial: Build a Test Project by Hand

This tutorial builds an AAT project file by file for a small e-commerce API that runs on your machine. By the end you will have a graph, request templates, two environments, plans with assertions, a reusable workflow, a recipe, and a layer matrix, and you will have seen what each piece is for by watching it run. Everything runs offline against `aat-sandbox`; allow about 45 minutes.

The API is the one the [shop example](examples/shop.md) tests. The example is the finished, larger version of what you build here, so you can compare your files with it at any point.

## What You'll Build

A test that buys a product end to end, using seven of the shop's operations:

| Operation | Method and path | Host |
|-----------|-----------------|------|
| `listProducts` | `GET /products` | shop API |
| `createCart` | `POST /carts` | shop API |
| `addItem` | `POST /carts/{cartId}/items` | shop API |
| `checkoutCart` | `POST /carts/{cartId}/checkout` | shop API |
| `paymentCharge` | `POST /payments/charges` | payments API |
| `deleteOrder` | `DELETE /orders/{orderId}` | shop API |
| `deleteCart` | `DELETE /carts/{cartId}` | shop API |

The shop API authenticates with OAuth2 bearer tokens and the payments API with an API key, on a different port — the same split a real service and its payment provider usually have.

## Prerequisites

- `aat` and `aat-sandbox` installed — see [Install](install.md)
- Two terminals

## Step 1: Start the Sandbox

In the first terminal, start the sandbox and leave it running:

```bash
aat-sandbox serve
```

```text
aat-sandbox: shop API      http://127.0.0.1:8765/{us,eu}/v1
aat-sandbox: payments API  http://127.0.0.1:8766/{us,eu}/v1   (header X-API-Key: pay-demo-key)
  token:    POST http://127.0.0.1:8765/oauth/token  grant_type=password username=demo password=demo client_id=aat-shop client_secret=aat-shop-secret
  regions:  us (USD, sales tax added at checkout)  ·  eu (EUR, VAT included, no overnight tier)
  chaos:    GET /inventory/SKU-1004 -> STALE_READ once per token; GET /shipments/{id} -> 503 for the first 2 calls
  latency:  x1 (shipOrder 600 ms, paymentCharge 350 ms)
  reset:    POST http://127.0.0.1:8765/admin/reset
```

The banner lists everything this tutorial needs: the two base URLs (the region, `us` or `eu`, is part of the path), the OAuth2 token endpoint and its demo credentials, and the payments API key.

## Step 2: Create the Project

In the second terminal, extract the finished shop example next to your project — you need its OpenAPI spec now and may want its files for comparison later — and create the project directory:

```bash
aat-sandbox init shop-reference
mkdir shop-tutorial && cd shop-tutorial
cp ../shop-reference/openapi.yaml .
mkdir templates plans
```

Every AAT project starts with a manifest that tells `aat` where its files are. Create `aat-project.yaml`:

```yaml
name: shop-tutorial
description: Tutorial project for the aat-sandbox shop API
graph: graph.yaml
templates: templates/
environment: env.yaml
defaultEnvironment: us
plans: plans/
archives: runs/
```

Any `aat` command run in this directory (or below it) finds the manifest by itself. See [Project Setup](project-setup.md).

## Step 3: Describe the Environment

An environment says where the API is and how to authenticate. The shop has two regions, so use the multi-environment format: an abstract `_base` environment (the underscore means it cannot be selected on its own) holds everything the regions share, and each region extends it. Create `env.yaml`:

```yaml
environments:
  _base:
    vars:
      apiHost: "localhost:8765"
    apiBaseUrl: http://${apiHost}/${region}/v1
    headers:
      Accept: application/json
    auth:
      type: oauth2
      tokenUrl: http://${apiHost}/oauth/token
      credentials:
        username: {source: literal, value: demo}
        password: {source: literal, value: demo}
        clientId: {source: literal, value: aat-shop}
        clientSecret: {source: literal, value: aat-shop-secret}

  us:
    extends: _base
    vars:
      region: us
```

- `${apiHost}` and `${region}` are filled from `vars` after inheritance, so `us` only has to set its region. `--var apiHost=localhost:9000` would point every environment at a sandbox on another port without editing the file.
- `oauth2` fetches a token from `tokenUrl` before the first request and sends it as a bearer token. The sandbox's credentials are demo values, so they are written as `literal` secrets; for a real API use `{source: env, var: SHOP_PASSWORD}` so no secret lives in the file.

Check that the file loads:

```bash
aat env list
```

```text
  us           http://localhost:8765/us/v1
```

See [Environments](environments.md).

## Step 4: Model the First Operations

The graph describes the API as operations (nodes) with named inputs and outputs, plus the facts a spec cannot express: which operations must run before which, which operation undoes which, and where an input's value normally comes from. Start with browsing, a cart, and adding an item. Create `graph.yaml`:

```yaml
version: "1.0.0"
title: Shop tutorial
oas: openapi.yaml

nodes:
  listProducts:
    description: List the catalog, optionally filtered by category
    adapter: listProducts
    oas:
      operationId: listProducts
    inputs:
      - name: category
        type: enum[gear, apparel, footwear]
        optional: true
    outputs:
      - name: currency
        type: string
      - name: products
        type: product[]
        elementFields:
          - name: sku
            type: string
          - name: name
            type: string
          - name: price
            type: integer
          - name: inStock
            type: boolean
    satisfies: [catalogBrowsed]

  createCart:
    description: Open a guest cart
    adapter: createCart
    oas:
      operationId: createCart
    outputs:
      - name: cartId
        type: string
    cleanup: deleteCart
    satisfies: [cartOpen]

  addItem:
    description: Add a product to a cart
    adapter: addItem
    oas:
      operationId: addItem
    inputs:
      - name: cartId
        type: string
        default:
          from: createCart.cartId
      - name: sku
        type: string
        default:
          from: listProducts.products
          select:
            strategy: match
            field: sku
            filter: inStock == true
      - name: quantity
        type: integer
        default: 1
    outputs:
      - name: lineCount
        type: integer
      - name: subtotal
        type: integer
    requires: [catalogBrowsed, cartOpen]
    satisfies: [cartPopulated]

  deleteCart:
    description: Delete a cart (the cleanup for createCart)
    adapter: deleteCart
    oas:
      operationId: deleteCart
    inputs:
      - name: cartId
        type: string
```

- **`oas:`** at the top points at the spec, and each node's `oas.operationId` ties it to an operation, so `aat validate` can check the graph and templates against the contract and runs can check every request and response.
- **`satisfies` and `requires`** record ordering: `addItem` needs a node that satisfies `catalogBrowsed` and `cartOpen` to have run first. Plans still order their steps with `dependsOn`; AAT turns the tokens into `dependsOn` when it composes a plan from a workflow (Step 9), and the MCP tools use them to trace what a test needs.
- **`cleanup: deleteCart`** pairs creation with teardown: whenever a plan creates a cart, AAT deletes it afterwards, even when a later step fails.
- **`default: {from: createCart.cartId}`** means a plan does not have to wire `cartId` by hand; it comes from the `createCart` step.
- **`select`** picks one element of an array output. `strategy: match` with `filter: inStock == true` takes the first product that is in stock, and `field: sku` takes its SKU. The sandbox's `SKU-1005` is always out of stock, so a naive "first product" would eventually pick it.

See [API Graphs](graphs.md) and [Value Resolution](value-flow.md).

## Step 5: Write Templates

A template turns a node's inputs into an HTTP request and its response into the node's outputs. Create one file per node in `templates/`:

`templates/listProducts.yaml`:

```yaml
adapter: listProducts
protocol: http
request:
  method: GET
  path: /products{{?category}}?category={{category}}{{/category}}
response:
  extract:
    currency: currency
    products:
      path: products
      fields:
        sku: sku
        name: name
        price: price
        inStock: inStock
```

`templates/createCart.yaml`:

```yaml
adapter: createCart
protocol: http
request:
  method: POST
  path: /carts
  headers:
    Content-Type: application/json
  body: "{}"
response:
  extract:
    cartId: cartId
```

`templates/addItem.yaml`:

```yaml
adapter: addItem
protocol: http
request:
  method: POST
  path: /carts/{{cartId}}/items
  headers:
    Content-Type: application/json
  body: |
    {
      "sku": "{{sku}}",
      "quantity": {{quantity}}
    }
response:
  extract:
    lineCount: lineCount
    subtotal: subtotal
```

`templates/deleteCart.yaml`:

```yaml
adapter: deleteCart
protocol: http
request:
  method: DELETE
  path: /carts/{{cartId}}
```

- `{{name}}` placeholders take input values. `{{?category}}…{{/category}}` is a conditional block: the query string is sent only when `category` has a value.
- `extract` maps each output to a path in the JSON response. For an array, `fields` names the element fields the graph declared, so `select` and assertions can use them.
- `quantity` has no quotes in the body because it is a number; the base URL, auth, and `Accept` header come from the environment.

Now validate. `--strict` also fails on warnings, which is how you want to run it while building:

```bash
aat validate --strict
```

```text
Manifest:        OK (project: shop-tutorial)
Environment:     OK (1 environment: us)
Graph structure: OK (4 nodes)
OAS validation:  OK
Adapter outputs: OK (4 templates)
Template inputs: OK

Project validation: PASSED
```

See [Templates](templates.md) and [Validation](validation.md).

## Step 6: Write and Run a Plan

A plan is a test: steps, the values they use, and what must be true afterwards. Create `plans/first-cart.yaml`:

```yaml
intent:
  goal: addItem
  description: Browse the catalog and put two of the first in-stock product in a cart

execution:
  steps:
    - node: listProducts
    - node: createCart
    - node: addItem
      dependsOn: [listProducts, createCart]
      isGoal: true
      values:
        quantity: 2
      assertions:
        mechanical:
          - type: status
            expect: 200
          - type: fieldEquals
            path: lineCount
            value: 1
          - type: predicate
            expr: subtotal > 0
```

Steps without an `id` are named after their node. `addItem` gets `cartId` and `sku` from the graph defaults and only overrides `quantity`. Run it:

```bash
aat run plan first-cart
```

```text
aat: loading environment...
aat: loaded environment "us"
aat: loaded graph (4 nodes)
aat: loaded 4 templates
aat: loaded 1 OAS spec(s) for runtime validation
aat: authenticated via oauth2
aat: executing plan (3 steps)...

  [1/3] listProducts         200  0ms
  [2/3] createCart           201  0ms
  [3/3] addItem              201  0ms  ASSERTIONS FAILED
        status: expected status 200, got 201

  cleanup:
    deleteCart             204  0ms

FAILED: step "addItem" failed mechanical validation
Archive: /home/you/shop-tutorial/runs/run-20260910-235242-31bacd01/archive.json
```

The request worked — the API answered `201 Created` — but the plan expected `200`. The line under the step says exactly which assertion failed, and the cart was still deleted. Two of the same SKU make one cart line, so `lineCount` is 1. Fix the expectation in `plans/first-cart.yaml`:

```yaml
          - type: status
            expect: 201
```

```bash
aat run plan first-cart
```

```text
aat: loading environment...
aat: loaded environment "us"
aat: loaded graph (4 nodes)
aat: loaded 4 templates
aat: loaded 1 OAS spec(s) for runtime validation
aat: authenticated via oauth2
aat: executing plan (3 steps)...

  [1/3] listProducts         200  0ms
  [2/3] createCart           201  0ms
  [3/3] addItem              201  0ms

  cleanup:
    deleteCart             204  0ms

PASSED (3/3 steps, 0ms)
Archive: /home/you/shop-tutorial/runs/run-20260910-235242-6e5b95fa/archive.json
```

Every run writes an archive under `runs/` with each request and response, how every input got its value, and each assertion's result. See [Plans and Recipes](plans.md) and [Running Tests](running.md).

## Step 7: Check Out and Pay

Add checkout, payment, and the order's cleanup to the end of `graph.yaml`, under `nodes:`:

```yaml
  checkoutCart:
    description: Price the cart and create an unpaid order
    adapter: checkoutCart
    oas:
      operationId: checkoutCart
    inputs:
      - name: cartId
        type: string
        default:
          from: createCart.cartId
      - name: shippingTier
        type: enum[standard, express, overnight]
        default: standard
      - name: postalCode
        type: string
        default: "78701"
    outputs:
      - name: orderId
        type: string
        display: Order
      - name: total
        type: integer
      - name: currency
        type: string
      - name: totalDisplay
        type: string
        display: Total
    cleanup: deleteOrder
    requires: [cartPopulated]
    satisfies: [orderCreated]

  paymentCharge:
    description: Charge the order on the payments API
    adapter: paymentCharge
    oas:
      operationId: paymentCharge
    inputs:
      - name: orderId
        type: string
        default:
          from: checkoutCart.orderId
      - name: amount
        type: integer
        default:
          from: checkoutCart.total
      - name: currency
        type: string
        default:
          from: checkoutCart.currency
      - name: cardNumber
        type: string
        default: "4242424242424242"
    outputs:
      - name: paymentId
        type: string
      - name: paymentStatus
        type: string
      - name: orderStatus
        type: string
    requires: [orderCreated]
    satisfies: [orderPaid]

  deleteOrder:
    description: Delete an order (the cleanup for checkoutCart)
    adapter: deleteOrder
    oas:
      operationId: deleteOrder
    inputs:
      - name: orderId
        type: string
```

`display:` labels an output for run output, so each checkout will print its order ID and total. The payment's `amount` and `currency` come straight from the order, which is what the payments API requires. Add the three templates:

`templates/checkoutCart.yaml`:

```yaml
adapter: checkoutCart
protocol: http
request:
  method: POST
  path: /carts/{{cartId}}/checkout
  headers:
    Content-Type: application/json
  body: |
    {
      "shippingTier": "{{shippingTier}}",
      "postalCode": "{{postalCode}}"
    }
response:
  extract:
    orderId: orderId
    total: total
    currency: currency
    totalDisplay: totalDisplay
```

`templates/paymentCharge.yaml`:

```yaml
adapter: paymentCharge
protocol: http
request:
  method: POST
  path: /payments/charges
  headers:
    Content-Type: application/json
  body: |
    {
      "orderId": "{{orderId}}",
      "amount": {{amount}},
      "currency": "{{currency}}",
      "method": "card",
      "cardNumber": "{{cardNumber}}"
    }
response:
  extract:
    paymentId: paymentId
    paymentStatus: paymentStatus
    orderStatus: orderStatus
```

`templates/deleteOrder.yaml`:

```yaml
adapter: deleteOrder
protocol: http
request:
  method: DELETE
  path: /orders/{{orderId}}
```

```bash
aat validate
```

```text
Manifest:        OK (project: shop-tutorial)
Environment:     OK (1 environment: us)
Graph structure: OK (7 nodes)
OAS validation:  WARN
  Warnings:
    - node "paymentCharge": output "paymentStatus" not found in OAS 2xx response schema for "paymentCharge"
Adapter outputs: OK (7 templates)
Template inputs: OK
Plans:           OK (1 file)

Project validation: PASSED with warnings in 1 section (--strict fails on them)
```

The payments API calls the charge's state `status`, not `paymentStatus`, and validation caught the mismatch before any request was sent. Without `--strict` a warning does not fail validation, but it is shown. The fix is the point of `extract`: the graph keeps the clearer name, and only the template knows what the API calls it. In `templates/paymentCharge.yaml`:

```yaml
response:
  extract:
    paymentId: paymentId
    paymentStatus: status
    orderStatus: orderStatus
```

Now a plan for the whole purchase. Create `plans/purchase.yaml`:

```yaml
intent:
  goal: pay
  description: Buy the first in-stock product and pay by card

execution:
  steps:
    - node: listProducts
    - node: createCart
    - node: addItem
      dependsOn: [listProducts, createCart]
    - node: checkoutCart
      dependsOn: [addItem]
      assertions:
        mechanical:
          - type: status
            expect: 201
          - type: predicate
            expr: total > 0
    - id: pay
      node: paymentCharge
      dependsOn: [checkoutCart]
      isGoal: true
      assertions:
        mechanical:
          - type: status
            expect: 201
          - type: fieldEquals
            path: paymentStatus
            value: captured
```

```bash
aat run plan purchase
```

```text
aat: loading environment...
aat: loaded environment "us"
aat: loaded graph (7 nodes)
aat: loaded 7 templates
aat: loaded 1 OAS spec(s) for runtime validation
aat: authenticated via oauth2
aat: executing plan (5 steps)...

  [1/5] listProducts         200  0ms
  [2/5] createCart           201  0ms
  [3/5] addItem              201  0ms
  [4/5] checkoutCart         201  0ms
        Order: ord_0001
        Total: $103.40
  [5/5] pay (paymentCharge)  404  0ms  ASSERTIONS FAILED  OAS: 1 warning(s)
        status: expected status 201, got 404
        fieldEquals: field "paymentStatus" does not exist

  cleanup:
    deleteOrder            204  0ms
    deleteCart             204  0ms

FAILED: step "pay" (paymentCharge) returned status 404
OAS: 1 warning(s)
Archive: /home/you/shop-tutorial/runs/run-20260910-235242-8e0a1311/archive.json
```

The payment went to the shop API, which does not serve `/payments/charges` (its error body, in the archive, says so), and the `OAS: 1 warning(s)` marker is runtime OpenAPI validation noticing that the request lacked the `X-API-Key` header the spec requires. Both cleanups ran anyway, in reverse order of creation. The payments API lives on its own host with its own credential, so route it there. Add `payHost` to the `_base` vars and an `overrides` entry to `_base` in `env.yaml`:

```yaml
environments:
  _base:
    vars:
      apiHost: "localhost:8765"
      payHost: "localhost:8766"
    apiBaseUrl: http://${apiHost}/${region}/v1
    headers:
      Accept: application/json
    auth:
      type: oauth2
      tokenUrl: http://${apiHost}/oauth/token
      credentials:
        username: {source: literal, value: demo}
        password: {source: literal, value: demo}
        clientId: {source: literal, value: aat-shop}
        clientSecret: {source: literal, value: aat-shop-secret}
    overrides:
      - match: "payment*"
        baseUrl: http://${payHost}/${region}/v1
        auth:
          type: apikey
          headerName: X-API-Key
          credentials:
            key: {source: literal, value: pay-demo-key}

  us:
    extends: _base
    vars:
      region: us
```

An override matches node names — here every `payment*` operation — and changes where those requests go and how they authenticate. Headers the environment sets still apply; the shop's bearer token is not sent to the payments host, because the override declares its own auth.

```bash
aat run plan purchase
```

```text
aat: loading environment...
aat: loaded environment "us"
aat: loaded graph (7 nodes)
aat: loaded 7 templates
aat: loaded 1 OAS spec(s) for runtime validation
aat: authenticated via oauth2
aat: override: payment*
aat: executing plan (5 steps)...

  [1/5] listProducts         200  0ms
  [2/5] createCart           201  0ms
  [3/5] addItem              201  0ms
  [4/5] checkoutCart         201  0ms
        Order: ord_0002
        Total: $103.40
  [5/5] pay (paymentCharge)  201  351ms

  cleanup:
    deleteOrder            204  0ms
    deleteCart             204  0ms

PASSED (5/5 steps, 352ms)
Archive: /home/you/shop-tutorial/runs/run-20260910-235242-60968cd7/archive.json
```

Both hosts, both credentials, one plan. The order and cart were deleted afterwards, so the run left nothing behind.

## Step 8: Capture the Pattern as a Workflow

Most tests of this API start the same way: browse, fill a cart, check out, pay. A workflow names that sequence once so tests can reuse it. Register it at the top of `graph.yaml`, after the `oas:` line:

```yaml
workflows:
  - name: Purchase
    description: Buy the first in-stock product as a guest and pay by card
    template: workflows/purchase.yaml
```

The template is an ordinary plan without the test-specific parts. Create `workflows/purchase.yaml`:

```bash
mkdir workflows
```

```yaml
intent:
  goal: pay
  description: Buy the first in-stock product as a guest and pay by card

execution:
  steps:
    - node: listProducts
    - node: createCart
    - node: addItem
      dependsOn: [listProducts, createCart]
    - node: checkoutCart
      dependsOn: [addItem]
    - id: pay
      node: paymentCharge
      dependsOn: [checkoutCart]
      isGoal: true
```

Tell the manifest where workflows live by adding a line to `aat-project.yaml`:

```yaml
workflows: workflows/
```

See [Workflows](workflows.md) for slots (choice points such as the payment method) and addons (optional steps spliced in), which the shop example uses heavily.

## Step 9: Write a Recipe

A recipe is a test written against a workflow: it names the workflow and states only what differs. Create `plans/two-items.yaml`:

```yaml
kind: recipe
selection:
  workflow: Purchase
  description: Buy two of a product; the payment is captured
overrides:
  values:
    addItem.quantity: 2
  assertions:
    pay:
      - type: fieldEquals
        path: paymentStatus
        value: captured
```

Override keys are `stepId.input` for values and step IDs for assertions. Steps composed from a workflow also get a default `status: 2xx` assertion. Run it like any plan:

```bash
aat run plan two-items
```

```text
aat: loading environment...
aat: loaded environment "us"
aat: loaded graph (7 nodes)
aat: loaded 7 templates
aat: loaded 1 OAS spec(s) for runtime validation
aat: reconstituting recipe "Purchase"...
aat: authenticated via oauth2
aat: override: payment*
aat: executing plan (5 steps)...

  [1/5] listProducts         200  0ms
  [2/5] createCart           201  0ms
  [3/5] addItem              201  0ms
  [4/5] checkoutCart         201  0ms
        Order: ord_0003
        Total: $200.82
  [5/5] pay (paymentCharge)  201  351ms

  cleanup:
    deleteCart             204  0ms
    deleteOrder            204  0ms

PASSED (5/5 steps, 353ms)
Archive: /home/you/shop-tutorial/runs/run-20260910-235242-ee8e9b6f/archive.json
```

AAT composed the workflow into a full plan, applied the overrides, and ran it. When the workflow changes — a new step, a different cleanup — every recipe built on it follows. The cleanup order differs from `purchase`: composition lists a workflow's cleanup pairings in step order, so the cart is deleted before the order (see [Running Tests: Cleanup](running.md#cleanup)).

## Step 10: Multiply with Layers

A layer is a named set of input values applied on top of a plan. Layers turn one test into a matrix without copying it. Add a `layers/` directory, one file per shipping tier, and register it in `aat-project.yaml`:

```bash
mkdir layers
```

`layers/standard.yaml`:

```yaml
name: standard
description: Standard shipping
inputs:
  shippingTier: standard
```

`layers/express.yaml`:

```yaml
name: express
description: Express shipping
inputs:
  shippingTier: express
```

`layers/overnight.yaml`:

```yaml
name: overnight
description: Overnight shipping
inputs:
  shippingTier: overnight
```

And in `aat-project.yaml`:

```yaml
layers: layers/
```

`aat run batch` runs every plan in `plans/`. Each `--layer-group` adds a dimension: a group of three layers runs every plan as-is and once per layer. `--quiet` prints one line per run:

```bash
aat run batch --layer-group standard,express,overnight --quiet
```

```text
first-cart [(base)]: PASSED
purchase [(base)]: PASSED
purchase [express]: PASSED
purchase [overnight]: PASSED
two-items [(base)]: PASSED
two-items [express]: PASSED
two-items [overnight]: PASSED
first-cart [express]: SKIPPED (duplicate of first-cart [(base)])
first-cart [overnight]: SKIPPED (duplicate of first-cart [(base)])
first-cart [standard]: SKIPPED (duplicate of first-cart [(base)])
purchase [standard]: SKIPPED (duplicate of purchase [(base)])
two-items [standard]: SKIPPED (duplicate of two-items [(base)])
Batch: 7/12 PASSED, 5 SKIPPED
Archive: /home/you/shop-tutorial/runs/batch-20260910-235243-e2267d6a
```

Twelve permutations, seven runs. `standard` is already the graph default, so those permutations would send exactly the same requests as the plans themselves, and `first-cart` never checks out, so no shipping tier changes it. AAT detects such duplicates before running anything and skips them. Add a second `--layer-group` (card types, customers, basket contents) and the runs multiply; see [Matrix Testing](batch-layers.md).

## Step 11: Add a Second Region

The EU region prices in euros with VAT included, and its checkout needs an EU postal code. Environment `values` are available to inputs as `{{env.NAME}}`, so each region can supply its own. Add a `values` block to `us` and a new `eu` environment at the end of `env.yaml`:

```yaml
  us:
    extends: _base
    vars:
      region: us
    values:
      postalCode: "78701"

  eu:
    extends: _base
    vars:
      region: eu
    values:
      postalCode: "10115"
```

Then make the graph default read it — in `graph.yaml`, change `checkoutCart`'s `postalCode` input:

```yaml
      - name: postalCode
        type: string
        default: "{{env.postalCode}}"
```

```bash
aat validate --strict
```

```text
Manifest:               OK (project: shop-tutorial)
Environment:            OK (2 environments: eu, us)
Graph structure:        OK (7 nodes)
OAS validation:         OK
Adapter outputs:        OK (7 templates)
Template inputs:        OK
Workflow compatibility: OK (1 workflow)
Workflows:              OK (1 file, 1 template)
Layers:                 OK (3 layers)
Plans:                  OK (3 files, 1 recipe)

Project validation: PASSED
```

```bash
aat run plan purchase --env eu
```

```text
aat: loading environment...
aat: loaded environment "eu"
aat: loaded graph (7 nodes)
aat: loaded 7 templates
aat: loaded 1 OAS spec(s) for runtime validation
aat: authenticated via oauth2
aat: override: payment*
aat: executing plan (5 steps)...

  [1/5] listProducts         200  0ms
  [2/5] createCart           201  0ms
  [3/5] addItem              201  0ms
  [4/5] checkoutCart         201  0ms
        Order: ord_0001
        Total: €87.78
  [5/5] pay (paymentCharge)  201  351ms

  cleanup:
    deleteOrder            204  0ms
    deleteCart             204  0ms

PASSED (5/5 steps, 352ms)
Archive: /home/you/shop-tutorial/runs/run-20260910-235245-a3cb7e02/archive.json
```

Same plan, same files, a different region: the base URL, prices, currency, and postal code all changed through the environment alone. `aat run batch --env eu --layer-group standard,express,overnight` would run the matrix there too — and show that the EU has no overnight tier.

## Step 12: Look at the Results

Open the newest run in the web UI (it needs a release or `make build` binary):

```bash
aat web view latest
```

The run view shows each step on a timeline with its request, response, resolved inputs, and assertion results; batches show as a permutation matrix. See [Web UI](web-ui.md) and [Archives](archives.md).

## Recap

| File | What it does |
|------|--------------|
| `aat-project.yaml` | Tells every `aat` command where the project's files are |
| `env.yaml` | Base URLs, auth, and per-host overrides for each region |
| `graph.yaml` | Operations, their inputs and outputs, ordering, cleanup, defaults, and workflows |
| `templates/*.yaml` | HTTP requests and response extraction, one per operation |
| `plans/first-cart.yaml`, `plans/purchase.yaml` | Full plans: steps, values, assertions |
| `workflows/purchase.yaml` | The reusable purchase sequence |
| `plans/two-items.yaml` | A recipe: a test stated as the difference from a workflow |
| `layers/*.yaml` | Input values that multiply plans into a matrix |

## Where to Go Next

The [shop example](examples/shop.md) (your `shop-reference` directory) covers what this tutorial left out, on the same API:

- **Slots and addons** — the Checkout workflow with a customer slot, a payment slot, and coupon, tracking, and return addons
- **Negative tests** — `expectFailure` steps, `mutations` that send broken payloads, and an overlay that turns a payment into a declined card
- **Retries** — steps that retry the sandbox's stale inventory read and its warming-up carrier
- **Checkpoints** — stopping after a step and handing the live session to another tool; see [Checkpoints](checkpoints.md)
- **Visualizers** — a receipt rendered in the web UI; see [Visualizers](visualizers.md)
- **AI assistants** — `aat mcp serve` gives Claude Code and other MCP clients the graph and tools to write and run plans like these; see [MCP Server](mcp-server.md)
