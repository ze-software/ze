---
kind: directive
level:
stage:
---
1. **Where is the buffer allocated?** Name the function and pool.
2. **Who holds it?** Which goroutine/struct owns the reference.
3. **When is it copied?** Only at the boundaries listed in "When Copies Happen."
4. **When is it released?** After TCP write, after pool dedup, after use.
5. **Could the caller provide this buffer?** If yes, change the signature.
6. **Could a pool provide this buffer?** If yes, Get/Put around the use.
