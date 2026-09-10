# Launch M0 — hygiene, docs drift, engine fixes, release plumbing

## 2026-09-10 — Engine fixes found while planning the public launch

**What:** While designing the public demo projects (see `LAUNCH-PLAN.md`), three engine behaviors
turned out to contradict the user docs. All three are fixed in this milestone.

1. **Plan-level `execution.cleanup` and `execution.verification` steps were never executed.** The
   engine only ran graph-level `cleanup:` pairings (FILO stack). Both blocks were validated,
   composed, formatted, and copied into archives, but `engine.Run` never iterated them.
2. **`retry.on` / `retry.failOn` matched category names only**, while `docs/user/plans.md` showed
   status codes (`on: [500, 502, 503]`). Those rules silently never retried, and nothing validated
   the entries.
3. **Override precedence was first-registered-wins** among globs, so an `.aat-overrides.yaml` or
   `--overlay` glob lost to an `env.yaml` glob for the same node. The docs promised the opposite.

**Decisions:**

- Cleanup order: plan-level cleanup steps run first, in declaration order, honoring
  `runOn: always|success|failure` (empty = always; `failure` covers failed, error, and aborted).
  Then the graph FILO stack runs; an entry whose node already ran as a plan-level cleanup step is
  skipped so a resource is not deleted twice. Cleanup never runs after a `--stop-after` checkpoint.
  Cleanup failures are archived but never change the outcome.
- Verification steps run after the main flow completes and before cleanup. Each becomes a synthetic
  step with ID `verify_<node>` (`plan.VerificationSteps`), receiving graph and layered input defaults
  exactly as main steps do; inputs still unset are matched by output name to the most recent earlier
  step that produced that name. They never push cleanup entries. A failed assertion, an error
  status, or a detected response-body error marks the run failed; with `ContinueOnAssertionFailure`
  the remaining verification steps still run. Verification results are appended to `Steps` so
  archives and the web UI show them without schema changes.
- `RunState.ExecutedSteps` now returns step IDs in execution order (it used to return map keys in
  random order, which made cleanup name-matching nondeterministic when two steps shared an output).
- Retry rules accept HTTP status codes as integers and category names (case-insensitive). The
  category list lives in `plan.RetryCategories`; an engine test keeps it in sync with
  `ErrorCategory.String()`. `aat validate plan` rejects unknown rules and negative `max`.
- Override precedence: exact names still beat globs; within each kind the last registered match
  wins. Registration order is env.yaml → `.aat-overrides.yaml` → `--overlay` → `--override`.
- `aat web` exits with code 2 and an install hint when the frontend bundle is not embedded (a plain
  `go install` build); `--dev` is exempt. `internal/version.Effective()` falls back to the module
  version from Go build info so `go install …@v0.1.0` reports the right version.
- The MCP `execute_plan` tool no longer advertises the removed `mode` parameter.
- Release plumbing staged for v0.1.0: versionless archive names for stable
  `releases/latest/download/…` URLs, a Homebrew tap formula (`gburgyan/homebrew-tap`), and a
  multi-arch distroless image at `ghcr.io/gburgyan/aat`. Tagging is deferred to M5.

**Open questions:**

- Whether the archive/web UI should mark verification steps distinctly (a `phase` field) rather
  than relying on the `verify_` ID prefix.
- Whether `retry.on` should also accept class patterns such as `5xx`.
