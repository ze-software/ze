---
kind: directive
level: MUST
stage:
---
**You MUST delete a shard at closure only when all rows are terminal. A homed live row MUST remain until its destination is complete. You MUST run `internal/le/commit` instead of counting rows.**
