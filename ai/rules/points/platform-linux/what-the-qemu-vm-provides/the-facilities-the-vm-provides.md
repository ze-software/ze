---
kind: directive
level:
stage:
---
- Root access (all capabilities)
- `/dev/ptmx` for PTY pairs
- Network namespaces (`ip netns`)
- Kernel modules: nftables, l2tp, ppp (loaded at boot)
- Go toolchain (downloaded and cached in `tmp/qemu/`)
- Repo mounted read-write via virtio-9p at `/workspace`
- No systemd, no getty, no desktop (Alpine minimal)
