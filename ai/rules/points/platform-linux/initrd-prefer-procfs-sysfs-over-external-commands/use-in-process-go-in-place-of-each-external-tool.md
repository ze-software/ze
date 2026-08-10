---
kind: directive
level: MUST
stage:
---
- **Bring a link up / apply address + route**: you MUST use `netlink`, not `ip`.
- **DHCP lease**: you MUST use in-process `nclient4` (`internal/install/disk/dhcp_linux.go`), not `udhcpc` plus a lease script.
- **HTTP image/database download**: you MUST use `net/http` (`internal/install/disk/download.go`), not `wget`.
- **mount / umount / loop / mknod / reboot / poweroff**: you MUST use `golang.org/x/sys/unix` syscalls and ioctls isolated in named `_linux.go` helpers (`mount_linux.go`, `loop_linux.go`, `blockdev_linux.go`), not `mount`/`losetup`/`reboot`.
