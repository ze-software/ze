---
kind: table
level:
stage:
---
| Suite | Where it runs | Blocking? |
|-------|---------------|-----------|
| `make ze-verify` (unit + functional + static gates) | `.github/workflows/verify.yml`, push + pull_request | yes |
| `ze-fuzz-test` | `.github/workflows/evidence-nightly.yml`, scheduled | advisory |
| `ze-integration-test` (non-QEMU kernel suites) | `.github/workflows/evidence-nightly.yml`, scheduled, `sudo` (root) | advisory |
| `ze-qemu-needs-linux-test` (Linux-only `.ci` functional surface) | `.github/workflows/qemu-nightly.yml`, job `needs-linux`, scheduled | advisory |
| `ze-qemu-ldp-frr-test`, `ze-qemu-isis-frr-test`, `ze-qemu-vrrp-keepalived-test` (stock-kernel protocol labs) | `.github/workflows/qemu-nightly.yml`, job `protocol-labs`, scheduled | advisory |
| `ze-qemu-l2tp-ppp-test`, `ze-qemu-pppoe-accel-test`, `ze-qemu-pppoe-test`, `ze-qemu-traffic-usage-test` (runtime-kernel labs) | `.github/workflows/qemu-nightly.yml`, job `runtime-kernel-labs`, scheduled | advisory |
| `ze-interop-test`, `ze-ipsec-interop-test` (Docker interop trees) | `.github/workflows/evidence-nightly.yml`, scheduled | advisory |
| `ze-qemu-integration-test` (Go `integration && linux` packages) | `make ze-release-evidence` only, by hand | -- |
| `ze-qemu-all-test` (full suite in the VM) | nothing; `manualQemuTargets` records why | -- |
