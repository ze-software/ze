---
kind: directive
level: MUST
stage:
---
- **The caller MUST own the buffer. The callee MUST write into `buf[off:]` and return the number of bytes written. Allocations MUST NOT occur.**
