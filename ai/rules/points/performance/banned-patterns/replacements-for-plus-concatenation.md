---
kind: table
level:
stage:
---
| `+` pattern | Replacement |
|-------------|-------------|
| `a + "/" + b` | `var tb textbuf.Buffer; tb.Str(a).Byte('/').Str(b).String()` |
| `"prefix:" + s` | `var b textbuf.Buffer; b.Str("prefix:").Str(s).String()` |
| `s + strconv.Itoa(n)` | `var b textbuf.Buffer; b.Str(s).Int(int64(n)).String()` |
| `addr.String() + "/" + strconv.Itoa(n)` | `var b textbuf.Buffer; b.Addr(addr).Byte('/').Int(int64(n)).String()` |
| `">" + textbuf.StringUint(v)` | `var b textbuf.Buffer; b.Byte('>').Uint(v).String()` |
| `strings.Join(items, sep)` | `textbuf.Join(items, sep)` |
