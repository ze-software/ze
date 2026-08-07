---
kind: note
level:
stage:
---
This avoids the `Len()`-then-`WriteTo()` double traversal. See `reactor_wire.go` for the canonical implementation.
