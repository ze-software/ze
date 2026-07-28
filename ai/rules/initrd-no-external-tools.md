# Initrd: Prefer Procfs/Sysfs Over External Commands

**When:** modifying the installer initrd (`cmd/ze-installer`, `internal/install/disk/*_linux.go`)
**Severity:** blocking

## Directives

Read before modifying the installer initrd (`cmd/ze-installer`,
`internal/install/disk/*_linux.go`).

The installer initrd is a single statically-linked Go binary (`cmd/ze-installer`)
running as PID 1 with **zero external binaries** (busybox removed). Detect system
state through `/proc` and `/sys` reads, not external commands, and never
reintroduce `exec.Command` of an external tool.

## Decision table

| Need | Use | Do NOT use |
|------|-----|------------|
| Check for IPv4 connectivity | `/proc/net/route` (default route = `00000000`) | `ip addr`, `ifconfig` |
| Check NIC carrier/link | `/sys/class/net/*/carrier` | `ip link show`, `ethtool` |
| Check interface flags | `/sys/class/net/*/flags` | `ip link show` |
| List interfaces | `/sys/class/net/` glob | `ip link`, `ls /sys/class/net` |
| Read NIC operstate | `/sys/class/net/*/operstate` | `ip link show` |
| Bring interface up, set address/route | `netlink` (`internal/plugins/iface/netlink`) | `ip link set`, `ip addr`, `ip route` |

## In-process replacements (no external client)

The operations the old shell init shelled out to are now in-process Go:

- **Bring a link up / apply address + route**: `netlink`, not `ip`.
- **DHCP lease**: in-process `nclient4` (`internal/install/disk/dhcp_linux.go`),
  not `udhcpc` plus a lease script.
- **HTTP image/database download**: `net/http` (`internal/install/disk/download.go`),
  not `wget`.
- **mount / umount / loop / mknod / reboot / poweroff**: `golang.org/x/sys/unix`
  syscalls and ioctls isolated in named `_linux.go` helpers (`mount_linux.go`,
  `loop_linux.go`, `blockdev_linux.go`), not `mount`/`losetup`/`reboot`.

Where a syscall is unavoidable, isolate it in a named `_linux.go` helper so the
platform dependency is visible and testable behind a fake. `internal/install/disk`
and `cmd/ze-installer` must contain no `exec.Command` of an external binary; a
QEMU install (`make ze-install-qemu-test`) proves it boots and installs cleanly.
