---
kind: note
level: MUST
stage:
---
A Linux-only interop lab that runs as **Docker containers and depends on
host-kernel features** (L2TP, PPPoE, netfilter, ...) does NOT run on macOS or in
CI by itself: Docker Desktop's VM lacks the kernel modules, and the Alpine QEMU
VM has no Docker. Shipping only the Docker lab leaves the test unrunnable on the
dev machine. Every such lab MUST also ship a QEMU-runnable path; treat "it's
Linux-only / needs the host kernel" as the trigger to build the QEMU runner, not
as an excuse to skip it.
