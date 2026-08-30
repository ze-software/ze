---
kind: directive
level: MUST NOT
stage:
---
**On an explicit commit request, preparing the commit IS the work: a late completeness check, health check, recent-commit style review or remaining-work table MUST NOT be run unless the user asks for one.** `./le verify status check` MUST be run before any verify target, and a FRESH result MUST NOT be followed by a rerun of `./le verify worktree`.
