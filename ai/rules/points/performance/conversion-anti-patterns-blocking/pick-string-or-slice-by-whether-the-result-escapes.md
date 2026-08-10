---
kind: directive
level: MUST
stage:
---
**Mechanical rule for bulk conversions:** when the result leaves the current
scope (return, struct field, map insert, channel send), `String()` with
`var b Buffer` MUST be used, or `Slice()` with `New()` MUST be used.
Slice-from-stack MUST be consumed before the buffer goes out of scope
(function arg, map lookup, comparison).
