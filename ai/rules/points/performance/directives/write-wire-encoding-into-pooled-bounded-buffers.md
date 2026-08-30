---
kind: directive
level: MUST
stage:
rationale: ai/rationale/buffer-first.md
---
**All wire encoding MUST write into a pooled, bounded buffer the CALLER owns: the callee writes into `buf[off:]`, returns the bytes written, and allocates nothing.** Encoding code MUST NOT call `append(buf, ...)`, `make([]byte, N)` in a helper, `buildFoo() ([]byte, error)`, `.Bytes()`, `.Pack()`, `x.Len()` before `x.WriteTo()` on a hot path, or a hand-built `BufHandle{Buf: make(...)}`, and MUST use an offset write, a pool-issued handle, `writeFoo(buf, off) int`, `.WriteTo(buf, off)` and skip-and-backfill for a length field instead. `make([]byte, N)` stays correct in a pool `New` func, session buffer creation, cached encoding, a result copy handed to a caller, JSON marshaling, tests, IPC framing and config parsing. Every buffer in one pool is the same maximum size; the model behind all of it is `docs/architecture/buffer-architecture.md`, and `/ze-find-alloc` and `/ze-fix-alloc` audit and repair a path.
