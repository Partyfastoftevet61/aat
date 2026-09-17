# gRPC payments

A small AAT project where **one plan spans two protocols**: a cart is opened and
checked out over HTTP against the shop API, and the resulting order is charged
and refunded over gRPC.

Both are `aat-sandbox`, offline and with no credentials to fetch. The payments
service on `:8767` is the same payments API the sandbox serves over HTTP on
`:8766` — the same orders, reached two ways — so the interesting part is not
that gRPC works, but that the step boundary does not care: the order id and
total flow out of an HTTP response and into a gRPC request untouched.

## Run it

```bash
make build                       # from the repository root
cd examples/grpc-payments
../../aat-sandbox serve &        # shop :8765, payments :8766, gRPC payments :8767

../../aat validate --strict      # checks the gRPC nodes against payments.protoset
../../aat run plan charge-and-refund
../../aat run plan declined-card
```

`make example-grpc` runs all of that from the repository root, starting its own
sandbox. That is also what CI runs.

## What is here

| File | |
|---|---|
| `payments.proto` | The contract, and the readable source of truth |
| `payments.protoset` | The descriptor set AAT reads, from `protoc --descriptor_set_out`. Regenerate with `make proto` |
| `graph.yaml` | Five nodes: three HTTP, two gRPC, wired by requires/satisfies |
| `templates/` | HTTP templates have `method` and `path`; gRPC ones have `rpc` and `message` |
| `env.yaml` | The shop over HTTP by default, with an override routing `payment*` to `grpc://localhost:8767` |
| `plans/` | The happy path, and a negative test naming a gRPC status |

## The three things worth reading

**A gRPC template names a method, not a path.** `templates/paymentCharge.yaml`
has `rpc: shop.v1.Payments/Charge`, sends `metadata:` rather than `headers:`,
and its `message:` is the request message written as JSON, with the same
`{{placeholder}}` substitution a body uses. Output extraction is unchanged —
the response is read as JSON, because that is what AAT turns a protobuf message
into.

**Routing is per node, and it predates gRPC.** `env.yaml` sends everything to
the shop except nodes matching `payment*`, which go to the gRPC target with a
different credential. The shop example uses the same mechanism to reach its
HTTP payments listener on another port; pointing it at `grpc://` is the only
new part.

**The amount is an `int64`, so it reads as a string.** Proto3 JSON encodes
64-bit integers as JSON strings, so the charge amount is `"10340"`, not `10340`.
That is the encoding rule most likely to surprise a plan author. See
[the gRPC guide](https://gburgyan.github.io/aat/grpc/) for the rest of them.
