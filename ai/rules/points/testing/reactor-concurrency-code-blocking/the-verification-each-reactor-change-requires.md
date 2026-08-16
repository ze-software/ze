---
kind: table
level:
stage:
---
| Touched | Required verification |
|---------|----------------------|
| `session*.go` lock acquire/release, field assign | `make ze-unit-reactor-test-race` |
| `forward_pool*.go` worker drain or buffer release | `make ze-unit-reactor-test-race` |
| New goroutine in reactor package | `make ze-unit-reactor-test-race` |
| Any reactor field shared between Run loop and other goroutines | `make ze-unit-reactor-test-race` |
| Reactor doc-only edits, log message changes | Not required |
