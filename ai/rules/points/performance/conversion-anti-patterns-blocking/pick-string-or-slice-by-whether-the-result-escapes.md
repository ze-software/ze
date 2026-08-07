---
kind: directive
level:
stage:
---
**Mechanical rule for bulk conversions:** when the result leaves the current
scope (return, struct field, map insert, channel send), use `String()` with
`var b Buffer`, or `Slice()` with `New()`. Slice-from-stack is only safe when
consumed before the buffer goes out of scope (function arg, map lookup, comparison).
