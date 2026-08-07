---
kind: table
level:
stage:
---
| Pattern | Use |
|---------|-----|
| Multi-part string, stored | `var b textbuf.Buffer; return b.Str(...).Uint32(...).String()` |
| Multi-part string, consumed immediately | `var b textbuf.Buffer; parse(b.Str(...).Uint32(...).Slice())` |
| Reuse in a loop | `var b textbuf.Buffer; for ... { use(b.Reset().Str(...).Slice()) }` |
| Single value | `return textbuf.StringUint32(v)` or `return textbuf.StringAddr(a)` |
| Append into `[]byte` | `textbuf.Uint(dst, v)`, `textbuf.Addr(dst, a)` |
