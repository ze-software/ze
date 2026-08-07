---
kind: table
level:
stage:
---
| Anti-pattern | Fix |
|-------------|-----|
| `a + ":" + b` (any `+` concatenation) | `var b textbuf.Buffer; b.Str(a).Byte(':').Str(b).String()` |
| `fmt.Sprintf("%d:%d", a, b)` | `b.Uint32(a).Byte(':').Uint32(b).String()` |
| `strconv.Itoa(n) + ":" + strconv.Itoa(m)` | Same |
| `strings.Join(items, sep)` | `textbuf.Join(items, sep)` |
| `var buf [N]byte; b := append(buf[:0]...); return string(b)` | `var b textbuf.Buffer; return b.Str(...).String()` |
| `addr.String() + "/" + strconv.Itoa(n)` | `b.Addr(addr).Byte('/').Int(n).String()` |
| `uint64(v)` cast at call site | Use typed method: `Uint16(v)`, `Uint32(v)` |
