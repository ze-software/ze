---
kind: note
level:
stage:
---
A `map[string]V` with 1000 entries stores 1000 string headers the GC must scan on every collection cycle. A `map[uint16]V` stores inline integers the GC ignores entirely.
