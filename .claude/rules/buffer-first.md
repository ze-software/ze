# Buffer-First Encoding

**BLOCKING:** All wire encoding MUST write into pooled, bounded buffers.
Rationale: `.claude/rationale/buffer-first.md`

## Design

Pool buffer = RFC max length = bounded encoding space. Buffer size enforces message limit.

| Pool | Size | Purpose |
|------|------|---------|
| `readBufPool4K` | 4096 | Standard message reads |
| `readBufPool64K` | 65535 | Extended message reads |
| `buildBufPool` | 4096 | UPDATE building |
| Per-plugin `nlriBufPool` | 4096 | NLRI encoding |

## Pattern: Get → Write → Put

1. Get buffer from pool
2. Write with `WriteTo(buf, off) int` — advance offset
3. Return buffer to pool after data consumed

## Banned in Encoding Code

| Banned | Use Instead |
|--------|-------------|
| `append(buf, ...)` | Pre-computed size + write at offset |
| `make([]byte, N)` in helpers | Write into caller's pool buffer |
| `buildFoo() ([]byte, error)` | `writeFoo(buf, off) int` |
| `.Bytes()` | `.WriteTo(buf, off)` + `.Len()` |
| `.Pack()` | `.WriteTo(buf, off)` |

## When make([]byte) IS OK

Pool `New` func, session buffer creation, cached encoding, result copies to callers, JSON marshaling, tests, IPC framing, config parsing.

## Before Writing Encoding Code

1. Where does buffer come from? → Pool or caller-provided
2. Using `append()`? → Use offset writes
3. Returning `[]byte` from helper? → Refactor to `writeFoo(buf, off) int`
4. Calling `make([]byte)`? → Get from pool
5. Type has `WriteTo`? → Use it

Enforced by hook `block-encoding-alloc.sh` (exit 2). Audit: `/find-alloc`. Fix: `/fix-alloc file:line`.
