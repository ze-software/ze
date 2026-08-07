---
kind: table
level:
stage:
---
| Target | What it runs | When required |
|--------|-------------|---------------|
| `make ze-qemu-integration-test` | iface, config/system, fib/kernel, firewall/nft, firewall/vpp, traffic/netlink in QEMU Alpine VM | Any change to `//go:build linux` code |
