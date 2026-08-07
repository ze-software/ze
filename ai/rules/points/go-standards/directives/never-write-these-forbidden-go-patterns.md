---
kind: directive
level:
stage:
---
- `panic()` for error handling. Allowed prefixes (enforced by `block-panic-error.sh`): `panic("BUG: ...")`, `panic("unreachable: ...")`, `panic("not implemented")`, `panic("unimplemented")`, `panic("TODO: ...")`, `panic("impossible: ...")`. Use `panic("BUG: <what>")` for programmer-error guards that must never fire at runtime. Any other `panic()` call is rejected at Write/Edit time (test files and `scripts/` excepted)
- `f, _ := func()` and `_, _ = func()` (ignoring errors). If you genuinely must discard, use `//nolint:errcheck // <why>` with a specific reason
- Global mutable state
- `init()` except registry patterns
- `log.Printf` (legacy log package)
- Silent defaults: `if x == "" { x = "0.0.0.0/0" }`
- `os.Getenv("ZE_*")` or `os.Getenv("ze.*")` -- use `env.Var()` instead
- `if end > x { end = x }` when clamping an int, use `end = min(end, x)` (Go 1.21+ built-in)
- `for i := 0; i < N; i++` when the body does not use `i` as anything but a counter, use `for range N` (Go 1.22+)
