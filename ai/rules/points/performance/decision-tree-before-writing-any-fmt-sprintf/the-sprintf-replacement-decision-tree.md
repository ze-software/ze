---
kind: fence
level:
stage:
---
```
1. Is the format string a constant with no verbs?
   → errors.New (for Errorf), literal string (for Sprintf)

2. Is the only verb %d?
   → textbuf.StringUint(uint64(n)) or textbuf.StringInt(int64(n))

3. Is it a string with separators or mixed types?
   → textbuf.Buffer chain: b.Str(a).Byte(':').Str(b).String()

4. Is it formatting an IP address?
   → textbuf.StringAddr(a) or textbuf.StringPrefix(p)

5. Is it formatting a MAC address?
   → textbuf.StringMAC(mac)

6. Is it host:port?
   → textbuf.HostPort(host, port)

7. Is it hex formatting (%x, %02x, %X)?
   → textbuf.StringHex(data) or textbuf.StringHexUpper(data)
   → textbuf.Hex(buf, data) for hot paths

8. Is it joining strings with a separator?
   → textbuf.Join(items, sep) or b.Join(items, sep)

9. Is it on a hot path (per-UPDATE, per-route, per-NLRI)?
   → AppendTo(buf []byte) []byte pattern (see text_append.go)
   → textbuf.Uint, textbuf.Int, textbuf.Addr, textbuf.Prefix, textbuf.Hex, textbuf.HexUpper, textbuf.MAC
   → Never fmt.Sprintf. No exceptions.
   → Never .String() + concatenation. Use stack buffer.

10. Is it writing to an io.Writer?
    → w.Write([]byte(...)) or io.WriteString(w, s)
    → strconv.AppendInt into a [20]byte scratch, then w.Write

11. None of the above?
    → fmt.Sprintf is acceptable (cold path, complex format)
```
