---
kind: table
level: MUST
stage:
---
| Need | Existing surface |
|------|------------------|
| Changed-file-aware wiring, doc, command, and inventory gate | `./le doc-wiring` |
| Documentation drift and YANG command contracts | `./le doc-check verify` |
| Source-to-document reverse index | `./le docs-to-code index-update`; read `ai/CODE-TO-DOCS.md` |
| RFC MUST requirement to enforcing-test coverage (which tests prove each requirement, plus the backlog) | `./le rfc index-update`. Read `rfc/requirements/<stem>.md` for one RFC's requirement to test rows. Read `ai/RFC-REQUIREMENTS.md` for the counts, the coverage rollup and the backlog over all of them. Both are generated. Coverage is gated by `./le rfc check`, staleness by `./le doc-check verify` |
| What each package does ("what does what") | `./le discovery-index update`; read `ai/PACKAGE-MAP.md` |
| Which `.go` files implement a design doc | read `ai/DOCS-TO-CODE.md` (inverse of `// Design:`) |
| Which tests enforce an RFC MUST | read `rfc/requirements/<stem>.md` after `./le rfc index-update`. `./le rfc check` and `./le doc-check verify` gate its freshness |
| The un-enrolled backlog, and how much each RFC still owes | read `ai/RFC-REQUIREMENTS.md`, the index over the per-RFC files (same generator, same gates) |
| Which problems recur | `./le journal report`; read `plan/journal/` (one file per class, row count is recurrence) |
| Whether every path the instruction corpus names still resolves | `./le doc-check links`. It is its own `./le verify current mode full` stage. The check also rejects a dead retired tool path or hook check name in hook documentation |
| Whether a `doc-links: ignore` marker states a reason, anywhere in the tree | `./le doc-check links` (`check_ignore_reasons` in `internal/le/doccheck/links.go`). The sweep is over every TRACKED file, not the walked corpus, so a marker nobody's gate reads is still audited |
| Whether every path a TRACKED file names resolves, outside the instruction corpus | `./le doc-check links` (`check_tracked_citations` in `internal/le/doccheck/links.go`). A dead path in any tracked file fails the gate. Repair the reference, or mark its line with a `doc-links: ignore` marker that states why the path cannot resolve. `vendor/` and `third_party/` are excluded because their references point into another repository, and `plan/handover/` because it records the tree as it was. `internal/le/doc_citation_baseline.txt` grandfathers the pairs that predate the check. `check_baseline_growth` compares the pairs against HEAD and refuses each pair HEAD does not hold, so that file only shrinks |
| Whether every symbol a `docs/` source anchor names is declared in the file that anchor points at | `./le doc-check verify` (`check_anchor_symbols` in `internal/le/docstocode/codetodocs.go`). It resolves the tokens after the anchor's `--` against that file's own top-level declarations, and the `report=` argument `main()` passes decides whether a finding is emitted |
| How data flows through a subsystem | read `ai/digests/<subsystem>.md` (living, hand-maintained flow digests; `ai/digests/README.md` lists them); anchors validated by `./le digest` |
| Plugin, command, YANG, and test inventory | `./le inventory`; use `./le inventory | json` for machine-readable output |
| Command inventory | `./le command-list`; use `./le command-list | json` for machine-readable output |
| Spec progress | `./le spec-status`; use `./le spec-status | json` for machine-readable output |
| Generated plugin imports | `./le plugin-imports check` |
| Whether the tree GIT HOLDS compiles | `./le repository-tracked-build check`. It runs in both full verification modes and is a structural gate in `internal/le/commit` |
| Runtime readiness | `ze doctor --json` and `ze explain <diagnostic-code>` |
