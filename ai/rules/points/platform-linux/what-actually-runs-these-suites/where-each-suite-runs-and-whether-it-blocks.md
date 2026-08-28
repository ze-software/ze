---
kind: table
level:
stage:
---
| Suite | Where it runs | Blocking? |
|-------|---------------|-----------|
| `./le verify worktree` (unit + functional + static gates) | `.github/workflows/verify.yml`, push + pull_request | yes |
| `./le fuzz` | `.github/workflows/evidence-nightly.yml`, scheduled | advisory |
| `./le integration` actions | `.github/workflows/evidence-nightly.yml`, scheduled; root where required | advisory |
| `./le qemu run ... command '<native action>'` for the Linux-only functional surface | `.github/workflows/qemu-nightly.yml`, job `needs-linux`, scheduled | advisory |
| `./le qemu` routing-protocol actions inside `./le qemu run` | `.github/workflows/qemu-nightly.yml`, job `protocol-labs`, scheduled | advisory |
| `./le qemu` access-protocol actions inside `./le qemu run` | `.github/workflows/qemu-nightly.yml`, job `runtime-kernel-labs`, scheduled | advisory |
| `./le integration interop`, `./le integration interop-ipsec` | `.github/workflows/evidence-nightly.yml`, scheduled | advisory |
| `./le qemu all-tests` inside the runtime-kernel guest | `.github/workflows/qemu-nightly.yml`, job `needs-linux`, scheduled | advisory |
