---
kind: directive
level: MUST NOT
stage:
---
- **A hand-maintained second list of gate tags or gated packages MUST NOT exist anywhere.**
- **A feature type MUST NOT appear in an always-on signature.** Widen the always-on handle to `Reconfigurable` or another always-on interface.
- **A gate MUST NOT be added without present and absent build-tag tests plus an `nm` symbol check.** The absent test asserts that zero feature symbols are linked.
