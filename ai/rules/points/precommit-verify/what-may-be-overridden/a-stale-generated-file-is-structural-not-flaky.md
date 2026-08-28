---
kind: note
level:
stage:
---
`./le repository generated-check` qualifies on the rule's own terms: a stale generated
file is deterministic, reproducible, and fixed by `./le repository generate` (or the
specific `--fix` the failing check names). It is never flaky or environmental.
