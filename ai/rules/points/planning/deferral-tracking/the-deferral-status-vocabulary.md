---
kind: table
level: MUST
stage:
---
| Status | Meaning | Destination checked? |
|--------|---------|----------------------|
| `deferred` | Live: the work is outstanding. MUST name its home spec | YES |
| `open` | Live: synonym of `deferred`. Prefer `deferred` | YES |
| `done` | Terminal. The work landed, or the row was superseded | no |
| `cancelled` | Terminal. User decided not to do it | no |
| `resolved` | Terminal. Closed with evidence (learned summary) | no |
