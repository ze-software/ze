---
kind: directive
level:
stage:
---
1. Buffer from? → Pool or caller-provided
2. `append()`? → Offset writes
3. Returning `[]byte` from helper? → `writeFoo(buf, off) int`
4. `make([]byte)`? → Get from pool
5. Type has `WriteTo`? → Use it
