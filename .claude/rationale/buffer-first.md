# Buffer-First Rationale

Why: `.claude/rules/buffer-first.md`

## Why Pool Buffer = RFC Length

BGP messages: 4096 bytes standard, 65535 extended (RFC 8654). Getting a 4096-byte buffer from sync.Pool simultaneously provides scratch space AND enforces the size bound. If encoding doesn't fit → fail (correct behavior).

| Concern | How pool solves it |
|---------|-------------------|
| RFC length enforcement | Buffer size = max message |
| Zero per-call allocation | Pool recycles |
| Single allocation site | Pool New func |
| Capacity checking | WriteTo writes into bounded space |

## Two-Phase Encoding

When encoding requires parsing before writing:
1. Parse phase — validate, compute sizes, return parsed values + total (NO allocation)
2. Write phase — `writeFoo(buf, off, parsed) int` into pool buffer

## Existing Infrastructure

| Tool | Location |
|------|----------|
| `wire.BufWriter` | `internal/plugins/bgp/wire/writer.go` |
| `wire.CheckedBufWriter` | Same |
| `wire.SessionBuffer` | Same |
| `attribute.WriteHeaderTo()` | `internal/plugins/bgp/attribute/attribute.go` |
| `attribute.WriteAttributeTo()` | Same |
| `nlri.WriteTo()` | Various NLRI types |
