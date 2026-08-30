---
kind: directive
level: MUST NOT
stage:
---
- **A new direct-call mechanism MUST NOT be proposed where DirectBridge already provides typed handler slots.** The bridge struct carries a `Set*`/`Has*`/call triplet for each fast-path handler, and a new one MUST follow that same pattern: a function type, an `atomic.Bool`, and the `Set`/`Has`/call methods.
