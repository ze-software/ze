---
kind: table
level:
stage:
---
| Surface | Why |
|---------|-----|
| Log / diagnostic output | Humans read; `String()` at emit |
| YANG leaf values | Parser converts on load |
| CLI tokens | Parser converts on dispatch |
| JSON wire format | `MarshalText`/`UnmarshalText` on typed value; wire string, Go typed |
| Config file tokens | Parser converts on load |
| Error messages | Human-readable |
