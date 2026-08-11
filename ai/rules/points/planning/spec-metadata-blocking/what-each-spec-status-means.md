---
kind: table
level:
stage:
---
| Status | Meaning |
|--------|---------|
| `skeleton` | Task defined, design not started |
| `design` | Research/design in progress |
| `ready` | Design complete, ready for implementation |
| `in-progress` | Actively being implemented |
| `verification` | Implementation complete and COMMITTED, awaiting an independent review and closure on Opus 5. Reached only under `Handoff: verify` (see "Two-Session Handoff") |
| `blocked` | Waiting on prerequisite (see Depends) |
| `deferred` | Explicitly postponed |
