---
kind: directive
level: MUST
stage:
---
- Root access (all capabilities)
- `/dev/ptmx` for PTY pairs
- Network namespaces (`ip netns`)
- Kernel modules: NONE. Every QEMU target boots ze's runtime kernel, and a `--kernel` run pairs it with Alpine's initramfs and Alpine's `/lib/modules`, which are built for 6.12.13-0-virt, so no module loads at all: not Alpine's, and not ze's own. Every symbol any QEMU run needs MUST therefore be `=y` in `gokrazy/kernel/*.config`, and the matching `gokrazy/kernel/*.require` manifest is what makes a silent demotion to `=m` fail the build instead of the test. The Alpine modules a stock boot used to supply (nftables, l2tp, ppp) are now config symbols like every other
- Go toolchain (downloaded and cached in `tmp/qemu/`)
- Repo mounted read-write via virtio-9p at `/workspace`
- No systemd, no getty, no desktop (Alpine minimal)
