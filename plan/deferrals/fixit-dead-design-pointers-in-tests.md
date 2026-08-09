# Deferrals: fixit-dead-design-pointers-in-tests

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-09 | `plan/spec-problem-journal.md` | 133 `// Design: plan/spec-*.md` pointers in `_test.go` files name specs removed at closure; `go_files()` in `scripts/dev/check_doc_links.py` excludes `_test.go`, so no gate sees them | Pre-existing and independent of the learned-corpus move. The goal of the work in hand does not depend on it, so it gets a spec rather than a fix folded into that commit (`ai/rules/completion.md`) | `plan/spec-fixit-dead-design-pointers-in-tests.md` | open |
