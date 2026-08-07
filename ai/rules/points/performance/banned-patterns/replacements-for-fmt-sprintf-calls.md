---
kind: table
level:
stage:
---
| Pattern | Replacement |
|---------|-------------|
| `fmt.Sprintf("%d", n)` | `textbuf.StringInt(int64(n))` (standalone) or `b.Int(int64(n))` (in chain) |
| `fmt.Sprintf("%s", s)` | `s` |
| `fmt.Sprintf("%s:%d", s, n)` | `var b textbuf.Buffer; b.Str(s).Byte(':').Int(int64(n)).String()` |
| `fmt.Sprintf("%d:%d", a, b)` | `var b textbuf.Buffer; b.Int(int64(a)).Byte(':').Int(int64(b)).String()` |
| `fmt.Sprintf("%d.%d.%d.%d", a,b,c,d)` | `netip.AddrFrom4` + `textbuf.StringAddr(addr)` |
| `fmt.Sprintf("%02x:%02x:...", mac...)` | `textbuf.StringMAC(mac)` |
| `fmt.Sprintf("%x", data)` | `textbuf.StringHex(data)` or `textbuf.Hex(buf, data)` |
| `fmt.Sprintf("%X", data)` | `textbuf.StringHexUpper(data)` or `textbuf.HexUpper(buf, data)` |
| `fmt.Sprintf("%q", s)` | `var b textbuf.Buffer; b.Quoted(s).String()` |
| `fmt.Sprintf("%.1f", v)` | `var b textbuf.Buffer; b.Float(v, 1).String()` |
| `fmt.Sprintf("ctx: %v", err)` | `var b textbuf.Buffer; b.Str("ctx: ").Err(err).String()` |
| `fmt.Sprintf("%-20s", s)` | `var b textbuf.Buffer; b.PadRight(s, 20).String()` |
| `fmt.Sprintf("%6s", s)` | `var b textbuf.Buffer; b.PadLeft(s, 6).String()` |
| `fmt.Sprintf("127.0.0.1:%d", port)` | `textbuf.HostPort("127.0.0.1", port)` |
| `fmt.Errorf("constant string")` | `var ErrFoo = errors.New("constant string")` at package level |
| `fmt.Fprintf(w, "%s", s)` | `io.WriteString(w, s)` |
| `fmt.Fprintf(w, "%d", n)` | `io.WriteString(w, textbuf.StringInt(int64(n)))` |
| Sprintf in a function that discards the result | Split into no-alloc + with-string variants |
