---
kind: table
level:
stage:
---
| Type | Location | Purpose | Lifecycle |
|---|---|---|---|
| `WireUpdate` | `wireu/wire_update.go` | Lazy-parsed UPDATE message | Lives as long as the readBuf it references |
| `PackContext` | `attribute/pack_context.go` | Negotiated capabilities for encoding | Per-peer, created from OPEN exchange |
| `ContextID` | `context/registry.go` | Compact uint16 identifying encoding context | Same ID = same encoding = zero-copy forward |
| `BufHandle` | `reactor/bufmux.go` | Reference to a pool-managed buffer | Tracks which pool + slot owns the buffer |
| `BufWriter` | `wire/writer.go` | Interface: `WriteTo(buf, off) int` | Implemented by all wire-encodable types |
