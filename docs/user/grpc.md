# gRPC

AAT drives gRPC services the same way it drives REST ones: the same graph, the
same plans, the same assertions, the same archives. A step is a step whichever
protocol carries it, and a plan can mix both — the order id an HTTP checkout
returned goes straight into a gRPC charge, with nothing in between.

Unary methods only. See [Streaming](#streaming) for why.

## The 60-second version

The sandbox serves a gRPC payments service beside its HTTP shop, and
`examples/grpc-payments` is a project that uses both:

```bash
make build
cd examples/grpc-payments
../../aat-sandbox serve &        # shop :8765, payments :8766, gRPC payments :8767

../../aat validate --strict
../../aat run plan charge-and-refund
```

```
  [1/5] cart (createCart)    201  0ms
  [2/5] item (addItem)       201  0ms
  [3/5] order (checkoutCart) 201  0ms
  [4/5] charge                OK  162ms
  [5/5] refund                OK  0ms

PASSED (5/5 steps, 164ms)
```

Three HTTP steps, then two gRPC ones. `make example-grpc` runs the whole thing
from the repository root, starting its own sandbox; that is what CI runs.

With `aat` and `aat-sandbox` installed rather than built, the project comes out
of the sandbox binary, which serves from the same descriptor set:

```bash
aat-sandbox init --example grpc-payments grpc-payments && cd grpc-payments
aat-sandbox serve &
aat run plan charge-and-refund
```

## What you need

**A descriptor set.** AAT reads a `FileDescriptorSet` — the compiled form of
your `.proto` files — rather than parsing `.proto` source. It is one command:

```bash
# with protoc
protoc --descriptor_set_out=payments.protoset --proto_path=proto proto/payments.proto

# or with buf
buf build -o payments.protoset
```

Include imports if your protos have them:

```bash
protoc --descriptor_set_out=api.protoset --include_imports --proto_path=proto proto/*.proto
```

Check the descriptor set in beside your graph. It is a build artifact, but a
stable one, and checking it in means neither CI nor a teammate needs protoc to
run your plans. Regenerate it when the `.proto` changes.

!!! note "Why not `.proto` directly?"
    `aat generate --oas` takes a resolved OpenAPI document rather than
    fragments, and this is the same choice. Reading the compiled artifact keeps
    a `.proto` parser out of the binary, and every gRPC toolchain already
    produces one.

## Setting up a project

### 1. Name the descriptor set

In `aat-project.yaml`:

```yaml
name: payments
graph: graph.yaml
templates: templates/
proto: payments.protoset
environment: env.yaml
```

A graph can also name its own default, and a node can override it when a
service lives in a different descriptor set:

```yaml
# graph.yaml
proto: payments.protoset

nodes:
  paymentCharge:
    proto: shop.v1.Payments/Charge          # the usual form
  legacyCharge:
    proto:
      service: legacy.v1.Payments
      method: Charge
      descriptor: legacy.protoset           # this node's own
```

### 2. Point a node at a method

A gRPC node is an ordinary node with a `proto:` reference in place of `oas:`:

```yaml
nodes:
  paymentCharge:
    description: Capture payment for an order.
    adapter: paymentCharge
    proto: shop.v1.Payments/Charge
    inputs:
      - name: orderId
        type: string
        default: {from: checkoutCart.orderId}
      - name: amount
        type: integer
        default: {from: checkoutCart.total}
    outputs:
      - name: paymentId
        type: string
      - name: paymentStatus
        type: string
```

Everything else about a node — `requires`, `satisfies`, `cleanup`, defaults,
selections — works exactly as it does for HTTP.

### 3. Write the template

A gRPC template declares `protocol: grpc` and names an `rpc` instead of a
method and a path:

```yaml
adapter: paymentCharge
protocol: grpc

request:
  rpc: shop.v1.Payments/Charge
  metadata:
    x-tenant: "{{tenant}}"
  message: |
    {
      "orderId": "{{orderId}}",
      "amount": "{{amount}}",
      "currency": "{{currency}}"
    }

response:
  extract:
    paymentId: paymentId
    paymentStatus: status
```

| HTTP | gRPC |
|---|---|
| `method:` + `path:` | `rpc:` |
| `headers:` | `metadata:` |
| `body:` | `message:` |
| `form:` | — |

`message:` is the request message written as JSON, with the same
`{{placeholder}}` substitution, [conditional blocks](templates.md#conditional-blocks),
and [iteration blocks](templates.md#iteration-blocks) a body uses. Values are
escaped for JSON, so a quote in an input cannot break the message.

**Extraction is unchanged.** `response.extract` reads the response as JSON,
because that is what AAT turns a protobuf message into, so paths, `optional`,
`default`, `fields`, and header rules all behave as they do for HTTP. An
`extract` rule with `header:` reads response metadata, and falls back to
trailing metadata.

Mixing the two protocols' fields is an error rather than a thing quietly
ignored:

```
request.path belongs to an HTTP template; a gRPC request names its rpc and sends a message
```

### 4. Route to the service

A gRPC target is a `grpc://` or `grpcs://` URL. `grpc://` is plaintext;
`grpcs://` is TLS, verified against the system roots.

```yaml
environment: local
apiBaseUrl: grpc://localhost:9090
```

A target is a host and port. It carries no path — the template names the
method — and `aat validate` reports one that does. `grpcs://` without a port
means 443; `grpc://` has no default port, so leaving it out is an error rather
than plaintext sent to 443, which is where gRPC would otherwise send it.

**Mixed projects route per node**, with the same `overrides:` mechanism a
multi-host HTTP project uses:

```yaml
apiBaseUrl: http://localhost:8765/us/v1       # the REST API

overrides:
  - match: "payment*"                          # these nodes go to gRPC
    baseUrl: grpc://localhost:8767
    auth:
      type: apikey
      headerName: x-api-key
      credentials:
        key: {source: env, var: PAYMENTS_KEY}
```

### 5. Authenticate

Nothing new. Every auth type resolves to a name and value, and on a gRPC call
that travels as metadata instead of a header:

```yaml
auth:
  type: bearer                # Authorization: Bearer <token>, as metadata
  credentials:
    token: {source: env, var: API_TOKEN}
```

`oauth2` works too, and its token endpoint stays HTTP — which is normal, since
most gRPC services issue tokens over REST.

For TLS beyond the system roots:

```yaml
grpc:
  tls:
    caFile: certs/ca.pem           # a private certificate authority
    certFile: certs/client.pem     # mTLS, if the service asks for one
    keyFile: certs/client-key.pem
    serverName: api.internal       # when the address is not the certificate's name
    insecureSkipVerify: false      # a sandbox with a self-signed certificate, nothing else
```

Paths resolve beside the environment file. One `grpc:` block serves every
`grpcs://` route of the environment; an override cannot carry its own. In a
multi-environment file, an environment that declares `grpc:` replaces the block
it inherits whole, as it does for `auth`.

## Validating before you run

`aat validate` checks gRPC nodes against the descriptor set offline, with no
server running:

```
Protobuf validation: OK (2 gRPC nodes)
Node protocols:      OK (2 gRPC, 3 HTTP)
```

The second line checks that each node's `proto:` and its template's
`protocol:` and `rpc:` agree — see
[Validation](validation.md#node-protocols).

It catches what otherwise fails at run time, or worse, quietly:

```
node "paymentCharge": service "shop.v1.Payment" not found (did you mean "shop.v1.Payments"?)
node "paymentCharge": shop.v1.Payments/Watch is a server-streaming method; aat runs unary methods
node "createCollection": input "vectorSize" is sent at "vectorsConfig.Params.size": qdrant.VectorsConfig does not declare "Params" (did you mean "params"?)
node "getCollection": output "project" reads "result.config.metadta.project": qdrant.CollectionConfig does not declare "metadta"
node "paymentCharge": output "orderId" reads "order_id", but the response encodes that field as "orderId": read "orderId"
```

Inputs are checked where the template's `message:` puts them, however deep:
`"vectorsConfig": {"params": {"size": "{{vectorSize}}"}}` checks
`vectorSize` at `vectorsConfig.params.size`, through oneof members and map
values alike. An input the message doesn't use must name a top-level field of
the request, or it is reported as reaching the request some other way.

Outputs are checked along their extract paths the same way. That last error is
the one to know about — see below.

## Statuses

A gRPC status is named, not numbered, and AAT uses the name everywhere you
would see an HTTP status code:

```
  [4/5] charge                     OK  162ms
  [1/1] missing             NOT_FOUND  0ms
```

The status column is sized for the names a plan writes, so the columns after
it stay put whichever protocol a step used.

Assertions and expectations take the name:

```yaml
assertions:
  mechanical:
    - type: status
      expect: OK

expectFailure:
  status: [NOT_FOUND]
```

**Names matter, because several gRPC codes share one HTTP status.**
`INVALID_ARGUMENT`, `FAILED_PRECONDITION`, and `OUT_OF_RANGE` are all HTTP 400,
so a negative test that expects one should not pass on another. Naming the code
gets that right; a number cannot.

An [override or an overlay file](environments.md#input-value-and-expected-failure-overrides)
names a status the same way, so an existing plan reruns as a negative test
without being edited:

```yaml
overrides:
  - match: paymentCharge
    values: {cardNumber: "4000000000000002"}
    expectFailure:
      status: [INVALID_ARGUMENT]
```

Numbers still work, so a plan can read against either protocol:

```yaml
expectFailure:
  status: [404]        # matches an HTTP 404 and a gRPC NOT_FOUND
```

Under the covers each gRPC code carries the HTTP status it maps to, which is
how retries, error categories, and `2xx`-style classes keep working unchanged:
`UNAVAILABLE` and `RESOURCE_EXHAUSTED` are transient and get retried,
`UNAUTHENTICATED` and `PERMISSION_DENIED` are auth failures. `OK` is the only
code that maps below 400. A retry rule can name a status too, and then matches
it alone: `retry: {max: 2, on: [ABORTED]}`, or `failOn: [FAILED_PRECONDITION]`
to stop retrying at that code while `INVALID_ARGUMENT`, the same HTTP 400,
still retries.

### A failed call still has a body

A failing RPC carries no reply message, so AAT synthesises one. Assertions,
`errorDetection` rules, and `aat run show` read a failure exactly as they read
a success:

```json
{
  "code": "NOT_FOUND",
  "message": "no such customer",
  "details": [{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": "CART_MISSING"}]
}
```

`details` holds the status details the server attached. Google's standard
ones — `google.rpc.BadRequest`, `ErrorInfo`, `RetryInfo`, and the rest of
`error_details.proto` — are read whether or not your descriptor set includes
them, so `details.0.fieldViolations.0.field` is there to assert on. A detail of
a type neither your descriptors nor AAT knows keeps its `@type` and nothing
else.

Only what the server said is a response. A run you interrupt, or a cleanup
that runs out of an aborted run's time, is an error on the step, as it is over
HTTP, and not a `CANCELLED` or `DEADLINE_EXCEEDED` to assert on.

## Encoding

Messages cross into AAT as JSON, using the canonical proto3 JSON mapping. Most
of it is unsurprising. These are the parts that are:

| Type | Encodes as | Watch out for |
|---|---|---|
| `int64`, `uint64`, `fixed64` | a JSON **string**: `"4200"` | `fieldEquals: 4200` fails; write `"4200"`, and in a predicate `total == "4200"`. Ordering compares by value, so `total > 100` works |
| `int32`, `float`, `double` | a JSON number | — |
| `bytes` | base64 | Decode in a [Lua transform](lua-transforms.md) if you need the bytes |
| `enum` | the name: `"SHIPPED"` | A value newer than your `.proto` arrives as a bare number |
| `google.protobuf.Timestamp` | RFC 3339: `"2026-09-17T12:00:00Z"` | Sorts correctly as a string. A path stops at it: `createdAt.seconds` reads nothing, and `aat validate` says so |
| `google.protobuf.Duration` | `"3s"` | A string, not a number |
| `google.protobuf.FieldMask` | comma-joined paths | — |
| `google.protobuf.Struct`, `Value` | plain JSON | The easy case. `aat validate` can't check a path below one, since any JSON may be there |
| `google.protobuf.Any` | `{"@type": "...", ...}` | Its type must be in your descriptor set, so pass `--include_imports`; `google.rpc`'s error details are the exception, and are always known. `aat validate` checks `@type` and nothing past it |
| wrappers (`Int32Value`, …) | the bare value, or `null` | How proto3 expresses real presence |
| `map<k,v>` | an object; keys always strings | Read a value by its key: `labels.region`, `payload.city.stringValue`. A dotted key needs escaping: `labels.my\.key` |
| `float` NaN / Infinity | `"NaN"`, `"Infinity"` | Strings, so numeric predicates will not match |

Only Google's own well-known types get the special forms above. A message
that merely copies one — Qdrant's `qdrant.Value`, a fork of
`google.protobuf.Value` — is an ordinary message, so a payload value arrives
as `{"stringValue": "Berlin"}`, not as `"Berlin"`, and a template writes it
that way too.

Two rules apply throughout:

**Field names are lowerCamelCase.** `order_id` in your `.proto` is `orderId` in
JSON, which is what extract paths must use. Requests accept either spelling,
so only extraction is affected — and `aat validate` reports a path that spells
any field the wrong way, at any depth, rather than letting it silently read
nothing, and gives the path to read instead. A map key is the map's own and is
never renamed.

**Zero values are present.** A field that is `0`, `""`, or `false` appears in
the JSON rather than being omitted, so an extract rule for it does not fail. An
empty list is `[]` and an empty map `{}`, even for a deprecated repeated field
the server never sets, so compare whole objects with that in mind.
An unset `optional` field, an unset message field, and an absent map entry
stay absent, which is what makes `fieldAbsent` meaningful.

Unknown fields in a **request** are an error, named before anything reaches the
wire:

```
building the request for shop.v1.Payments/Charge: unknown field "cardNumbr"
```

## Reading a run

`aat run show` names the method and the status code:

```
step charge (node paymentCharge)
GRPC grpc://localhost:8767/shop.v1.Payments/Charge
status INVALID_ARGUMENT (CARD_DECLINED: card ending in 0002 was declined by the issuer)  pass  144ms
```

The [web UI](web-ui.md) shows the method and the service it went to in place of
a verb and a URL, separates metadata from trailers, and offers **Copy as
grpcurl** where an HTTP step offers Copy as cURL. grpcurl learns a method's
messages from the server's reflection service; for a server without one, which
includes `aat-sandbox`, add `-protoset` with your descriptor set, as the
command's first line says. Credentials are `[REDACTED]`, as they are for cURL.

[Archives](archives.md) record `protocol`, `grpcCode`, `grpcMessage`,
`grpcDetails`, and `trailers` alongside the usual fields. The `status` field
keeps the mapped HTTP status, so anything that reads archives by number still
works.

## Streaming

**Unary methods only.** A streaming method is rejected by `aat validate` and by
the executor, rather than half-working:

```
node "watchOrder": shop.v1.Orders/Watch is a server-streaming method; aat runs unary methods, where one request has one response
```

Server-streaming may arrive later, as a bounded collect that gathers messages
into a list output.

**Bidirectional streaming will not.** In a bidirectional stream the next
message depends on the previous reply, which is a program rather than a step,
and a step in AAT is a declarative function of its resolved inputs. Supporting
it would mean a second execution model. For a flow that genuinely needs one,
`--stop-after` with `--dump-state` hands a live run to a tool that can — see
[Checkpoints](checkpoints.md).

## What does not work yet

- `aat generate` does not scaffold a graph from a descriptor set. Write the
  graph and templates by hand, or have an AI tool do it through the
  [MCP server](mcp-server.md), which describes gRPC operations by service,
  method, metadata, and message.
- gRPC-Web and the Connect protocol are not supported.
- A call gets the same fixed 30-second timeout an HTTP request gets, and fails
  in the `timeout` category. It is not configurable. A reply is read whatever
  its size, as an HTTP response is: gRPC's 4 MiB default does not apply.
- The MCP server has no tools for browsing a descriptor set, as it has for an
  OpenAPI spec. An assistant writing new gRPC nodes reads the `.proto` source.
- Binary metadata, a key ending in `-bin`, is not handled specially. gRPC
  base64-encodes such a value itself, so a template writes the raw value and
  cannot write arbitrary bytes; a binary value a server returns is not valid
  text, and is archived with its unreadable bytes replaced.
- `unknown` in `retry.on` or `retry.failOn` is the gRPC status `UNKNOWN`. No
  error category has that name, so nothing else could be meant.
- gRPC's own retry and load-balancing configuration is deliberately disabled;
  AAT [retries steps](running.md#retries) itself, and both would double every
  attempt.
