---
kind: table
level:
stage:
---
| Goroutine pattern | Pool strategy | Example |
|---|---|---|
| Single goroutine, sequential processing | Ring buffer (fixed array, index rotation) | `peerPool` -- reactor loop processes one UPDATE at a time per peer |
| Multiple goroutines, concurrent access | `sync.Pool` seeded for peak | `readBufPool` -- multiple peers reading concurrently |
