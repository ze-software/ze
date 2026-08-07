---
kind: fence
level:
stage:
---
```
ai/rules/points/buffer-first/manifest.md

    ---
    title: Buffer-First Encoding
    when: touching wire encoding or allocating memory
    severity: blocking
    related: performance
    ---
    directives ## Directives
      all-wire-encoding-must-write-into-pooled-bounded-buffers

ai/rules/points/buffer-first/directives/all-wire-encoding-must-write-into-pooled-bounded-buffers.md

    ---
    kind: directive
    level: MUST
    stage:
    ---
    - All wire encoding MUST write into pooled, bounded buffers.
```
