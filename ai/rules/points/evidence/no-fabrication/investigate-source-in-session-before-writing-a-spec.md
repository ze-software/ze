---
kind: directive
level: MUST
stage:
---
**You MUST read the implementation source this session before you write a spec or a design.** `writeDesignEvidence` (`internal/le/hookruntime/writeedit.go`) requires current source records for the design's existing producer subjects, bound to path, content digest and harness context. An unrelated read, stale digest or generic LSP marker proves nothing about the subject. The hook is a backstop you MUST NOT treat as evidence that the source was understood.
