---
kind: table
level:
stage:
---
| Situation | Use |
|---|---|
| Caller has a buffer in scope | Pass it as parameter |
| Function called from 1-2 call sites | Add buf parameter to those callers |
| Function called from many sites, scratch is internal | `sync.Pool` |
| Buffer needed across goroutines | `sync.Pool` (goroutine-safe) |
| Single goroutine, sequential processing | Ring buffer or struct field |
