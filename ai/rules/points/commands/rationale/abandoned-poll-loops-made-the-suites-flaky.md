---
kind: note
level:
stage:
---
An abandoned poll loop keeps taking CPU after its answer is no longer needed.
That contention can make concurrent QEMU, Docker, and verification work fail.
