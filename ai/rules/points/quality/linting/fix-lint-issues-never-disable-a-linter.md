---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/quality.md
---
**MUST fix lint issues. MUST NOT disable linters.** Only exclusions: `fieldalignment` (govet), test-file exclusions for `dupl`, `goconst`, `prealloc` and `gosec`.
