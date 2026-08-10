---
kind: directive
level: MUST
stage:
---
**A homed row MUST stay live.** The status answers "is this work still outstanding",
NOT "does it have a home". Homing is mandatory, so a live row is the NORMAL,
correct state of a deferral: it has a spec AND the work has not landed yet. It goes
`done` when the work is implemented, not when it is filed. A live row is not a
violation and is not a backlog of unfiled work.
