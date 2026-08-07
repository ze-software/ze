---
kind: table
level:
stage:
---
| Touched | Required verification |
|---------|----------------------|
| `session*.go` lock acquire/release, field assign | `make ze-race-reactor` |
| `forward_pool*.go` worker drain or buffer release | `make ze-race-reactor` |
| New goroutine in reactor package | `make ze-race-reactor` |
| Any reactor field shared between Run loop and other goroutines | `make ze-race-reactor` |
| Reactor doc-only edits, log message changes | Not required |
