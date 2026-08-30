---
kind: directive
level: MUST
stage:
---
**A commit script MUST be prepared with `./le commit create`, and the path its `script=` line prints MUST be the one that is run.** `internal/le/commit` owns session id reuse, message creation, explicit add and remove validation, script generation and the pre-staging gates. A hand-written compatibility path MUST NOT be used.
