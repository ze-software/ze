---
kind: note
level:
stage:
---
`internal/core/textbuf/textbuf.go` uses a `noescape` function identical to the technique `strings.Builder` uses via `abi.NoEscape` to prevent self-referential slices from escaping to the heap.
