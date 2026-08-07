---
kind: table
level:
stage:
---
| Pattern | Replacement |
|---------|-------------|
| `strings.Join(items, ", ")` | `textbuf.Join(items, ", ")` |
| `strings.Builder` + loop | `textbuf.Buffer` + loop with `Reset()` |
| `strings.Repeat(s, n)` | `var b textbuf.Buffer; b.Repeat(s, n).String()` |
| `b.WriteString(strings.Repeat("\t", indent))` | `b.Repeat("\t", indent)` |
