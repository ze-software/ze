# Deferrals: fixit-doc-gate-and-refs

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-doc-gate-and-refs functional-proof | the optional local check-doc-drift.sh hook is owned by a separate follow-up | live-server/QEMU constraint, deferred to CI. **CLOSED 2026-08-30: the artifact this row defers no longer exists.** `check-doc-drift.sh` was retired with the other scripts in `eae282592`, and the check is native and locally runnable as `./le docvalid doc-drift` (`internal/le/docvalid/drift.go`). It needs neither a live server nor QEMU, which was this row's stated reason for deferring it to CI | plan/future/spec-ci-coverage-remaining-surfaces.md | done |

