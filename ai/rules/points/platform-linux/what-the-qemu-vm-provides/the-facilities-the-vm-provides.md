---
kind: directive
level:
stage:
---
- Root access (all capabilities)
- `/dev/ptmx` for PTY pairs
- Network namespaces (`ip netns`)
- Kernel modules: nftables, l2tp, ppp (loaded at boot) -- **only under the stock Alpine kernel.** A `--kernel` run pairs ze's kernel with Alpine's initramfs and Alpine's `/lib/modules`, which are built for 6.12.13-0-virt, so NO module of the ze kernel can load. Every symbol such a run needs must be `=y` in `gokrazy/kernel/*.config`, and `gokrazy/kernel/kernel.require` is what makes a silent demotion to `=m` fail the build instead of the test
- Go toolchain (downloaded and cached in `tmp/qemu/`)
- Repo mounted read-write via virtio-9p at `/workspace`
- No systemd, no getty, no desktop (Alpine minimal)
