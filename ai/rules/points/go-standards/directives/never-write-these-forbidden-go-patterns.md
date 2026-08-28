---
kind: directive
level: MUST
stage:
---
**Code MUST NOT write these forbidden Go patterns:**
- `panic()` for error handling. The native Write/Edit gate in `internal/le/hookruntime/writeedit.go` blocks a new `panic()` call. Return an error from operating paths; reserve `panic("BUG: <what>")` for a programmer-error invariant only where the owning rule permits it
- `f, _ := func()` and `_, _ = func()` (ignoring errors). If you genuinely MUST discard, use `//nolint:errcheck // <why>` with a specific reason
- Global mutable state
- `init()` except registry patterns
- `log.Printf` (legacy log package)
- Silent defaults: `if x == "" { x = "0.0.0.0/0" }`
- `os.Getenv("ZE_*")` or `os.Getenv("ze.*")` -- use `env.Var()` instead
- `if end > x { end = x }` when clamping an int, use `end = min(end, x)` (Go 1.21+ built-in)
- `for i := 0; i < N; i++` when the body does not use `i` as anything but a counter, use `for range N` (Go 1.22+)
