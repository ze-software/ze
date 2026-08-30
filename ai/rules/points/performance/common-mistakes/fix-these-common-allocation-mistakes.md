---
kind: directive
level: MUST NOT
stage:
---
**A per-UPDATE, per-route or per-NLRI function MUST NOT allocate: no `make([]byte, n)`, no `func Encode() []byte`, no `fmt.Sprintf`, no `.String()` in a loop, no `[]string` plus `strings.Join`.** It MUST take a pool buffer or the caller's buffer and return the bytes written, and a `WireUpdate` MUST NOT be held past the return of the pool buffer its payload references. The mistake-and-fix table, with the cost of each, is `docs/architecture/buffer-architecture.md`.
