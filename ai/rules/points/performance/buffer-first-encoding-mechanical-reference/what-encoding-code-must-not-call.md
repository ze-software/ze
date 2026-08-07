---
kind: table
level:
stage:
---
| Banned | Use Instead |
|--------|-------------|
| `append(buf, ...)` | Pre-computed size + write at offset |
| `make([]byte, N)` in helpers | Write into caller's pool buffer |
| `buildFoo() ([]byte, error)` | `writeFoo(buf, off) int` |
| `.Bytes()` | `.WriteTo(buf, off)` + `.Len()` |
| `.Pack()` | `.WriteTo(buf, off)` |
| `x.Len()` then `x.WriteTo()` in hot path | Skip-and-backfill, or `WriteAttrToWithLen()` |
