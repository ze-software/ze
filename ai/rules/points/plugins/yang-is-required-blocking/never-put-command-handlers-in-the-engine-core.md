---
kind: directive
level:
stage:
---
**Anti-pattern:** Placing command handlers in reactor/ (couples engine core to command surface), in a separate handler/ package (middleman), or in a `command/` folder (formalizes missing YANG as acceptable). Commands belong in `plugins/` with YANG schemas.
