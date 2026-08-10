---
kind: directive
level: MUST
stage:
---
**Anti-pattern:** command handlers MUST NOT be placed in reactor/ (couples engine core to command surface), in a separate handler/ package (middleman), or in a `command/` folder (formalizes missing YANG as acceptable). Commands MUST belong in `plugins/` with YANG schemas.
