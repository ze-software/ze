---
kind: directive
level: MUST
stage:
---
**The verification each row names MUST run before the change is claimed done.**

| Touched | Required verification |
|---------|----------------------|
| `session*.go` lock acquire/release, field assign | `go test -race -count=20 ./internal/component/bgp/reactor/...` |
| `forward_pool*.go` worker drain or buffer release | same repeated reactor race proof |
| New goroutine in reactor package | same repeated reactor race proof |
| Any reactor field shared between Run loop and other goroutines | same repeated reactor race proof |
| Reactor doc-only edits, log message changes | Not required |
