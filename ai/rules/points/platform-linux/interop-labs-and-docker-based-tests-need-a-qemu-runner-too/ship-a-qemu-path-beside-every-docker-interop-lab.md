---
kind: directive
level: MUST
stage:
---
**Every Linux-only interop lab that runs as Docker containers and depends on host-kernel features MUST also ship a QEMU-runnable path.** Treat "it is Linux-only, it needs the host kernel" as the trigger to build the QEMU runner, never as a reason to skip it: Docker Desktop's VM lacks the kernel modules and the Alpine QEMU VM has no Docker, so a Docker-only lab is unrunnable on the dev machine. The paired actions each lab ships are `docs/architecture/testing/qemu-integration.md`.
