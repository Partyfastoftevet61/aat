#!/usr/bin/env bash
# Runs examples/grpc-payments against a local aat-sandbox: strict validation of
# the gRPC graph against its descriptor set, the mixed HTTP-then-gRPC plan, and
# the negative plan that names a gRPC status code.
#
# Usage: scripts/example-grpc.sh (or `make example-grpc`, which builds first).
# The binaries default to the repository root builds; set AAT and AAT_SANDBOX to
# use others. Needs curl, jq, and free ports 8765, 8766, and 8767. Run output
# goes to examples/grpc-payments/_output/example-grpc/.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
aat="${AAT:-$root/aat}"
sandbox="${AAT_SANDBOX:-$root/aat-sandbox}"
out="_output/example-grpc"

step() { printf '\n==> %s\n' "$*" >&2; }
fail() { printf 'example-grpc: %s\n' "$*" >&2; exit 1; }

# expect FILE FILTER: fail, showing what did not pass, unless the jq filter holds.
expect() {
  if ! jq -e "$2" "$1" >/dev/null 2>&1; then
    jq '{outcome, error, summary, failedSteps: [.steps[]? | select(.passed | not)]}' "$1" >&2 2>/dev/null || cat "$1" >&2
    fail "$1 does not satisfy: $2"
  fi
}

for tool in curl jq "$aat" "$sandbox"; do
  command -v "$tool" >/dev/null || fail "$tool not found (run make example-grpc, or set AAT and AAT_SANDBOX)"
done
for port in 8765 8766; do
  if curl -fsS -o /dev/null --max-time 1 "http://127.0.0.1:$port/healthz" 2>/dev/null; then
    fail "something already answers on port $port; stop it first"
  fi
done

cd "$root/examples/grpc-payments"
mkdir -p "$out"

"$sandbox" serve --latency 0 --quiet &
sandbox_pid=$!
trap 'kill "$sandbox_pid" 2>/dev/null || true' EXIT

for port in 8765 8766; do
  curl -fs -o /dev/null --retry 20 --retry-connrefused --retry-max-time 10 "http://127.0.0.1:$port/healthz" ||
    fail "the sandbox did not come up on port $port"
done

step "aat validate --strict: the gRPC graph against its descriptor set"
"$aat" validate --strict

step "aat run plan charge-and-refund: three HTTP steps, then two gRPC ones"
"$aat" run plan charge-and-refund --output "$out" --json > "$out/charge-and-refund.json"
expect "$out/charge-and-refund.json" '.outcome == "passed" and ([.steps[] | select(.passed)] | length) == 5'
# The gRPC steps are there and passed; their assertions checked that the
# payment captured and the order reached paid, which is how the order id is
# known to have crossed the protocol boundary.
expect "$out/charge-and-refund.json" '[.steps[] | select(.name == "charge" and .passed)] | length == 1'
expect "$out/charge-and-refund.json" '[.steps[] | select(.name == "refund" and .passed)] | length == 1'

step "aat run plan declined-card: a gRPC status named in expectFailure"
"$aat" run plan declined-card --output "$out" --json > "$out/declined-card.json"
expect "$out/declined-card.json" '.outcome == "passed"'

step "the archive records the gRPC status by name, not the HTTP status it maps to"
archive="$(jq -r '.archive_path' "$out/declined-card.json")"
expect "$archive" '[.steps[] | select(.request.protocol? == "grpc")] | length == 1'
expect "$archive" '[.steps[] | select(.response.grpcCode? == "INVALID_ARGUMENT")] | length == 1'
expect "$archive" '[.steps[] | select((.response.grpcMessage? // "") | test("CARD_DECLINED"))] | length == 1'
# The HTTP steps of the same run record no gRPC status at all.
expect "$archive" '[.steps[] | select(.request.protocol? == null)] | length == 3'

step "aat run show reads a gRPC step"
"$aat" run show "$archive" --step charge | grep -q 'status INVALID_ARGUMENT' ||
  fail "run show did not name the gRPC status"

printf '\nexample-grpc: all checks passed\n' >&2
