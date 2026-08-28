---
kind: note
level:
stage:
---
`./le test-sensitivity check` (stage 10 of `./le verify worktree`, both modes) counts
them and enforces committed floors in `test/health/sensitivity-baseline.json`. The
counts may only go DOWN, following the `test/.ci-sleep-baseline` convention.
