---
kind: table
level:
stage:
---
| Need | Use | Do NOT use |
|------|-----|------------|
| Check for IPv4 connectivity | `/proc/net/route` (default route = `00000000`) | `ip addr`, `ifconfig` |
| Check NIC carrier/link | `/sys/class/net/*/carrier` | `ip link show`, `ethtool` |
| Check interface flags | `/sys/class/net/*/flags` | `ip link show` |
| List interfaces | `/sys/class/net/` glob | `ip link`, `ls /sys/class/net` |
| Read NIC operstate | `/sys/class/net/*/operstate` | `ip link show` |
| Bring interface up, set address/route | `netlink` (`internal/plugins/iface/netlink`) | `ip link set`, `ip addr`, `ip route` |
