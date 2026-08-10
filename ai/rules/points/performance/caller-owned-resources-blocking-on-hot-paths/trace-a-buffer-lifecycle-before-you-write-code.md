---
kind: directive
level: MUST
stage:
---
**A buffer's lifecycle MUST be traced before writing code:**

1. **Where is the buffer allocated?** The function and pool MUST be named.
2. **Who holds it?** Which goroutine/struct owns the reference.
3. **When is it copied?** Only at the boundaries listed in "When Copies Happen."
4. **When is it released?** After TCP write, after pool dedup, after use.
5. **Could the caller provide this buffer?** If yes, the signature MUST change.
6. **Could a pool provide this buffer?** If yes, Get/Put MUST wrap the use.
