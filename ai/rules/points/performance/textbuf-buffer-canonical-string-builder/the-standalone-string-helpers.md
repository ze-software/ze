---
kind: table
level:
stage:
---
| Function | Returns |
|----------|---------|
| `textbuf.StringUint(v)`, `textbuf.StringUint8(v)`, `textbuf.StringUint16(v)`, `textbuf.StringUint32(v)` | Decimal string |
| `textbuf.StringInt(v)` | Signed decimal string |
| `textbuf.StringAddr(a)` | IP address string |
| `textbuf.StringPrefix(p)` | CIDR prefix string |
| `textbuf.StringHex(data)`, `textbuf.StringHexUpper(data)` | Hex-encoded string |
| `textbuf.StringMAC(mac)` | MAC address string (e.g. "de:ad:be:ef:ca:fe") |
| `textbuf.HostPort(host, port)` | "host:port" string |
| `textbuf.Join(items, sep)` | Joined string (replaces `strings.Join`) |
| `textbuf.StrInt(prefix, v)` | "prefix" + decimal |
| `textbuf.StrUint(prefix, v)` | "prefix" + unsigned decimal |
| `textbuf.IntStr(v, suffix)` | Decimal + "suffix" |
| `textbuf.UintStr(v, suffix)` | Unsigned decimal + "suffix" |
| `textbuf.StrIntStr(prefix, v, suffix)` | "prefix" + decimal + "suffix" |
| `textbuf.StrUintStr(prefix, v, suffix)` | "prefix" + unsigned decimal + "suffix" |
