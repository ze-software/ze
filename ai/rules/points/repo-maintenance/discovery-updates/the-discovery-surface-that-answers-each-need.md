---
kind: table
level: MUST
stage:
---
| Need | Existing surface |
|------|------------------|
| Changed-file-aware wiring, doc, command, and inventory gate | `make ze-verify-wiring-docs` |
| Documentation drift and YANG command contracts | `make ze-doc-test` |
| Source-to-document reverse index | `make ze-doc-index`; read `ai/CODE-TO-DOCS.md` |
| RFC MUST requirement to enforcing-test coverage (which tests prove each requirement, plus the backlog) | `make ze-rfc-index`. Read `rfc/requirements/<stem>.md` for one RFC's requirement to test rows. Read `ai/RFC-REQUIREMENTS.md` for the counts, the coverage rollup and the backlog over all of them. Both are generated. Coverage is gated by `make ze-rfc-check`, staleness by `make ze-doc-test` |
| What each package does ("what does what") | `make ze-discovery-index`; read `ai/PACKAGE-MAP.md` |
| Which `.go` files implement a design doc | read `ai/DOCS-TO-CODE.md` (inverse of `// Design:`) |
| Which tests enforce an RFC MUST | read `rfc/requirements/<stem>.md`, or print it with `python3 scripts/dev/rfc_requirements.py --show <stem>`. `make ze-rfc-index` generates it. `make ze-rfc-check` and `make ze-doc-test` gate its freshness |
| The un-enrolled backlog, and how much each RFC still owes | read `ai/RFC-REQUIREMENTS.md`, the index over the per-RFC files (same generator, same gates) |
| Which problems recur | `make ze-journal`; read `plan/journal/` (one file per class, row count is recurrence) |
| Whether every path the instruction corpus names still resolves | `make ze-doc-links`. It is its OWN `ze-verify` stage: `make ze-doc-test` runs no part of it, and `ze-regen-check` ends with the `--md-only` subset. It also rejects a dead `*.sh` or `c_*`/`check_*` name in the hook-describing documents |
| Whether a `doc-links: ignore` marker states a reason, anywhere in the tree | `make ze-doc-links` (`check_ignore_reasons` in `scripts/dev/check_doc_links.py`). The sweep is over every TRACKED file, not the walked corpus, so a marker nobody's gate reads is still audited |
| Whether every path a TRACKED file names resolves, outside the instruction corpus | `make ze-doc-links` (`check_tracked_citations` in `scripts/dev/check_doc_links.py`). A dead path in any tracked file fails the gate. Repair the reference, or mark its line with a `doc-links: ignore` marker that states why the path cannot resolve. `vendor/` and `third_party/` are excluded because their references point into another repository, and `plan/handover/` because it records the tree as it was. `scripts/dev/doc_citation_baseline.txt` grandfathers the pairs that predate the check. `check_baseline_growth` compares the pairs against HEAD and refuses each pair HEAD does not hold, so that file only shrinks |
| Whether every symbol a `docs/` source anchor names is declared in the file that anchor points at | `make ze-doc-test` (`check_anchor_symbols` in `scripts/dev/code_to_docs.py`). It resolves the tokens after the anchor's `--` against that file's own top-level declarations, and the `report=` argument `main()` passes decides whether a finding is emitted |
| How data flows through a subsystem | read `ai/digests/<subsystem>.md` (living, hand-maintained flow digests; `ai/digests/README.md` lists them); anchors validated by `make ze-digest-check` |
| Plugin, command, YANG, and test inventory | `make ze-inventory`, `make ze-inventory-json` |
| Command inventory | `make ze-command-list`, `make ze-command-list-json` |
| Spec progress | `make ze-spec-status`, `make ze-spec-status-json` |
| Generated plugin imports | `make ze-plugin-imports-check` |
| Whether the tree GIT HOLDS compiles, as opposed to the working tree every other gate reads | `make ze-tracked-build-check` (`REV=<sha>` judges another commit). Runs in `ze-verify`, both modes, and is a structural gate in `scripts/dev/commit_helper.py` |
| Runtime readiness | `ze doctor --json` and `ze explain <diagnostic-code>` |
