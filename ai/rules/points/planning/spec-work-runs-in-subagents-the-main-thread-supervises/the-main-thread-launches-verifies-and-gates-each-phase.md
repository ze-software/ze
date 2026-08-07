---
kind: directive
level:
stage:
---
**The main thread supervises. It does not perform the spec work itself.** Most phases run in a subagent invoked through their `ze-*` skill, and the main thread launches each one, reads the report back, verifies it, decides, and gates the next phase. The `Runs in` column names the four exceptions, so read it before you delegate.
