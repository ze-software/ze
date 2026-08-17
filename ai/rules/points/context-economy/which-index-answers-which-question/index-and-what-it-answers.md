---
kind: table
level:
stage:
---
| Question | Where the answer is | What comes back |
|----------|---------------------|-----------------|
| What is this file for, and which doc governs it? | the file's own `// Design:` header, in its first 25 lines | the design doc, plus the sibling files that own each detail |
| Which docs cover this code? | `grep` the basename under its package heading in `ai/CODE-TO-DOCS.md` | every doc citing it. Rows are keyed by BASENAME, so the package heading is what stops a bare `grep peer.go` returning three packages |
| Which `.go` files implement this design doc? | `grep` the doc path in `ai/DOCS-TO-CODE.md` | every file whose `// Design:` header cites that doc, one line each |
| What does this package do? | `grep` the package path in `ai/PACKAGE-MAP.md` | one line, derived from the package doc comment |
| How does this subsystem flow, entry to exit? | `ai/digests/<subsystem>.md` | the flow with `file:line`, the load-bearing files, and the invariants |
