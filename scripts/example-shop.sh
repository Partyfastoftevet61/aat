#!/usr/bin/env bash
# Runs examples/shop against a local aat-sandbox with the checks the CI
# example-shop job runs: strict validation, every plan in both regions with
# strict OpenAPI validation, the layer matrix and its dedup counts, the
# declined-card overlay, and a checkpoint handed off to curl.
#
# Usage: scripts/example-shop.sh (or `make example-shop`, which builds first).
# The binaries default to the repository root builds; set AAT and AAT_SANDBOX to
# use others. Needs curl, jq, and free ports 8765 and 8766. Run output goes to
# examples/shop/_output/example-shop/.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
aat="${AAT:-$root/aat}"
sandbox="${AAT_SANDBOX:-$root/aat-sandbox}"
out="_output/example-shop"

step() { printf '\n==> %s\n' "$*" >&2; }
fail() { printf 'example-shop: %s\n' "$*" >&2; exit 1; }

# expect FILE FILTER: fail, showing what did not pass, unless the jq filter holds.
expect() {
  if ! jq -e "$2" "$1" >/dev/null 2>&1; then
    jq '{outcome, error, summary,
         failedRuns: [.runs[]? | select(.outcome != "passed" and .outcome != "skipped")],
         failedSteps: [.steps[]? | select(.passed | not)]}' "$1" >&2 2>/dev/null || cat "$1" >&2
    fail "$1 does not satisfy: $2"
  fi
}

for tool in curl jq "$aat" "$sandbox"; do
  command -v "$tool" >/dev/null || fail "$tool not found (run make example-shop, or set AAT and AAT_SANDBOX)"
done
for port in 8765 8766; do
  if curl -fsS -o /dev/null --max-time 1 "http://127.0.0.1:$port/healthz" 2>/dev/null; then
    fail "something already answers on port $port; stop it first"
  fi
done

cd "$root/examples/shop"
mkdir -p "$out"
state="$(mktemp)"

"$sandbox" serve --latency 0 --quiet &
sandbox_pid=$!
trap 'kill "$sandbox_pid" 2>/dev/null || true; rm -f "$state"' EXIT

for port in 8765 8766; do
  curl -fs -o /dev/null --retry 20 --retry-connrefused --retry-max-time 10 "http://127.0.0.1:$port/healthz" ||
    fail "the sandbox did not come up on port $port"
done

step "aat validate --strict"
"$aat" validate --strict

for env in us eu; do
  step "aat run batch --env $env --oas-validate strict"
  "$aat" run batch --env "$env" --oas-validate strict --no-auto-overrides \
    --output "$out" --json >"$out/batch-$env.json" || true
  expect "$out/batch-$env.json" '.outcome == "passed" and .summary.total_plans == 7 and .summary.passed_plans == 7'
done

step "aat run batch --layer-group shipping-standard,shipping-express --layer-group basket-gear,basket-apparel --parallel 4"
"$aat" run batch --layer-group shipping-standard,shipping-express --layer-group basket-gear,basket-apparel \
  --parallel 4 --oas-validate strict --no-auto-overrides --output "$out" --json >"$out/matrix.json" || true
expect "$out/matrix.json" '.outcome == "passed" and .summary.total_plans == 63 and .summary.skipped_plans == 36 and .summary.passed_plans == 27'

step "aat run plan smoke --overlay overlays/declined-card.yaml"
"$aat" run plan smoke --overlay overlays/declined-card.yaml --oas-validate strict --no-auto-overrides \
  --output "$out" --json >"$out/declined-card.json" || true
expect "$out/declined-card.json" '.outcome == "passed" and any(.steps[]; .node == "paymentCharge" and .status == 402)'

step "aat run plan smoke --stop-after checkout --dump-state -, then read the live order with curl"
"$aat" run plan smoke --stop-after checkout --dump-state - --quiet --no-auto-overrides --output "$out" >"$state"
expect "$state" '.outcome == "stopped" and .stoppedAt == "checkout"'
order="$(jq -r '.values["checkout.orderId"]' "$state")"
authorization="$(jq -r '.auth.headers.Authorization' "$state")"
curl -fsS -H "Authorization: $authorization" "http://127.0.0.1:8765/us/v1/orders/$order" >"$out/checkpoint-order.json"
expect "$out/checkpoint-order.json" '.status == "created"'

printf '\nexample-shop: all checks passed\n' >&2
