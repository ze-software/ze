---
kind: directive
level: MUST NOT
stage:
---
**An integration test MUST NOT require physical hardware.** The QEMU VM provides the kernel features, so a virtual device stands in for the hardware. The substitute for each device is `docs/architecture/testing/qemu-integration.md`.
