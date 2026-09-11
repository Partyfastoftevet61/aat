# Launch M3 — recordings and screenshots, preceded by a review of M2

## 2026-09-11 — Review of M2: what the recordings would have shown

**What:** Before recording anything, read the M2 code the recordings exercise (run output, the web server and UI,
archives, the environment loader, strict YAML) and flagged what looked off. The author chose to fix run output,
archive redaction, and three small cleanups before recording, so the GIFs and screenshots show the corrected
behaviour, and to log the rest (F36–F40 in `LAUNCH-PLAN.md`). A design check by a planning agent against the code
corrected the first draft of the output and redaction fixes before any code was written.

**Decisions:**

- **Run output names steps by ID.** `--stop-after`, `dependsOn`, the archive, the web UI, and (since M2) the
  `--json` step `name` use step IDs; only the progress lines printed the node. `checkpoints.md` showed
  `[4/5] checkoutCart` and then used `--stop-after: no step "checkoutCart" in plan` as its example of an unknown
  step. The label is the ID, with the node in parentheses when it differs and fits the column. The column formula
  stays (20 characters at 80 columns), so a narrow terminal shows `checkout` alone: the ID is the actionable name.
  Engine errors use one `stepRef` helper, and `--stop-after` given a node lists the step IDs that run it.
- **One step-line writer.** The sequential batch observer was a copy of the plan observer that had drifted (no
  OAS total, a width-dependent retry form). Both call `writeStepResult`, and a test checks that they print the same
  lines apart from the indent.
- **Durations are wall-clock, recorded by the engine.** Deriving a run's span from step records does not work:
  cleanup steps had no start time, and a retry replaced the whole step result, so the first attempt's start was
  lost. `Engine.Run` records `StartTime` and `Duration` in its existing defer, which covers every return path;
  retried steps keep their first attempt's start; the archive stores `result.durationMs`, and older archives fall
  back to the step sum (not a span, which their zero cleanup start times would corrupt). Six places had summed
  step durations. Step durations of a second or more print as `1.4s`.
- **Ctrl+C was not an abort.** Regenerating the interrupt example for `running.md` showed `ERROR: context
  canceled` and exit 2: a step failing because the context was cancelled mid-request or during a retry wait was
  classified as an infrastructure error, so only an interrupt landing between steps produced `aborted`/130. The
  existing test cancelled before the first step. The engine now checks the context when a step errors. The
  documented 30-second cleanup budget was also lost: `runCleanup` wrapped the timeout context in
  `context.WithoutCancel`, which drops the deadline. Cleanup now detaches first and adds the budget on top.
- **Redaction: one pass, only secrets.** Redacting per field had missed URLs, bodies, outputs, and messages, and
  every new field would miss again. `archive.Redact` deep-copies through encoding/json (with `UseNumber`, so
  numbers keep their text) and walks every string, skipping `time.Time` and fields tagged `redact:"-"`
  (identifiers). JSON bodies are scanned token by token so only string values change. A byte-level replace over
  the marshaled archive was rejected: it corrupts timestamps, keys, and number tokens. A test fills every field of
  `Archive` and `BatchArchive` and checks the secret survives only in tagged fields.
- **What counts as a secret, and how it matches.** Every credential was a secret and matched as a substring, so
  the shop's `demo`/`demo` login turned `demo@example.com` into `[REDACTED]@example.com` while the same email sat
  in clear text in the request body. The oauth2 `username` and `clientId` are identifiers now; a secret shorter
  than eight characters is redacted only as a whole value; longer ones also inside strings, with their
  URL-escaped forms, and overlapping matches are covered together. Terminal and `--json` output stay unredacted,
  like the requests themselves. Tokens an API issues at run time are still unknown to AAT outside credential
  headers (F37).
- **Small cleanups.** `yamlx.Decode` rejects a second YAML document with content (a trailing `---` is fine).
  `web_cmd.go`'s three copies of directory resolution and two serve loops are one each.

**Open questions:** none for these commits.

## 2026-09-11 — F13 and a matrix layout bug the screenshot exposed

**What:** The web server now passes `retriedOn` through, and the timeline badge and step page read
`retried 2x: transient` like the CLI. The first batch-matrix screenshot clipped the rotated permutation labels at
the top (fixed 160px header) and on the right (the last label leaned out of the scroll container), and the
timeline printed `listProducts listProducts`.

**Decisions:**

- The header height and the wrapper's trailing space are computed from the label lengths (about 8.5px per
  uppercase character at 0.75rem, times sin 45°). A wide matrix may also use the space right of the 72rem page
  column; otherwise nine permutations with their labels do not fit at 1440px.
- The retry badge drops the uppercase transform: `RETRIED 2X: RESPONSE_ERROR` misrepresents category names.

## 2026-09-11 — `make demos`

**What:** `demos/run.sh` behind `make demos` extracts the shop project into a temporary directory, starts a fresh
sandbox, records `demos/plan.tape` and `demos/batch.tape` with VHS, checks both runs in their archives, serves
them with `aat web` for `demos/screenshots.mjs` (Playwright), optimizes the GIFs, enforces size budgets, and only
then writes `docs/user/assets` (committed) and `demos/out` (ignored). The committed images total about 2.8 MB:
hero GIF about 100 KB, batch GIF about 2 MB, three screenshots about 650 KB.

**Decisions:**

- **VHS is pinned to v0.11.0**, installed by the script into `demos/.bin`. The current Homebrew release, v0.12.0
  (published 2026-09-09), calls `cancel()` in its teardown and then renders with that context, so
  `exec.CommandContext` never starts ffmpeg: VHS prints "Creating out/x.gif..." and writes nothing, with no error.
  Confirmed in the v0.12.0 source (`evaluator.go`, `vhs.go`) against v0.11.0, whose `Render()` takes no context.
  Worth reporting upstream.
- **Playwright 1.63.0** is pinned by `demos/package-lock.json`, installed with `npm ci` and its own
  `playwright install chromium`, not `npx playwright@latest`, which can mismatch the library.
- **The recorded shell does not inherit the caller's.** VHS appends `os.Environ()` to its shell environment, so an
  exported `PS1` replaces the `> ` prompt its `Wait` looks for, and `NO_COLOR` turns off colour and the batch
  progress display. `run.sh` unsets both (and the `AAT_*` project variables).
- **Tapes wait for the prompt, not for `PASSED`.** `Wait+Screen /PASSED/` returns as soon as the batch's first
  result line appears. Outcomes are checked afterwards from the archives.
- **IDs come from the archive directory.** `/api/runs/latest` redirects to whichever of the run and the batch is
  newer, so after the batch tape it points at the batch.
- **Screenshots pin the clock** (`page.clock.setFixedTime`), locale, time zone, colour scheme, and reduced motion,
  so "just now" and dates do not depend on how long the recording took. Regenerate from a clean tree: the
  screenshots show the tool version, which carries `-dirty` otherwise.
- **Only embedded images are committed** (author decision): the repository was 2.1 MiB packed, and each
  regeneration adds its images to history. The MP4 (for M7/M8) and the social preview (uploaded in GitHub
  settings) go to `demos/out`. `docs/go.mod` makes `docs/` its own module, so the images stay out of the module
  zip that `go install` downloads.
- **GIF budget.** `gifsicle -O3 --lossy=60 --colors 64`: the terminal theme needs few colours, and 64 keeps the
  batch GIF near 2 MB against its 3 MB budget (128 colours put it at 2.9 MB).
- **The social preview quotes three real lines** of the batch output rather than the 116-character command,
  which does not fit.

**Open questions:** VHS v0.12.0's render bug should be reported to charmbracelet/vhs; the pin can move once a fixed
release exists.
