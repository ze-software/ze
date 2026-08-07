---
kind: table
level:
stage:
---
| Pool | Location | Shape | Purpose |
|---|---|---|---|
| `readBufPool4K` | reactor | `sync.Pool` | Standard message TCP reads |
| `readBufPool64K` | reactor | `sync.Pool` | Extended message TCP reads |
| `buildBufPool` | reactor | `sync.Pool` | UPDATE building workspace |
| `peerPool` (per-peer) | `forward_pool.go` | Ring (64 slots) | Per-peer inbound/outbound buffers |
| `MixedBufMux` | `bufmux.go` | Byte-budgeted | Global overflow, mixed block sizes |
| `textbuf.Buffer` pool | `internal/core/textbuf` | `sync.Pool` | String building (via `Get()`/`Release()`) |
| Attribute pools | `attrpool/` | Per-type dedup slabs | RIB attribute deduplication |
| NLRI pools | per-family plugins | Per-family dedup | RIB NLRI deduplication |
