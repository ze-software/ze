---
kind: directive
level: MUST
stage:
---
**Anti-pattern:** MUST NOT propose a new direct-call mechanism when DirectBridge
already provides typed handler slots. The bridge struct has `Set*`/`Has*`/call
triplets for each fast-path handler. Adding a new one MUST follow the same
pattern (function type + `atomic.Bool` + `Set`/`Has`/call methods).
