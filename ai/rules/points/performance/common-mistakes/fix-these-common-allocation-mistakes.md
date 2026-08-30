---
kind: directive
level: MUST NOT
stage:
---
- **A per-UPDATE, per-route or per-NLRI function MUST NOT allocate: no `make([]byte, n)`, no `func Encode() []byte`, no `fmt.Sprintf`, no `.String()` in a loop, no `[]string` plus `strings.Join`. It MUST take a pool buffer or the caller's buffer, and MUST return bytes written.**
- **A `BufHandle` MUST NOT be hand-built. `BufHandle{Buf: make(...)}` names no block and corrupts pool tracking, so only a pool-issued handle is valid. `writeGoPatterns` in `internal/le/hookruntime/writeedit.go` refuses both at edit time.**
- **A `WireUpdate` MUST NOT be held past the return of the pool buffer its payload references. Anything still needed MUST be copied first.**
- The full mistake-and-fix table, with the reason each one costs, is `docs/architecture/buffer-architecture.md` ("Common allocation mistakes").
