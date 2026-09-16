# M8 — launch content

Drafts, positioning, and launch day. This log starts with the piece every other launch surface quotes.

## 2026-09-16 — Why AAT exists: the origin story, and where it lands

**What:**

- **A new "Why AAT exists" page** (`docs/user/why.md`), first under *Getting started* in the nav. It
  opens with where AAT came from — a pile of shared Postman collections that had stopped being
  trustworthy — maps each failure to what replaced it, and closes on the three public example projects
  as the evidence that the project runs rather than merely validates.
- **The README's `## Why` rewritten** from seven lines of abstract third person to the same story in
  first person, keeping the "three things kept apart" sentence and the 8-to-20-call framing, which
  explains *why* a folder of single requests was the wrong shape.
- **A three-sentence lead-in on the docs home.** `docs/user/index.md` had no "why" at all: it went from
  the tagline straight to a capability paragraph, so a reader arriving from a search engine never met
  the argument.
- **The three sister packages listed as what they are.** The README's demo table, the examples index,
  and `ROADMAP.md` all described real-API examples as planned. `aat-duffel` (66 endpoints, 47 plans),
  `aat-stripe` (82 operations, 53 plans, against a 205,000-line vendored spec), and `aat-shippo`
  (46 of 70 operations, 28 plans, with real labels in the web UI) are public and passing.

- **The second half of the story: what else could read the graph.** The page now turns on the moment
  the description became more valuable than the tests it was written for. The MCP server is the first
  answer — the same graph that wires a test tells an AI coding tool how to call the API — and the run
  archive is the second: a single exported file holding every request, response, resolved value, retry
  and assertion, which the other team opens in the same viewer. That is what you send when someone asks
  you to prove that an API works or doesn't, and it is strictly better evidence than a screenshot. The
  old "It composes" section was folded into it, since handing off mid-run is the same argument.

**Decisions:**

- **Name Postman, and be fair to it.** The story is specific because it happened; a vague "request
  runners" euphemism costs the argument its credibility. The criticisms are limited to ones that stay
  true — the format is proprietary, the collections live in a workspace rather than a repository, and
  variation scales by copying — with explicit credit for what it is good at. Nothing about pricing,
  licensing, or account requirements, which change and would date the copy.
- **First person, professional register.** One maintainer, one history; the page closes on "working
  with someone else's API should cost less than working around it."
- **`## Why` moved below the demos in the README** (author). A reader sees the tool work in 60 seconds
  and picks a demo before being told why it exists; the essay reads better once they have seen the
  thing. The docs site keeps its own short lead-in above the fold.
- **Frame the MCP server and the archives as consequences, not features.** They were not planned; they
  fell out of having one machine-readable description of the API. Saying so is both true and the
  argument for why this is a toolkit rather than a test runner — and it is what "adaptive" means in the
  name, the many ways one graph gets used.
- **No migration claim.** `aat import` reads AAT's own `.aar`/`.aab` archives — there is no Postman,
  HAR, or curl importer, and the page says so rather than implying one. The honest on-ramps are
  `aat generate --oas` and, when an API's documentation *is* a collection, the `expost` reader the
  primer already points to.
- **No comparison table.** The repository names no competitor anywhere. A feature matrix is a more
  defensive document and invites an argument the launch does not need.
- **One canonical airline sentence,** because four places cited the numbers with different companions:
  *a private 74-node airline booking API with 63 workflows, 53 recipes, and 6 environments*. The full
  table stays in the case study. The project itself is never named.

**Open questions:**

- **None of the three packages has CI.** Every pass/fail number in their READMEs comes from `_output/`
  archives that are gitignored, so none of it is reproducible from the public repositories. For a
  launch that argues "the proof is that it runs", that is the weakest joint. A nightly workflow guarded
  on a secret would close it.
- **`aat-shippo`'s GitHub description is stale** — it still says 23 operations run by 17 plans.
- **Should the AI assistant primer carry any motivation?** It opens as pure schema reference, so an
  assistant that reads only the primer can author a graph but cannot explain what the tool is for. It
  is token-budget sensitive and ships in the binary, so this was left alone for now.
