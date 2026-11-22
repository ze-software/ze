---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/quality.md
---
**MUST fix lint issues your change introduces and re-run the linter on the fixed packages before claiming work complete; pre-existing or unrelated lint reds do not block a pre-release commit (`ai/rules/pre-release.md`), and linters MUST NOT be disabled.** Only exclusions: `fieldalignment` (govet), test-file exclusions for `dupl`, `goconst`, `prealloc` and `gosec`.
