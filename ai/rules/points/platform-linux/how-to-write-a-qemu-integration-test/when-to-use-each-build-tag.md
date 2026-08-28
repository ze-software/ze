---
kind: table
level:
stage:
---
| Tag | When to use | Runs during |
|-----|------------|-------------|
| `//go:build linux` | Test only needs Linux types (imports linux-only packages) but no kernel capabilities | `go test` on any Linux host, including QEMU |
| `//go:build integration && linux` | Test needs kernel capabilities (root, /dev, netns, ioctl) | `./le qemu all-tests` (passes `-tags integration`) |
