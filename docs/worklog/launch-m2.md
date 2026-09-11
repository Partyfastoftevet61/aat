# Launch M2 — docs site, preceded by a review of M1

## 2026-09-10 — Review of M1, commit 1: root-cause fixes

**What:** Before writing the docs site, reviewed the M1 commits (sandbox, `examples/shop`, and the two review
rounds) plus the docs they touch, with three read-only passes: a link and structure audit of `docs/user`, a
code-facts survey for the new pages, and a claim-by-claim check of the docs against the code. The author chose to
fix what M1 caused and anything on the docs path in this commit, and to log the rest (F1–F23 in `LAUNCH-PLAN.md`).
Every fix has a test; where a test could run against the old code it was checked to fail there first (slot order,
`--stop-after`, the validator race).

**Decisions:**

- **Slots fill in declaration order.** `fillSlots` ranged over a map built only for that loop. Order decides which
  option's cleanup comes first, which option's verification of a node wins, and which `inject` value lands, and
  `plan.Fingerprint` hashes the order-sensitive `Execution`, so two permutations of one recipe could escape dedup at
  random. The shop's slot options carry no cleanup or verification, which is why CI's 63/36/27 never wobbled.
- **State export: per-step routes, default route on top.** The dump kept only the last request's host and headers,
  so stopping the shop after `paymentCharge` handed a harness the payments API key and lost the shop bearer. Each
  step now records its `baseUrl` and `headers`; top-level `baseUrl`/`auth` come from the last request to the
  environment's `apiBaseUrl`, falling back to the last request. Per-step fields were chosen over a `routes` map keyed
  by base URL, which would merge routes that share a URL but differ in credentials. Version stays `"1"`: additive.
  The file is written to a temp file and renamed, so an existing file also ends up `0600`, and a write failure goes
  to stderr even under `--quiet`/`--json`. The `--json` step `name` became the step ID ci-cd.md already promised.
- **`--var` over `AAT_VAR_*`.** Vars could only come from the YAML, so the e2e test rewrote `env.yaml` text and a
  sandbox on other ports meant editing the example. A flag matches `--override`, works with parallel subtests
  (`t.Setenv` cannot), and is accepted wherever an environment loads (run, prompt, validate, env list, mcp serve).
  An unknown key is checked against the whole file, not the selected environment, so `aat validate --var` across
  all environments does not flag a var only one of them uses. Substitution now walks every string of the partial by
  reflection (map keys and the vars map excluded) instead of two hand-kept lists that had drifted apart; a guard test
  fills every string field and asserts none survive.
- **Loopback by default; `AAT_HOST` in the image.** `aat web` served archives plus rename and import routes on every
  interface while printing localhost; M1 had already moved the sandbox to loopback for the same reason. Inside a
  container loopback is unreachable through `-p`, so the Dockerfile sets `AAT_HOST=0.0.0.0` and `-p` remains the
  explicit exposure step. `server.BrowseURL` keeps printed URLs at `localhost` for loopback and unspecified hosts.
- **`internal/httpstatus`, a foundation package.** `plan` and `validate` each had a status-class parser with a "keep
  the two in step" comment, since neither may import the other. A zero-import package under `internal/` holds one
  copy; CLAUDE.md gains a Foundation tier for it. The engine now skips only status assertions that contradict
  `expectFailure` (success codes or 1xx–3xx classes) and evaluates agreeing ones.
- **libopenapi-validator v0.14.0 instead of the R8 lock.** Removing the lock on v0.13.1 reproduced the race (five
  race reports in five runs). On v0.14.0 with libopenapi v0.38.7 the same test ran clean 20 times under `-race`, as
  did the shop matrix three times, with strict results unchanged, so the lock is gone and the concurrency test stays
  as the guard. The upgrade moves the `go` directive to 1.25.7. v0.14.0 no longer emits "failed schema rendering";
  the check for it stays, harmlessly.
- **`aat generate` emits templates that run.** Optional query parameters, headers, and body properties now sit in
  `{{?x}}` blocks; with several optional query parameters and no required one, a gate emits `?` and each later block
  emits `&` behind a compound gate on the earlier names. A test renders every subset of optional values and checks
  the URL and JSON body. Numbers, booleans, and arrays go in unquoted. Header parameter names contain hyphens, which
  conditional block keys did not allow, so block keys now accept hyphens. `--output-graph -` writes nothing unless
  `--output-templates` is explicit, which removes the `mktemp -d` workaround from the shop README.
- **Static OAS check: fields a template supplies count.** Rule 6 flagged `photoUrls` on the petstore example although
  its template sends `"photoUrls": []`, so `examples/petstore` failed `--strict` and nothing caught it. The check
  now accepts query parameters, headers, and top-level body keys a template sends unconditionally (a tolerant scan;
  keys in conditional blocks do not count), the input-side twin of M1's R3. Petstore strict validation now runs in
  `make check` and `make example-shop`.
- **Layer sources replace graph default sources.** `MergeInputDefault` dropped a layer's `fromResolved` and let a
  graph default's `from` shadow a layer value (resolution prefers `from`). A layer that names any source now replaces
  the default's sources; a `select` survives only alongside a `from`.
- Smaller: `aat validate` walks workflow subdirectories (the shop's eight slot and addon templates were never parsed
  there); `aat prompt` rejects layers without a directory like every other path; the layers-dir error lives in
  `graph.ResolveLayerNames`; `embed_test.go` reads the example's `.gitignore`; the visualizer frame sends the
  documented CSS variable names, fixing the receipt visualizer's `--color-text-secondary`; `make clean` keeps the
  tracked `index.html`.

**Open questions:** none for this commit. Strict YAML decoding and the dead-key removal follow in commit 2, the docs
site in commits 3 and 4.

## 2026-09-10 — Commit 2: strict YAML decoding, dead keys removed

**What:** Every loader of project YAML decodes through `internal/yamlx`, which rejects keys no field accepts and
reports each with its line, the key, a noun for where it appeared, and either a near-miss suggestion or the valid
keys. Keys nothing read were deleted rather than kept as accepted-and-ignored: step `fallback`,
`assertions.semantic`, the settings `maxRunDuration`/`defaultRetries`/`archiveFormat` (and `config.Duration`,
`ArchiveFormat`, `applyDefaults`), template `response.validate`, the selection `prompt` fields and the `llm`
strategy, recipe `overrides.descriptions`, the `warn` OAS mode, and the web UI's trace `repetitions`.

**Decisions:**

- **The callback form of `UnmarshalYAML`.** yaml.v3's `Node.Decode` builds a fresh decoder without `KnownFields`,
  so any type with a `*yaml.Node` unmarshaler (step values, assertions, input defaults, extract rules) would have
  stayed lenient inside. The older `UnmarshalYAML(func(any) error)` form decodes on the parent decoder, keeping
  strictness, line numbers, anchors, and `<<` merges; a test in `yamlx` pins both behaviors. `yamlx.Node` lets such
  a method branch on the node's kind, and `yamlx.KindError` returns a `*yaml.TypeError` so the decoder records it
  and keeps collecting problems. Rejected: re-encoding nodes (loses line numbers and outside anchors), a
  hand-written key check per type, and switching YAML libraries.
- **Nouns from Go types.** The noun in `unknown key "x" in step value` comes from the type yaml.v3 names in its
  error: the `raw` prefix of method-less alias types and the Go-only suffixes `Def` and `Partial` are dropped, so
  alias types must be named `raw<Type>`. Valid keys come from a reflection walk of the target type, only on the
  error path.
- **Lenient on purpose, and marked.** The `kind` probe before a recipe decode, the multi-environment probe, the
  names listed in an error message, the overlay `environment` peek, the comment pass over the domain file, and
  the user config (shared by every installed aat version, so an older binary must tolerate newer keys) use
  `yaml.Unmarshal` with `//nolint:forbidigo` and a reason. The new `forbidigo` rule bans `yaml.Unmarshal` and
  `yaml.Node.Decode` elsewhere outside tests.
- **AI input.** Plan YAML given to the MCP plan tools is strict: an agent fixes a precise error in one turn,
  while a dropped key yields a "valid" plan that does something else. `aat prompt` parses model output as JSON
  with `encoding/json` and is unchanged; the `$EDITOR` round-trip goes through `plan.ParseFile` and is strict.
- **Errors name the file where it is read.** `plan.ParseFile`, `graph.ParseFile`, `ParseLayerFile`,
  `ParseTemplateFile`, `domain.ParseFile`, and the config loaders prefix the path; the byte-level parsers no longer
  add "YAML parse error:" (the `line N:` form already says what it is), and syntax errors read `invalid YAML:`
  because yaml.v3 omits the line number for the first line. Callers that added a path (`LoadLayersFromDir`,
  `LoadTemplates`, workflow templates, `aat validate` sections) stopped. `aat validate` rewrites the manifest's
  paths relative to the working directory, so its errors say `plans/smoke.yaml: line 12: …`.
- **Manifest errors surface.** `ResolveProjectPaths` ignored every `LoadManifest` error, so a typo in
  `aat-project.yaml` made commands fall back to `AAT_PROJECT`, the user's default project, or nothing. A manifest
  that exists but fails to load is now an error unless a higher-priority level loads one after it; a missing one is
  still skipped.
- **`aat validate` covers every file kind.** Domain and Visualizers sections join the file-level sections before
  the graph, so "run `aat validate` to find unknown keys" holds for everything except overlays, which load
  strictly when a run uses them.
- **Selection strategies in one place.** `plan.SelectionStrategies()` feeds plan validation and `aat prompt`'s
  response check (which had its own list, with `llm`); an engine test applies each strategy.
- **`warn` removed rather than aliased.** It behaved exactly like `auto`; an alias would keep documenting a
  distinction that does not exist. The error names the replacement.

**Open questions:** the private airline project may carry keys strict decoding now rejects; `aat validate` lists
them. Fixture sweep: only `graph/testdata/valid/travel_flow.yaml` and inline graph YAML in two `cmd/aat` tests had
ignored keys (an input `source:`/`from:`), plus `settings.defaultRetries` in `examples/petstore/env.yaml`.

## 2026-09-10 — Commit 3: docs site scaffold

**What:** `mkdocs.yml` over `docs/user` with Material for MkDocs 9.7.7 (pinned in `docs/requirements.txt`), a nav of
the existing pages plus `changelog.md` and `examples/shop.md`, `.github/workflows/docs.yml` (strict build on pull
requests and pushes, Pages deploy from main), and `make docs` / `make docs-serve`. The strict build passes with no
warnings.

**Decisions:**

- **Strict validation is the drift guard.** MkDocs 1.6 reports omitted nav pages, unrecognized links, and missing
  anchors at `info` by default; `validation:` raises all four to `warn`, so `--strict` fails on them. A scratch
  orphan page with a bad anchor confirmed both are caught.
- **Material for MkDocs now, Zensical later.** Material gets critical fixes only until 2026-11-05
  (squidfunk/mkdocs-material#8523) and its successor, Zensical, aims to build existing Material projects. Switching
  before launch would add a young tool to the launch path for no reader-visible gain; ROADMAP carries the move.
  Pinning an exact version keeps the build reproducible while upstream is in maintenance.
- **No macros plugin.** The docs are full of template placeholders such as `{{petId}}`; a Jinja-based plugin would
  try to render them.
- **One source for the shop README.** `examples/shop.md` includes `examples/shop/README.md` between HTML-comment
  section markers (invisible on GitHub and harmless in `aat-sandbox init` output), and `changelog.md` includes
  `CHANGELOG.md`, which keeps its own H1. These two includes are the only site-only syntax; every other page stays
  plain Markdown that reads the same on GitHub. Links that left `docs_dir` now point at `examples/shop.md` or, for
  petstore, at GitHub.
- **Python-Markdown differences fixed in the source.** A list needs a blank line before it (12 places) and content
  inside a list item needs four spaces (one fence in plans.md); GitHub renders both forms, so the fixes change
  nothing there. The missing `assets/ui-batch-matrix.png` image was removed until M3 records the web UI.
- **Pages the scaffold does not add yet.** install, lua-transforms (rewritten), checkpoints, archives, generate,
  docs-generate, the examples index, and the airline case study join the nav in commit 4, each in the change that
  creates it, so every commit builds strict.

**Open questions:** the deploy job fails until the author enables Pages (Settings → Pages → Source = GitHub Actions).

## 2026-09-10 — Commit 4: the pages, and the bugs writing them exposed

**What:** Eight new pages (install, checkpoints, archives, generate, docs-generate, lua-transforms, the examples
index, the airline case study), a new home page, the quickstart and tutorial rewritten, and a claim-by-claim drift
pass over every other guide. Four authors worked in parallel on disjoint files; each verified claims against the
code and by running `aat` against the sandbox (on private ports) or the public Petstore, and loaded every
whole-file YAML example with the strict loaders. A final read-only pass looked for contradictions between pages.

**Decisions:**

- **Fix, don't document around.** Writing verified output exposed bugs. Those on the docs path were fixed in this
  commit: run output named no failed assertion and never printed the `OAS: N warning(s)` markers the docs promised
  (the only code that printed them wrote to `io.Discard`); `aat validate` reported `OK` for sections with warnings;
  counts read "1 files"; the batch header printed `mode=strict`, a leftover of the runtime modes; a failed run's
  error printed twice; a filter-only `select` failed at run time; a missing `--manifest` was ignored; `aat generate`
  made every response property a required extract, so scaffolds failed their first run. A manifest found in the
  working directory inherited fields from the project in `AAT_PROJECT` (found because the author's shell points
  there), fixed in its own commit before this one. Everything else went to LAUNCH-PLAN as F24–F35.
- **Archive redaction was a credential leak.** Header redaction matched six names, so an API key under a custom
  `headerName` was archived in full; host-override and overlay-override credentials were never collected as
  secrets; and `metadata.plan`/`instantiatedPlan` kept a plan's literal credentials. Archives now scrub every
  known secret from all header values, collect every auth block that can apply, and redact plan credentials and
  credential headers, with a test that serializes the archive and finds no secret anywhere. `--verbose-auth` now
  shows at most half of a short password.
- **The tutorial is generated from a run.** A script writes each file exactly as the page shows it, runs each
  command against a fresh `aat-sandbox`, and renders the page with the real output, so the page and a verbatim
  run cannot disagree. The script lives outside the repository for now; F18 (a docs example guard) is the place
  to decide whether to keep such a guard in CI. The tutorial deliberately shows two failures and their fixes (a
  wrong status expectation, and the payment sent to the wrong host) because the new failure output makes both
  self-explanatory.
- **The quickstart uses the live Petstore.** It needs network access, and the public API gives every pet created
  without an `id` the same ID, so a concurrent client can make `verify` read another pet; the page says so.
  Offline users are pointed at the shop.
- **MCP leads, `aat prompt` follows.** The author considers `aat prompt` vestigial; pages present the MCP server
  as the AI path and mention `aat prompt` as a convenience. The AI honesty line stays on the home page.
- **Case study numbers were recounted.** The private project has 63 workflows (10 bases, 15 slot options, 38
  addons), not the 64 with 2 bases in LAUNCH-PLAN; the author accepted the measured counts. The page describes
  only shapes and counts.

- **A last read-only review across pages found contradictions the parallel authors could not see**, all fixed
  from the code: steps run in `dependsOn` order (tokens become `dependsOn` only when a plan is composed), graph
  defaults do wire data, domain value pools are never read at run time, composed plans clean up in creation
  order, and the honesty line's "execution is deterministic" was false because pool defaults pick at random; it
  now reads "execution never calls an LLM" everywhere, including `aat --help`. It also found that `aat prompt`
  archives did not collect overlay credentials and that `ABORTED` lines counted only the steps that ran; both
  fixed. The review was scoped to every page and took about 20 minutes before it was cut short; later reviews
  should be per topic or limited to changed pages.

**Open questions:** strict decoding rejects four kinds of keys in the private airline project: `llm.reasoningEffort`
(never read; the author wants it gone rather than wired), `settings.maxRunDuration`/`defaultRetries`,
`description` on `elementFields` entries, and `enum` under input `constraints` (use an `enum[...]` type). The author
fixes them there. F24, composed plans cleaning up in creation order, matters for recipes written through MCP and
is targeted at M5.
