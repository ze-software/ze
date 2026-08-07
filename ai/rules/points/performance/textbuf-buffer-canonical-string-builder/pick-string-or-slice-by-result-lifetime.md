---
kind: table
level:
stage:
---
| Result lifetime | Use | Allocations |
|----------------|-----|-------------|
| Returned from function (single use) | `Slice()` | 0 (Buffer on heap; GC traces interior pointer) |
| Stored in a struct field | `String()` | 1 (inline copy) or 0 (heap transfer) |
| Consumed before `Reset()`/`Release()` | `Slice()` | 0 |
| Passed to `netip.ParsePrefix()` etc. | `Slice()` | 0 (parser copies internally if needed) |
| Buffer reused after extraction | `String()` | 1 or 0 (does not freeze buffer) |
