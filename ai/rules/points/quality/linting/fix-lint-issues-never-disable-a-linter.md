---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/quality.md
---
**MUST fix lint issues and re-run the linter on the fixed packages before committing or claiming work complete. MUST NOT disable linters.** Only exclusions: `fieldalignment` (govet), test-file exclusions for `dupl`, `goconst`, `prealloc` and `gosec`.
