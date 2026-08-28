---
kind: directive
level: MUST
stage:
---
- **A Go symbol question MUST use the LSP tool or `gopls` before a whole-file read.**
- Where no symbol server is available, use a narrow search followed by a ranged read.
