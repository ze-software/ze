---
kind: table
level:
stage:
---
| Pattern | Replacement |
|---------|-------------|
| Storing `string` then parsing back for comparison | Store `netip.Addr` (or typed value), compare directly |
| `net.ParseIP(s)` in a comparison function | Store `netip.Addr` at construction, use `.Compare()` |
| `parts[i] = prefix + strconv.Itoa(n)` in a loop | Single Buffer: `b.Str(prefix).Uint(uint64(n))` |
| `fmt.Fprintf(b, "%-*s...", width, s, ...)` | `b.PadRight(s, width).Str(...)` |
