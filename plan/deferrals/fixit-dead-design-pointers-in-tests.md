# Deferrals: fixit-dead-design-pointers-in-tests

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-09 | `plan/spec-problem-journal.md` | 133 `// Design: plan/spec-*.md` pointers in `_test.go` files name specs removed at closure; `go_files()` in the retired `scripts/dev/check_doc_links.py` (current producer: `internal/le/doc/check/links.go`) excludes `_test.go`, so no gate sees them | Pre-existing and independent of the learned-corpus move. The goal of the work in hand does not depend on it, so it gets a spec rather than a fix folded into that commit (`ai/rules/completion.md`) | `plan/spec-fixit-dead-design-pointers-in-tests.md` | done |
| 2026-08-09 | `plan/spec-fixit-dead-design-pointers-in-tests.md` phase 3 | `./le spec citation anchors` is red at HEAD with 12 dangling `plan/spec-*.md` citations, so `./le doc wiring` cannot go green. `find_dangling` in the retired `scripts/dev/spec-citation-check.py` (current producer: `internal/le/spec/citation/speccitation.go`) is correct; nothing adds a closed stem to `plan/.citation-baseline` | Pre-existing at HEAD and independent: phase 3 touched no `plan/` file and cleared the Design gate it owns. The goal of the work in hand does not depend on it, so it gets a spec (`ai/rules/completion.md`) | `spec-fixit-spec-closure-leaves-dangling-spec-citations`, implemented and closed 2026-08-10 in `67d207a53` | closed |


Closed 2026-08-29 after verifying the producer rather than the row: the population fell from 133 to 2, and both survivors are fixtures of the gate itself (`sweepTracked` and `sweepExcluded`, `internal/le/doc/check/links.go`).
