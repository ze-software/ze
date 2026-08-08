# Deferrals: fixit-ospf-sr-missing-label-passes

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** an ospf-sr scenario downgrades a missing MPLS label and passes

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | ad-hoc (found while auditing interop retry loops for swallowed failures) | `test/interop/scenarios/ospf-sr-frr/check.py` step 3 downgrades a MISSING MPLS label 16100 to `log_info` and passes. A scenario that cannot find the label it exists to prove reports success | Same class as the `docker_exec_quiet` fail-open already homed as Guard 3 and Guard 4, and separable from them: this one is a single scenario's own assertion rather than a shared helper's contract. The trailing-value goal holds without it | `plan/spec-fixit-test-harness-fail-open-guards.md` | deferred |
