# Deferrals: fixit-agent-tooling-misleads

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | plan/spec-fixit-agent-tooling-misleads (T-4/T-5, found while writing that spec) | Two more surfaces assuming every spec is about the daemon: the spec-write gate accepts ONLY `.go` under internal/pkg/cmd as evidence of investigation, so a spec about Python or shell cannot satisfy it by reading its own subject (writing that spec required reading an unrelated Go file to pass); and the validator demands a `.ci` in Functional Tests, which a hooks spec cannot have, since `.ci` drives the daemon and hooks never touch it. That spec ships with its validator red as the evidence | Filed, not fixed: both gates are CORRECT in intent and must be SCOPED rather than relaxed. The spec-write gate exists because inference-written specs cost ten false premises on 2026-07-16; the `.ci` rule exists because unit tests alone let 30 tests ship that never bound a peer (`aaefef8ce`). Each needs a must-not-fire test proving it still rejects what it should | `plan/spec-finish-ci-coverage.md` (T-4, T-5) | deferred |
| 2026-07-19 | spec-fixit-agent-tooling-misleads functional-proof | none beyond the corrected gates | live-server/QEMU constraint, deferred to CI | plan/spec-finish-ci-coverage.md | deferred |

