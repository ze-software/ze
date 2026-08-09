# Deferrals: fixit-unexport-package-private-symbols

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-09 | `plan/spec-problem-journal.md` | 467 `exported symbol X has no cross-package non-test caller` findings from `make ze-validate` | Pre-existing over-exported API surface, surfaced because a 1155-file comment sweep pulled nearly the whole tree into `check_cross_package_wiring`'s changed-file scope. A comment cannot create an unwired symbol, so none of it was introduced by that work | `plan/spec-fixit-unexport-package-private-symbols.md` | open |
