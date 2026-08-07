---
kind: table
level:
stage:
---
| Hardware | Virtual substitute | Example |
|----------|--------------------|---------|
| Serial port (`/dev/ttyS*`) | PTY pair via `creack/pty` | `master, slave, _ := pty.Open()` then `applyTermios(slave.Name(), 9600)` |
| Network interface | `veth` pair or `dummy` in a netns | `ip link add ze0 type dummy` |
| Firewall table | `nftables` in a netns | `nft add table ip ze_test` |
| Kernel route | Netlink in a netns | `route.Add(...)` |
| Block device | Loop device on tmpfs file | `losetup` |
