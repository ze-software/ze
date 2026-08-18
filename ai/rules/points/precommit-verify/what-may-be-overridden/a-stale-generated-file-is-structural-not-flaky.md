---
kind: note
level:
stage:
---
`ze-generated-files-check` qualifies on the rule's own terms: a stale generated
file is deterministic, reproducible, and fixed by `make ze-generated-files-update` (or the
specific `--fix` the failing check names). It is never flaky or environmental.
