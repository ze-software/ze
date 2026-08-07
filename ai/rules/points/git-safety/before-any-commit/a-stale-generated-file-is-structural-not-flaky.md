---
kind: note
level:
stage:
---
`ze-regen-check-readonly` qualifies on the rule's own terms: a stale generated
file is deterministic, reproducible, and fixed by `make ze-regen` (or the
specific `--fix` the failing check names). It is never flaky or environmental.
