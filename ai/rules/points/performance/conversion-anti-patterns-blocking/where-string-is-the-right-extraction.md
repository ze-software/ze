---
kind: note
level:
stage:
---
Use `String()` when the result is:
- Stored in a struct field or variable that outlives the next `Reset()`
- Inserted as a map key (must own the memory)
- Extracted mid-chain and the buffer is reused afterward
