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
| Any new linux-only package | Package added to `ze-qemu-integration-test` Makefile target |
| Docker interop lab needing host-kernel features (l2tp, pppoe, ...) | A netns `effective-<feature>.py` + `ze-qemu-<feature>-test` target -- the Docker lab cannot run in the Alpine VM (see "Interop Labs" below) |
