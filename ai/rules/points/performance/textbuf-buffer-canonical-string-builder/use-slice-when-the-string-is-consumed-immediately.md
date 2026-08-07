---
kind: directive
level:
stage:
---
**Prefer `Slice()` when the string is consumed immediately** (passed to a
function, used as a map lookup, parsed, or appended into another buffer).
`Slice()` does zero allocations at any size. `String()` copies inline data
(<=128B) and does zero-copy for heap data (>128B).
