# Initrd: Prefer Procfs/Sysfs Over External Commands

**BLOCKING:** Read before modifying `tools/installer-initrd/init`.

The installer initrd targets gokrazy appliances where busybox may not be
present. Detect system state through `/proc` and `/sys` reads, not external
commands.

## Decision table

| Need | Use | Do NOT use |
|------|-----|------------|
| Check for IPv4 connectivity | `/proc/net/route` (default route = `00000000`) | `ip addr`, `ifconfig` |
| Check NIC carrier/link | `/sys/class/net/*/carrier` | `ip link show`, `ethtool` |
| Check interface flags | `/sys/class/net/*/flags` | `ip link show` |
| List interfaces | `/sys/class/net/` glob | `ip link`, `ls /sys/class/net` |
| Read NIC operstate | `/sys/class/net/*/operstate` | `ip link show` |
| Bring interface up | `link_up()` helper (isolates the ioctl dependency) | Inline `ip link set` calls |

## Exceptions

- **`link_up()`**: bringing an interface up requires an ioctl (SIOCSIFFLAGS);
  no procfs/sysfs write exists. The `link_up()` helper isolates this so
  it can be swapped for a purpose-built binary when busybox is removed.
- **`udhcpc`**: DHCP requires a userspace client. Same isolation applies.
- **`wget`**: HTTP download requires a userspace client.

When an external command is unavoidable, wrap it in a named helper function
so the dependency is visible and replaceable.
