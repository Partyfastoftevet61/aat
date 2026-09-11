# Launch M4 — README, example docs, and the integration kit, preceded by a review of M3

## 2026-09-11 — Review of M3: what to fix before describing it

**What:** Before the README work, read the M3 commits (run output, archive redaction, the web commands, F13,
`make demos`) and the MCP code the README describes. The author chose three fixes: redaction that fails
open, the default MCP server's missing tools, and the timeline's `0 / 0`. The batch-matrix header sizing is
logged as F41. Research for the integration kit then found three MCP bugs on that path, fixed in commit 2,
and gaps logged as P14–P16 and F42–F44.

**Decisions:**

- **Redaction fails closed.** `engine.ToArchive` kept the unredacted archive when `archive.Redact` failed,
  and a batch entry kept its unredacted error text. The comment said `archive.Write` would fail the same
  way, which holds only for marshal errors. The error path is latent: no type has custom JSON methods, and
  a NaN from a Lua transform is the realistic trigger. A silent fallback is still the wrong default in the
  one function that exists to remove secrets. `ToArchive` now returns the error, and its three callers
  write nothing.
- **No persona means every tool.** A server started without `--persona` lacked `get_data_flow`,
  `get_response_shape`, and `explain_field`, although the docs said it registers everything. They move
  into `registerFieldFlowTools`, which the api personas and the all-tools server both call. The test now
  asserts exact counts (32 and 39), so the next tool left out fails it.
- **Manifest OAS paths are resolved once.** `LoadManifest` resolves `oas:` against the manifest's
  directory, and `collectSpecPaths` joined the result onto the graph's directory again. A relative
  `--manifest` then failed at startup. The shop's `.mcp.json` only worked because its graph sits next to
  its manifest, and every test used absolute paths.
- **Samples prefer success.** `get_sample_response` returned the newest response of any status, so a
  negative test's `409` could be the sample an AI tool learns from. It now returns the newest 2xx
  response and marks a failed one, which it returns only when nothing succeeded. It also searches runs
  inside batch directories, which it never saw, and falls back to the output shape when the manifest sets
  no `archives`.

## 2026-09-11 — The integration kit: one framework, two wins

**What:** The author's positioning point for M4: the AAT project an API producer tests with is what an
integrator's AI coding tool needs, served by `aat mcp serve --persona api`. The shop now ships a kit
manifest, a packaging script, and a docs page (`docs/user/integration-kit.md`) that describes the layout
for any API. CI and the e2e test package the kit and check the unpacked copy.

**Decisions:**

- **A kit manifest, not a kit directory.** The first design moved the API model into a self-contained
  `examples/shop/kit/` that would be published as-is. The reason: the MCP server anchors the served
  README, workflow template paths, and OAS paths at the graph file's directory, so a kit manifest that
  points up (`graph: ../graph.yaml`) exposes the parent's README and every workflow. The author then
  clarified that a kit only has to be cheap to repackage in a CI step, not identical to a directory.
  So the shop keeps its layout. `aat-kit.yaml` names the shipped subset with same-directory paths,
  `internal/plans/` holds what stays, and `package-kit.sh` copies what the kit manifest names into a
  directory and a tarball. Before packaging, the kit view serves the shop README, which the author
  accepted.
- **Internal suites in `internal/plans/`, with unchanged names.** `ListPlans` names a plan by its path
  inside its directory and sorts across directories. Listing `[plans/, internal/plans/]` therefore keeps
  `negative/state-machine` and the 7 / 63 / 36 / 27 batch figures, and the M3 recordings stay valid.
  `giftcard-express` moved too: it names a layer, and the kit ships no layers.
- **Packaging is a script, checked the way an integrator uses it.** `make example-shop` and the e2e test
  run `package-kit.sh`, unpack the tarball into an empty directory, validate strictly, and run the three
  reference plans. The e2e test also loads the MCP context from the unpacked copy. So the script's copy
  list and `aat-kit.yaml` cannot drift apart unnoticed. A primitive that derives the copy list from the
  manifest is logged as P16.
- **What an integrator's tool can see is documented from the code.** The api persona reads:
  - the graph, the whole templates directory, domain, OpenAPI specs, and the docs directory
  - the workflow templates next to the graph
  - archives, for samples
  - the README next to the graph
  - the manifest's name, description, and tags

  No api tool reads plan directories, layers, or overlays, or exposes the environment file. The page turns
  that list into hygiene rules rather than promising a filter AAT does not have.
- **Environments that stay internal** live in a file that `include:`s the shipped one. Checked before
  documenting it: `extends: _base` resolves across the include, and `aat env list` shows both files'
  environments.

**Open questions:**
- P14 (`render_request`) and P15 (`execute_plan` exchanges) are what would make the MCP server a full
  oracle for a client under construction; they belong with M7's recording.
- P16: `aat kit pack`, and layering internal-only operations or workflows onto a kit.

## 2026-09-11 — README

**What:** Rewrote the README GIF-first. It covers the header with tagline and badges, why, a 60-second start
with real output, the demos, a what-it-does grid, layers and environments, how it works, the integration
kit, MCP, install, commands, and pointers into the docs site. The petstore README leads with install, and
`install.md` carries the README's curl and PowerShell lines.

**Decisions:**

- **Only shipped examples in the demo table.** Duffel, GitHub, and Stripe (M6, M9) get one roadmap line: a
  row without a working link would fail the milestone's link check.
- **No version claim before the tag.** Status says pre-1.0 instead of v0.1.0, and Install carries
  `install.md`'s pre-release note; M5 removes both notes.
- **Images by absolute raw URLs on main.** They render wherever the README is shown, and `docs/` is outside
  the module zip.
- **Every command was run.** The quick start ran verbatim against a fresh sandbox: 15/15 passed; 27 of 63
  passed with 36 skipped; the EU run showed VAT. `--override checkoutCart=…` records the overridden URL in
  the archive. Install paths were tried as far as an untagged release allows:
  - source builds
  - `go install …@main` from the module proxy: it runs, `aat web` exits 2, and `docs/` is not in the zip
  - the curl one-liner's archive name and extraction, against a goreleaser snapshot archive
  - the snapshot Docker image, with `--version` and `validate` on a mounted shop
  - the snapshot cask's binaries and quarantine hook

  The real download, the cask install, and the ghcr pull wait for the tag (M5); the PowerShell lines are
  untried.
- **Length.** About 270 lines against the planned ~220; the integration-kit section and five install
  methods account for the difference.

**Open questions:** view the rendered README and recheck the `/integration-kit/` docs link after merge.

## 2026-09-11 — The oracle, clarified, and proved on the shop

**What:** The author corrected the kit's framing. "Oracle" means exposing the entire workflow so that callers
can use it: how the calls really work, which parts of them matter, how they relate, and what ordering they
need. That carries far more information than an OpenAPI spec or traditional documentation. On the airline
API, that knowledge made a working search-and-booking integration a single prompt in Java, C#, Go, Python,
Perl, and Lisp. This session had called the oracle "partial" because no tool renders a single request, and
the entry above still calls P14 and P15 what "would make the MCP server a full oracle". Both were wrong;
those primitives are conveniences.

**Decisions:**

- **Framing:** the README, the integration-kit page, and the airline case study lead with the whole
  workflow, and the page's new table maps what an integrator needs to what the kit adds over an OpenAPI
  spec. Every row is taken from what the `api` tools print (`describe_operation`, `get_integration_flow`,
  and the domain tools). Retry settings are not exposed, so no row claims them. The request-renderer
  "limit" is gone.
- **Citation, by numbers only** (author decision): the README, the integration-kit page, and the case study
  cite the single-prompt clients. The private project is never named.
- **The shop kit had to carry what made that work.** Compared with the airline project, it had about 3
  descriptions per operation against about 9, 3 domain concepts, and no flow map. Every statement added was
  checked against the sandbox handlers rather than the M1 plan:
  - each operation's error codes, in the order the handler checks them
  - what each input must be and each output means (a charge must equal the order total; `method` decides
    which companion field is required; cancelling does not refund)
  - six new concepts: authentication, the payments host and its key, the error envelope, the two
    retryable failures, the cart lifecycle, and stock
  - pricing, order-lifecycle, and money concepts that now include the numbers
  - a kit README with a connection table, a flow map, and the rules that matter
- **Proof on the packaged kit** (author decision to spend the usage). `package-kit.sh` output went into an
  empty integrator directory, and a headless Claude Code session got one prompt per language. The session
  could use only the kit's MCP server (`--strict-mcp-config`, `api` persona), read and write only its own
  directory, and run only the language toolchain; web tools were denied. The prompt: a standard-library
  client that authenticates, buys two different in-stock products, pays by card, cancels, and refunds.
  - **Python:** correct on its first run. 85 s, 28 turns, 20 MCP calls, $0.58.
  - **Go:** correct on its first run. 146 s, 34 turns, 23 MCP calls, $0.77. The harness's allowlist blocked
    running the built binary, so the session used `go run .` instead.
  - Neither session read a kit file; everything came through MCP. Both leaned on `get_oas_operation` (8
    calls each) together with `list_integration_flows`, `get_integration_flow`, `list_concepts`,
    `explain_concept`, `explain_field`, and `get_sample_response`. Each client's own summary named the rules
    it got from the kit: two hosts with two credentials, cancel does not refund, integer minor units, and a
    charge equal to the total.
  - Rerunning each client and reading the order back from the sandbox gave `cancelled` / `refunded`.

**Open questions:**
- Record the same single-prompt run for M7 (the Python and Common Lisp takes), now on the enriched kit.
- P14 and P15 stay logged as conveniences: rendering one request, and `execute_plan` returning exchanges.
