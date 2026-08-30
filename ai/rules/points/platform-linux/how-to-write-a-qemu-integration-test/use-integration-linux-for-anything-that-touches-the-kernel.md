---
kind: directive
level: MUST
stage:
---
**A test that touches the kernel MUST carry `integration && linux`, and bare `linux` MUST be used only when the test imports linux-only types and makes no syscall.** Which tag runs where, and the file-name pattern each one takes, is `docs/architecture/testing/qemu-integration.md`.
