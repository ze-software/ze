---
kind: table
level:
stage:
---
| Method | Use |
|--------|-----|
| `Str(s)` | Append string literal or variable |
| `Byte(c)` | Append separator or single char |
| `Uint(v uint64)` | Append decimal uint64 |
| `Uint8(v)`, `Uint16(v)`, `Uint32(v)` | Typed variants (no cast at call site) |
| `Int(v int64)` | Append decimal int64 |
| `Float(v, prec)` | Append float with N decimal places |
| `Float2(v)` | Append float with 2 decimal places |
| `Bool(v)` | Append "true" or "false" |
| `Addr(a netip.Addr)` | Append IP address |
| `Prefix(p netip.Prefix)` | Append CIDR prefix (e.g. "10.0.0.0/24") |
| `Hex(data []byte)` | Append lowercase hex |
| `HexUpper(data)` | Append uppercase hex |
| `MAC(mac []byte)` | Append MAC address (e.g. "de:ad:be:ef:ca:fe") |
| `Quoted(s)` | Append Go-quoted string with escapes (wraps in `"..."`) |
| `Err(err)` | Append error string (nil-safe, no-op on nil) |
| `Join(items, sep)` | Append strings joined by separator |
| `PadRight(s, width)` | Append `s` then spaces to fill `width` (rune-aware) |
| `PadLeft(s, width)` | Prepend spaces then `s` to fill `width` (rune-aware) |
| `Repeat(s, n)` | Append `s` N times (indentation, padding) |
| `Grow(n)` | Pre-grow capacity to avoid mid-chain reallocation |
| `String()` | Return built string (single alloc for inline, zero-copy for heap). Does NOT freeze: writes continue safely |
| `Slice()` | Return string **zero-copy at any size**. Freezes buffer: writes panic until `Reset()` |
| `Bytes()` | Return raw `[]byte` (shares buffer memory). For `w.Write()` or `string()` in map/switch (compiler elides alloc) |
| `Reset()` | Clear the buffer for reuse. Resets to inline array. Chainable |
| `Write(p)` | Append raw bytes (implements `io.Writer`) |
| `WriteString(s)` | Append string (implements `io.StringWriter`). Returns `(int, error)` |
| `WriteByte(c)` | Append byte (implements `io.ByteWriter`). Returns `error` |
| `WriteRune(r)` | Append rune. Returns `(int, error)` |
| `Len()` | Current content length |
