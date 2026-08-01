# 813: ze installer initrd (build artifact)

## Context

spec-install-6: minimal Linux initrd for PXE-based bare-metal provisioning. The initrd is the final step in the PXE chain: bootloader fetches kernel+initrd via HTTP, kernel boots, init script downloads gokrazy disk image, writes to disk, injects zefs database, reboots.

## Decisions

- **Busybox-based, not u-root.** Shell script init with busybox provides wget, dd, mount, blockdev without compiling Go. Simpler build, smaller image, easier to debug (drop-to-shell on error). u-root would add Go compilation dependency to the initrd build.
- **Stream image directly to disk.** `wget -O - | dd` avoids needing disk space for a temp copy. The initrd runs from RAM with no writable persistent storage.
- **Kernel-level DHCP, not userspace.** `ip=dhcp` on the kernel cmdline handles network configuration. No DHCP client needed in the initrd, keeping it minimal.
- **Sysfs removable attribute for disk detection.** `/sys/block/*/removable` is the standard Linux mechanism. Virtual devices (loop, ram, dm, zram) filtered by name pattern. First non-removable device selected.
- **Partition naming handles NVMe/eMMC.** NVMe and eMMC use `p4` suffix (`/dev/nvme0n1p4`, `/dev/mmcblk0p4`), SATA/SCSI use bare number (`/dev/sda4`).

## Consequences

- Build requires busybox-static. On systems without it, the BUSYBOX= Makefile variable must point to a statically-linked busybox.
- QEMU integration test is a placeholder. Full PXE chain testing requires QEMU with PXE ROM and network bridging, which is dedicated CI infrastructure.
- No image integrity verification in v1. SHA256 checksum support is a v2 feature.

## Gotchas

- `blockdev --rereadpt` is needed after writing the disk image. The kernel may not detect new partitions from a full-disk write without it.
- `reboot -f` may not work in all initrd environments. Fallback to `echo b > /proc/sysrq-trigger` covers the edge case.
- The init script must mount proc, sys, and devtmpfs before accessing /proc/cmdline or /sys/block or /dev.

## Files

None recorded.
