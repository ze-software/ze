---
kind: directive
level: MUST
stage:
---
**You MUST read the implementation source this session before you write a spec or a design.** `writeDesignEvidence` (`internal/le/hookruntime/writeedit.go`) refuses the write when the session recorded no source read, and it is a BACKSTOP you MUST NOT treat as evidence: it accepts ANY recorded read rather than the spec's own subject.
