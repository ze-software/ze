---
kind: note
level:
stage:
---
`make ze-test-sensitivity-check` (stage 10 of `make ze-precommit-verify`, both modes) counts
them and enforces committed floors in `test/health/sensitivity-baseline.json`. The
counts may only go DOWN, following the `test/.ci-sleep-baseline` convention.
