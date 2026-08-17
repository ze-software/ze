# lint contract not applied

A test or helper changes in a way that violates an existing lint contract. The
contract is correct, and the failure is fixed at the source rather than bypassed.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-17 | spec-rfc4271-med-ibgp-readvertisement | scripts/checks staticcheck feature matrix tests | `TestStaticcheckFeatureMatrixCompletedVerdictWinsExpiredContext` called `t.Parallel` inside subtests but not on the parent test, so `make ze-lint-changed` failed with `tparallel` before the MED commit could be prepared | added top-level `t.Parallel` to the parent test |
| 2026-08-17 | spec-fixit-zefs-diff-structural-ops | bgp message RFC 7606 receive test | `internal/component/bgp/message/rfc7606_section51_receive_test.go` was committed un-gofmt'd at line 26, so `make ze-lint-changed` failed with `gofmt` for every session whose changed set reached the package. The file is clean in the working tree, so the red comes from HEAD, not from another session | ran `gofmt -w` on the file |
| 2026-08-17 | none (owner directive: a Go commit owes a full verify) | verify runner tests | `scripts/status/verify_run_test.go` is un-gofmt'd in HEAD at line 82, so `make ze-lint-changed` failed with `gofmt` as soon as any edit put the file in the changed set. Third row of this shape today: an un-gofmt'd file reaches HEAD and lies dormant until somebody else's change pulls its package into the changed set | ran `gofmt -w` on the file |
