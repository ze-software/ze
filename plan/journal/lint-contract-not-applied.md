# lint contract not applied

A test or helper changes in a way that violates an existing lint contract. The
contract is correct, and the failure is fixed at the source rather than bypassed.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-17 | spec-rfc4271-med-ibgp-readvertisement | scripts/checks staticcheck feature matrix tests | `TestStaticcheckFeatureMatrixCompletedVerdictWinsExpiredContext` called `t.Parallel` inside subtests but not on the parent test, so `make ze-lint-changed` failed with `tparallel` before the MED commit could be prepared | added top-level `t.Parallel` to the parent test |
