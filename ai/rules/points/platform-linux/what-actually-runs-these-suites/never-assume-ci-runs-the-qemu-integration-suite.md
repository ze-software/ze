---
kind: directive
level:
stage:
---
**`ze-qemu-integration-test` is still NOT automated:** it additionally needs nested virt / KVM, which GitHub-hosted runners do not reliably provide. It remains enforced by review and by this rule ALONE. Do not assume CI catches a broken QEMU test for you; wiring it up needs a self-hosted or KVM-capable runner.
