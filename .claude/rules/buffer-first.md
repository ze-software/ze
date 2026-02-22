# Buffer-First Encoding

**BLOCKING:** All wire encoding MUST write into pooled, bounded buffers.
Rationale: `.claude/rationale/buffer-first.md`

Pool buffer = RFC max length = bounded encoding space.

| Pool | Size | Purpose |
|------|------|---------|
| `readBufPool4K` | 4096 | Standard message reads |
| `readBufPool64K` | 65535 | Extended message reads |
| `buildBufPool` | 4096 | UPDATE building |
| Per-plugin `nlriBufPool` | 4096 | NLRI encoding |

## Pattern: Get → Write → Put

Get buffer from pool → write with `WriteTo(buf, off) int` → return to pool.

## Banned in Encoding Code

| Banned | Use Instead |
|--------|-------------|
| `append(buf, ...)` | Pre-computed size + write at offset |
| `make([]byte, N)` in helpers | Write into caller's pool buffer |
| `buildFoo() ([]byte, error)` | `writeFoo(buf, off) int` |
| `.Bytes()` | `.WriteTo(buf, off)` + `.Len()` |
| `.Pack()` | `.WriteTo(buf, off)` |

## `make([]byte)` IS OK For

Pool `New` func, session buffer creation, cached encoding, result copies to callers, JSON marshaling, tests, IPC framing, config parsing.

## Before Writing Encoding Code

1. Buffer from? → Pool or caller-provided
2. `append()`? → Offset writes
3. Returning `[]byte` from helper? → `writeFoo(buf, off) int`
4. `make([]byte)`? → Get from pool
5. Type has `WriteTo`? → Use it

Enforced by `block-encoding-alloc.sh` (exit 2). Audit: `/find-alloc`. Fix: `/fix-alloc file:line`.
