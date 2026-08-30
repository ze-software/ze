---
kind: directive
level: MUST NOT
stage:
---
**A `./le repository generated-check` red MUST NOT be treated as flaky or
environmental.** A stale generated file is deterministic and reproducible, and it
is fixed by `./le repository generate`, or by the specific `--fix` the failing
check names.
