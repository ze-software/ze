---
kind: directive
level:
stage:
---
**sync.Pool sizing:** Seed with the common-case size. The pool holds
same-max-size buffers. If a caller needs more, `append()` will grow the
slice and the grown slice returns to the pool for future reuse.
