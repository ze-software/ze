---
kind: directive
level: MUST
stage:
---
**A new exported symbol MUST have a non-test caller. Grep it across `internal/` and `cmd/`: if the only hits are its definition and test files, it is dead code, and dead code is a BLOCKER rather than a NOTE.**
