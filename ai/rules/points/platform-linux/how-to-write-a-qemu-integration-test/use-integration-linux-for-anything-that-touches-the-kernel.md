---
kind: note
level:
stage:
---
Use `integration && linux` for anything that touches the kernel. Use bare
`linux` only when the test imports linux-only types but makes no syscalls.
