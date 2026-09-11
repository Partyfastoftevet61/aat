#!/usr/bin/env bash
# Regenerates the docs site's recordings and screenshots against a fresh
# aat-sandbox: the hero GIF of `aat run plan full-lifecycle`, the batch-matrix
# GIF, three web UI screenshots, an MP4 of the hero for posts, and the
# repository's social preview.
#
# Usage: demos/run.sh (or `make demos`, which builds both binaries and the web UI
# first). The binaries default to the repository root builds; set AAT and
# AAT_SANDBOX to use others.
#
# Needs: go, ttyd and ffmpeg (for VHS), gifsicle, node and npm, jq, curl, nc, and
# the JetBrains Mono font (macOS: brew install ttyd ffmpeg gifsicle and
# brew install --cask font-jetbrains-mono). The script installs the pinned VHS
# into demos/.bin (vhs 0.12.0 writes no GIF or MP4: it renders with a cancelled
# context) and the pinned Playwright with its Chromium. It needs free ports 8765
# and 8766 (the example's env.yaml points there) and 9129, and a network
# connection the first time.
#
# The project is extracted into a temporary directory, so examples/shop is not
# touched. Nothing under docs/user/assets changes unless every check passes:
# both recorded runs passed with the expected counts, and each GIF is within its
# size budget. Committed: the two GIFs and three screenshots in docs/user/assets.
# Not committed: demo-plan.mp4 and social-preview.png, written to demos/out. Set
# KEEP_WORK=1 to keep the temporary directory; it is also kept after a failure.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
aat="${AAT:-$root/aat}"
sandbox="${AAT_SANDBOX:-$root/aat-sandbox}"
demos="$root/demos"
vhs_version="v0.11.0"
vhs="$demos/.bin/vhs"
web_port=9129

# Size budgets, in bytes, for the images the docs and README embed.
hero_budget=$((2500 * 1024))
batch_budget=$((3000 * 1024))
images_budget=$((8000 * 1024))

step() { printf '\n==> %s\n' "$*" >&2; }
fail() { printf 'demos: %s\n' "$*" >&2; exit 1; }
size() { wc -c <"$1" | tr -d ' '; }

step "Checking tools"
for tool in go ttyd ffmpeg gifsicle node npm jq curl nc "$aat" "$sandbox"; do
  command -v "$tool" >/dev/null || fail "$tool not found (see the header of demos/run.sh)"
done
# grep without -q reads all of its input, so pipefail does not see a SIGPIPE.
if command -v fc-list >/dev/null; then
  fc-list : family | grep -i 'JetBrains Mono' >/dev/null || fail "the JetBrains Mono font is not installed"
else
  ls "$HOME/Library/Fonts" /Library/Fonts 2>/dev/null | grep -i 'JetBrainsMono' >/dev/null ||
    fail "the JetBrains Mono font is not installed"
fi
for port in 8765 8766 "$web_port"; do
  if nc -z localhost "$port" 2>/dev/null; then
    fail "something already listens on port $port; stop it first"
  fi
done

step "Installing VHS $vhs_version and Playwright"
if [[ ! -x "$vhs" ]] || [[ "$("$vhs" --version)" != *"$vhs_version"* ]]; then
  GOBIN="$demos/.bin" go install "github.com/charmbracelet/vhs@$vhs_version"
fi
(cd "$demos" && npm ci --silent && npx --no-install playwright install chromium)
for tape in "$demos"/*.tape; do
  "$vhs" validate "$tape" >/dev/null
done

# Recordings must not depend on the caller's shell: VHS passes its environment
# to the recorded bash, where a PS1 would replace the prompt its Wait looks for
# and NO_COLOR would turn off colour and the batch progress display.
unset NO_COLOR PS1 PROMPT AAT_PROJECT AAT_ENV_NAME AAT_HOST

work="$(mktemp -d /tmp/aat-demos.XXXXXX)"
sandbox_pid=""
web_pid=""
cleanup() {
  status=$?
  [[ -n "$web_pid" ]] && kill "$web_pid" 2>/dev/null || true
  [[ -n "$sandbox_pid" ]] && kill "$sandbox_pid" 2>/dev/null || true
  if [[ $status -eq 0 && -z "${KEEP_WORK:-}" ]]; then
    rm -rf "$work"
  else
    printf 'demos: work directory kept at %s\n' "$work" >&2
  fi
}
trap cleanup EXIT

mkdir -p "$work/bin" "$work/out" "$work/final"
ln -s "$aat" "$work/bin/aat"
ln -s "$sandbox" "$work/bin/aat-sandbox"
export PATH="$work/bin:$PATH"

step "Extracting the shop project and starting the sandbox"
aat-sandbox init "$work/shop" >/dev/null
aat-sandbox serve --quiet &
sandbox_pid=$!
for port in 8765 8766; do
  curl -fs -o /dev/null --retry 20 --retry-connrefused --retry-max-time 10 "http://127.0.0.1:$port/healthz" ||
    fail "the sandbox did not come up on port $port"
done
kill -0 "$sandbox_pid" 2>/dev/null || fail "the sandbox exited"

runs="$work/shop/_output/runs"

step "Recording demos/plan.tape"
(cd "$work" && "$vhs" "$demos/plan.tape" >/dev/null)
shopt -s nullglob
run_dirs=("$runs"/run-*)
[[ ${#run_dirs[@]} -eq 1 ]] || fail "expected one run from plan.tape, found ${#run_dirs[@]}"
jq -e '.result.outcome == "passed" and (.steps | length) == 15' "${run_dirs[0]}/archive.json" >/dev/null ||
  fail "the recorded full-lifecycle run did not pass with 15 steps (${run_dirs[0]})"
run_id="$(basename "${run_dirs[0]}")"

step "Recording demos/batch.tape"
(cd "$work" && "$vhs" "$demos/batch.tape" >/dev/null)
batch_dirs=("$runs"/batch-*)
[[ ${#batch_dirs[@]} -eq 1 ]] || fail "expected one batch from batch.tape, found ${#batch_dirs[@]}"
jq -e '.result.outcome == "passed" and .result.totalRuns == 63 and .result.skippedRuns == 36 and .result.passedRuns == 27' \
  "${batch_dirs[0]}/batch.json" >/dev/null ||
  fail "the recorded batch did not pass 27 of 63 runs with 36 skipped (${batch_dirs[0]})"
batch_id="$(basename "${batch_dirs[0]}")"

step "Taking web UI screenshots"
aat web --manifest "$work/shop" --host 127.0.0.1 --port "$web_port" 2>/dev/null &
web_pid=$!
curl -fs -o /dev/null --retry 20 --retry-connrefused --retry-max-time 10 "http://127.0.0.1:$web_port/health" ||
  fail "aat web did not come up on port $web_port"
node "$demos/screenshots.mjs" "http://127.0.0.1:$web_port" "$run_id" "$batch_id" "$work/out"
kill "$web_pid" 2>/dev/null || true
web_pid=""

step "Optimizing GIFs"
for gif in demo-plan demo-batch; do
  gifsicle -O3 --lossy=60 --colors 64 -o "$work/final/$gif.gif" "$work/out/$gif.gif"
done
cp "$work/out/ui-run-gantt.png" "$work/out/ui-step-request-curl.png" "$work/out/ui-batch-matrix.png" "$work/final/"

step "Checking size budgets"
(( $(size "$work/final/demo-plan.gif") <= hero_budget )) ||
  fail "demo-plan.gif is $(size "$work/final/demo-plan.gif") bytes, over $hero_budget"
(( $(size "$work/final/demo-batch.gif") <= batch_budget )) ||
  fail "demo-batch.gif is $(size "$work/final/demo-batch.gif") bytes, over $batch_budget"
total=0
for file in "$work/final"/*; do
  total=$((total + $(size "$file")))
done
(( total <= images_budget )) || fail "the committed images total $total bytes, over $images_budget"

step "Writing docs/user/assets and demos/out"
mkdir -p "$root/docs/user/assets" "$demos/out"
cp "$work/final"/* "$root/docs/user/assets/"
cp "$work/out/demo-plan.mp4" "$work/out/social-preview.png" "$demos/out/"
for file in "$work/final"/* "$demos/out/demo-plan.mp4" "$demos/out/social-preview.png"; do
  printf '  %-26s %8d KB\n' "$(basename "$file")" $(($(size "$file") / 1024)) >&2
done
printf '  %-26s %8d KB\n' "committed total" $((total / 1024)) >&2

printf '\ndemos: done\n' >&2
