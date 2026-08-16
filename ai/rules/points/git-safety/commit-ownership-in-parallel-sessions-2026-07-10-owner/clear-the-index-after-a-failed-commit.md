---
kind: directive
level: MUST
stage:
---
**A FAILED commit leaves the index STAGED, and the next session's commit inherits
it. You MUST clear it before you walk away.** The script stages first and commits second, so
a failed commit can leave foreign files staged in the shared index.
