---
kind: table
level:
stage:
---
| To close as | Set Status to | Set Destination to |
|-------------|---------------|--------------------|
| Implemented | `done` | Spec or commit where implemented |
| User decided not to do it | `cancelled` | `user-approved-drop` |
| Superseded (another row or spec now owns it) | `done` | The row or spec that took it over |
