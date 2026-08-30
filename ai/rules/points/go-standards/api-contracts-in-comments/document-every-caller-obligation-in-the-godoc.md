---
kind: directive
level: MUST
stage:
rationale: ai/rationale/api-contracts.md
---
**A function with a caller obligation MUST document it in its godoc, and MUST state it on BOTH sides of a pair.** The trigger is any function where skipping a step causes a resource leak, a deadlock, a panic, or silent misbehavior. The comment each lifecycle pattern owes is in `docs/contributing/go-conventions.md`.
