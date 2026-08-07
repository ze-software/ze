---
kind: note
level:
stage:
---
Use `Slice()` when the result is:
- Returned from a function (Buffer is heap-allocated; GC keeps it alive)
- Passed directly to a function call (consumed immediately)
- Used as a map key lookup (not insertion)
- Compared with `==` or passed to `strings.HasPrefix`
- The last extraction before the buffer goes out of scope
