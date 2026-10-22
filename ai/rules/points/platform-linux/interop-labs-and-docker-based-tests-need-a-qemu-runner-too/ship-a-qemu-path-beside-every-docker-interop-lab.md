---
kind: directive
level: MUST
stage:
---
**Every Linux-only interop lab that runs as Docker containers and depends on host-kernel features MUST also ship a QEMU-runnable path.** Treat "it is Linux-only, it needs the host kernel" as the trigger to build the QEMU runner, never as a reason to skip it: the Linux VM of Docker Desktop or colima lacks the kernel features, so a Docker-only lab is unrunnable on the dev machine. `./le test qemu docker-lab` runs a Docker lab inside a QEMU guest that boots Ze's docker-host image (Ze's kernel and system, with dockerd shipped in the image and no Alpine); the paired actions each lab ships are `docs/architecture/testing/qemu-integration.md`.
