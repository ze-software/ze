---
kind: directive
level:
stage:
---
**Anti-pattern:** Proposing a new direct-call mechanism when DirectBridge already
provides typed handler slots. The bridge struct has `Set*`/`Has*`/call triplets
for each fast-path handler. Adding a new one follows the same pattern (function
type + `atomic.Bool` + `Set`/`Has`/call methods).
