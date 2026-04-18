---
paths:
  - "**/*.go"
---

# Buffer-First Encoding -- Mechanical Reference

**BLOCKING:** All wire encoding MUST write into pooled, bounded buffers.
Principle: `rules/design-principles.md` -- Encapsulation onion + Buffer-first encoding.
Rationale: `.claude/rationale/buffer-first.md`

| Pool | Size | Purpose |
|------|------|---------|
| `readBufPool4K` | 4096 | Standard message reads |
| `readBufPool64K` | 65535 | Extended message reads |
| `buildBufPool` | 4096 | UPDATE building |
| Per-plugin `nlriBufPool` | 4096 | NLRI encoding |

## Pattern: Get → Write → Put

Get buffer from pool → write with `WriteTo(buf, off) int` → return to pool.

## Pattern: Skip-and-Backfill (hot path)

For messages with variable-length sections and fixed-position length fields:

1. Write fixed bytes (marker, type)
2. **Skip** length field — save position (`lengthPos := off; off += 2`)
3. Write payload forward at advancing offset
4. **Backfill** length at saved position (`buf[lengthPos] = byte(totalLen >> 8)`)

This avoids the `Len()`-then-`WriteTo()` double traversal. See `reactor_wire.go` for the canonical implementation.

## Banned in Encoding Code

| Banned | Use Instead |
|--------|-------------|
| `append(buf, ...)` | Pre-computed size + write at offset |
| `make([]byte, N)` in helpers | Write into caller's pool buffer |
| `buildFoo() ([]byte, error)` | `writeFoo(buf, off) int` |
| `.Bytes()` | `.WriteTo(buf, off)` + `.Len()` |
| `.Pack()` | `.WriteTo(buf, off)` |
| `x.Len()` then `x.WriteTo()` in hot path | Skip-and-backfill, or `WriteAttrToWithLen()` |

## `make([]byte)` IS OK For

Pool `New` func, session buffer creation, cached encoding, result copies to callers, JSON marshaling, tests, IPC framing, config parsing.

## Before Writing Encoding Code

1. Buffer from? → Pool or caller-provided
2. `append()`? → Offset writes
3. Returning `[]byte` from helper? → `writeFoo(buf, off) int`
4. `make([]byte)`? → Get from pool
5. Type has `WriteTo`? → Use it

Enforced by `block-encoding-alloc.sh` (exit 2). Audit: `/ze-find-alloc`. Fix: `/ze-fix-alloc file:line`.

## Text/JSON Format Generation

A sibling hook `block-format-alloc.sh` (exit 2) guards the BGP text/JSON
format-generation files migrated by fmt-0 and fmt-2-json-append: every file
that emits OPEN / NOTIFICATION / ROUTE-REFRESH / NEGOTIATED text or JSON is
allowlisted, and `fmt.Sprintf`, `fmt.Fprintf`, `strings.Builder`,
`strings.Join`, `strings.NewReplacer`, `strings.ReplaceAll`,
`strconv.FormatUint`, `strconv.FormatInt` are rejected at Write/Edit time.
Allowed helpers: `strconv.AppendUint`, `netip.Addr.AppendTo`,
`hex.AppendEncode`, or a local `[N]byte` scratch plus `append`. `json.go`
is intentionally excluded while its `map[string]any` + `json.Marshal`
idiom remains.
