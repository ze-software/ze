---
kind: directive
level: MUST NOT
stage:
---
**These Go patterns MUST NOT be written:** `panic()` for error handling, a discarded error (`f, _ :=`, `_, _ =`) without `//nolint:errcheck // <why>`, global mutable state, `init()` outside a registry pattern, `log.Printf`, a silent default such as `if x == "" { x = "0.0.0.0/0" }`, `os.Getenv("ZE_*")`, `os.Exit()` in a handler, a function whose name joins two responsibilities with `And`, `if end > x { end = x }` where `min` works, and `for i := 0; i < N; i++` where the body never uses `i`.
