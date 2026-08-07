---
kind: note
level:
stage:
---
If the command has a rendering path that does NOT call `ApplyPipes`/`formatFn`,
verify that `| resolve` and `| origin` are still applied in that path.
