---
kind: table
level:
stage:
---
| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `func (t T) Marshal() ([]byte, error)` | `func (t T) WriteTo(buf []byte, off int) int` | `ai/rules/performance.md` | Zero alloc on hot path; caller owns the buffer |
| `bytes.Buffer` / `append` in helpers | Pre-allocated pooled buffers, slice inward | `ai/rules/performance.md` | Bounded memory, no GC pressure |
| `make([]byte, n)` for variable-length wire data | Pool-backed buffers of fixed MAX size | `ai/rules/performance.md` | Pool strategy by goroutine shape |
| Helper allocates its own scratch | Caller passes buffer down, callee writes into it | `ai/rules/performance.md` | One allocation at outermost scope, not N in sub-functions |
| `sync.Pool` only for "reuse" | `sync.Pool` is the default for multi-goroutine scratch, ring buffer for single-goroutine | `ai/rules/performance.md` | Pool shape must match goroutine shape |
| Parse into structs eagerly | Lazy iterators over raw byte slices (`Next()`) | "Design Principles" above (Lazy over eager) | N->0-until-needed, not N->1 |
| `fmt.Sprintf` for formatting | `textbuf.Buffer` (128B stack-inline) or `strconv.Append*` | `ai/rules/performance.md` | Sprintf allocates 2-3x; textbuf allocates once |
| `strings.Join(parts, " ")` | Single `textbuf.Buffer` with `.Byte(' ')` separators | `ai/rules/performance.md` | Eliminates intermediate `[]string` + final join |
