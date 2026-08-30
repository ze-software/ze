---
kind: directive
level: MUST NOT
stage:
---
**Encoding code MUST NOT call anything in the left column. The right column is what it MUST call instead.**

| Banned | Use Instead |
|--------|-------------|
| `append(buf, ...)` | Pre-computed size, then a write at the offset |
| `make([]byte, N)` in a helper | A write into the caller's pool buffer |
| `buildFoo() ([]byte, error)` | `writeFoo(buf, off) int` |
| `.Bytes()` | `.WriteTo(buf, off)` plus `.Len()` |
| `.Pack()` | `.WriteTo(buf, off)` |
| `x.Len()` then `x.WriteTo()` on a hot path | Skip-and-backfill, or `WriteAttrToWithLen()` |
| `BufHandle{Buf: make(...)}` | A pool-issued handle only; a hand-built one names no block and corrupts pool tracking |
