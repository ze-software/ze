---
kind: directive
level: MUST
stage:
---
- **Closure MUST run in this order: `in-progress`, then a clean Review Gate, then the journal row, then `git rm` of the spec.** A completed spec left in `plan/` MUST NOT happen: every future session counts it as open work.
