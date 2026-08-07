---
kind: directive
level:
stage:
---
**EVERY reference survives closure, not only `// Design:`.** Before commit B, grep
the WHOLE PATH `plan/spec-<stem>.md` across the tree, not the `// Design:` prefix,
and rewrite every hit to the learned summary (`plan/learned/NNN-<name>.md`)
inside commit A.
