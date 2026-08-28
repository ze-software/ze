---
kind: table
level:
stage:
---
| Mistake | Why it's wrong | Fix |
|---|---|---|
| `make([]byte, n)` in a per-UPDATE function | Allocates on every UPDATE | Get from pool or write into caller's buffer |
| `func Encode() []byte` returning allocated bytes | Caller must copy into its buffer | Change to `WriteTo(buf, off) int` |
| `fmt.Sprintf` in reactor/wire/attribute code | 2+ allocations per call | `textbuf.Buffer` or `textbuf.StringUint32()` |
| `addr.String()` in a loop | Allocates per iteration | `addr.AppendTo(buf[:0])` into stack buffer |
| Holding WireUpdate past readBuf return | WireUpdate references readBuf memory | Copy needed data before returning readBuf to pool |
| Building `[]string` + `strings.Join` in a loop | N+1 allocations | Single `textbuf.Buffer` outside the loop |
| `string(bytes)` + comparison in a filter | Allocates the string | Compare bytes directly or use typed value |
| `map[string]V` keyed by value from a known set | String keys cost: hash over bytes, GC scans pointers | `map[uint16]V` or `map[TypedEnum]V`; parse string at boundary (`ai/rules/go-standards.md`) |
| `BufHandle{Buf: make(...)}` | Corrupts pool tracking | Only use pool-issued BufHandles; `writeGoPatterns` in `internal/le/hookruntime/writeedit.go` enforces |
