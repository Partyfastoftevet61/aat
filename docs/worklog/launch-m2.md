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
