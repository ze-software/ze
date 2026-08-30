---
kind: directive
level: MUST
stage:
---
**A buffer's lifecycle MUST be traced before writing code:**

1. **Where is the buffer allocated?** The function and the pool MUST be named.
2. **Who holds it?** The goroutine or struct owning the reference MUST be named.
3. **When is it copied?** Only at a deliberate copy trigger (`docs/architecture/buffer-architecture.md`).
4. **When is it released?** After the TCP write, after pool dedup, or after use.
5. **Could the caller provide this buffer?** If yes, the signature MUST change.
6. **Could a pool provide this buffer?** If yes, Get and Put MUST wrap the use.
