---
kind: directive
level: MUST NOT
stage:
---
- **A row MUST NOT be closed early to quiet the Stop-hook count.** One observer reads these rows and it only counts them, so an early close hides the work from the only thing watching it. Homing a deferral is your obligation, not something a gate enforces (`docs/contributing/spec-workflow.md`).
