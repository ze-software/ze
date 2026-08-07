---
kind: note
level:
stage:
---
`Buffer` is a chainable builder with a 128-byte inline backing array.
`Reset()` uses `noescape` (same technique as `strings.Builder` via
`abi.NoEscape`) to break the self-referential slice from escape analysis.
`var b Buffer` stays on the stack for local use; the inline array avoids
any heap allocation for content <= 128B.
