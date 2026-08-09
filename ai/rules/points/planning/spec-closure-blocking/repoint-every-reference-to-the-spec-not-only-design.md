---
kind: directive
level:
stage:
---
**EVERY reference survives closure, not only `// Design:`.** Before commit B, grep
the WHOLE PATH `plan/spec-<stem>.md` across the tree, not the `// Design:` prefix,
and rewrite every hit to the appropriate destination (an `ai/rules/` file, a
`docs/architecture/` page, or one of the three `plan/learned/` aggregates:
`DESIGN-HISTORY.md`, `HOOK-FRICTION.md`, `RECURRING-PATTERNS.md`) inside commit A.
