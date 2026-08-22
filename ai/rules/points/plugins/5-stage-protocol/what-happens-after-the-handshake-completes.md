---
kind: note
level:
stage:
---
After Stage 5: SDK wraps Socket A in `MuxConn` for concurrent RPCs. Engine dispatches Socket A requests in goroutines. Wire format: `#<id> <verb> [<json>]\n` (see `docs/architecture/api/wire-format.md`).
