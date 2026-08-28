---
kind: table
level:
stage:
---
| You wrote | You need |
|-----------|----------|
| `//go:build linux` source file | Corresponding `*_integration_linux_test.go` |
| termios / serial port code | PTY-pair test (`creack/pty`, already vendored) |
| netlink / interface code | Network namespace + veth/dummy test |
| nftables / firewall code | Network namespace + nft test |
| sysctl / kernel tuning | procfs read test (may need `t.Skip` for write) |
| Any new Linux-only package | Add it to `integrationPackages` in `internal/le/qemu/alltests.go` |
| Docker interop lab needing host-kernel features | Add a native `./le qemu <feature>` action beside the Docker action |
