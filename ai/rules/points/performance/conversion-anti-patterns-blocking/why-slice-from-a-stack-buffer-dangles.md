---
kind: note
level:
stage:
---
During bulk `+` → textbuf conversions, it is tempting to use `Slice()` everywhere
for zero-copy. But `var b Buffer` uses `noescape` to stay on the stack, so
`Slice()` returns a string pointing into stack memory. If that string escapes
the function (returned, stored in a struct, sent to a goroutine), it dangles.
