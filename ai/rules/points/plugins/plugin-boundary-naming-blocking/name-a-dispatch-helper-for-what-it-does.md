---
kind: directive
level: MUST
stage:
---
- **A helper that sends a command through `DispatchCommand` MUST be named for what it does, never for where it sends it.** The engine routes by prefix, so a function, variable or type name MUST NOT encode the destination: `dispatchCommand`, never `dispatchRIBCommand`.
