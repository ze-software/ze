# Deferrals: fixit-runner-kill-background

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-runner-kill-background functional-proof | AC-4 IPsec DPD e2e depends on plugin-event spec; stop-background.ci live run deferred to CI | live-server/QEMU constraint, deferred to CI | plan/future/spec-ci-coverage-remaining-surfaces.md | done |



Closed 2026-08-29 after verifying the producer rather than the row: `functional` is a full-mode verify stage run by `verify.yml` on push and PR, through `suiteRunner` (`internal/le/functional/suites.go`).
