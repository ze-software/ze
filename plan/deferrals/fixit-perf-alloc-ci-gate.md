# Deferrals: fixit-perf-alloc-ci-gate

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-perf-alloc-ci-gate functional-proof | the retired make ze-alloc-check (current: ./le verify deps alloc) full run (needs bench log) deferred to CI/nightly | live-server/QEMU constraint, deferred to CI | plan/future/spec-ci-coverage-remaining-surfaces.md | done |



Closed 2026-08-29 after verifying the producer rather than the row: `verify deps/alloc` is a stage of `./le verify list mode full`, sharded by `.github/workflows/verify.yml`.
