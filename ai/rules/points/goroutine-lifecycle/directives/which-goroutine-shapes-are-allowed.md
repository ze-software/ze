---
kind: table
level:
stage:
---
| Pattern | Status |
|---------|--------|
| Long-lived goroutine reading from channel | Required |
| Goroutine per lifecycle (process, session, peer) | OK |
| Goroutine per event in hot path | Forbidden |
| `go func()` inside `for range` on events | Forbidden |
