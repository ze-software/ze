---
kind: directive
level: MUST
stage:
---
**A change to a `*.go` file under `internal/` MUST also check everything its row names.**

| What changed | Also check |
|---|---|
| New exported symbol | Its wiring: who calls it (`ai/rules/completion.md`) |
| Modified function signature | Every caller (`gopls references`, or a grep) |
| New goroutine | `ai/rules/goroutine-lifecycle.md`, and cleanup on shutdown |
| New `make([]byte, N)` on a wire path | The pool-backed alternative (`ai/rules/performance.md`) |
| New `fmt.Sprintf` | The append-based alternative (`ai/rules/performance.md`) |
| A guard or fallback added | The sibling call-site audit above |
| An error return ignored | `./le verify lint run` reports the errcheck finding |
