---
kind: directive
level:
stage:
---
- **The caller always owns the buffer. The callee writes into `buf[off:]` and returns the number of bytes written. No allocations.**
