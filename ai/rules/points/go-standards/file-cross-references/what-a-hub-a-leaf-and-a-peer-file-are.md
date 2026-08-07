---
kind: directive
level:
stage:
---
**Hub file** = orchestrator, core types, dispatch (typically shortest name: `server.go`, `decode.go`, `peer.go`).
**Leaf file** = specific concern split from hub (has suffix: `_text`, `_routes`, `_batch`, or prefix: `cmd_`).
**Peer files** = siblings at same abstraction level, neither contains the other.
