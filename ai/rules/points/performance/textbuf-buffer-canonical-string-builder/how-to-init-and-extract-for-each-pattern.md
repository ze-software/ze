---
kind: table
level:
stage:
---
| Pattern | Init | Extract | Allocs |
|---------|------|---------|--------|
| Local use, write to io.Writer | `var b Buffer` | `Bytes()` | 0 |
| Local use, map lookup / comparison | `var b Buffer` | `string(b.Bytes())` | 0 |
| Hot loop, string consumed immediately | `Get()` + `defer Release()` | `Slice()` or `Bytes()` | 0 |
| Caller-owned buffer | `AppendTo` functions | none | 0 |
| Return a string (single use) | `var b Buffer` | `String()` | 1 |
| Return a string (zero-copy) | `New()` | `Slice()` | 1 |
| Return a string, reuse buffer | `Get()` + `defer Release()` | `String()` | 1 |
