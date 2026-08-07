---
kind: note
level: MUST
stage:
---
All goroutines MUST be long-lived workers. Never per-event goroutines in hot paths.
Rationale: `ai/rationale/goroutine-lifecycle.md`
