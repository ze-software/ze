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
| `ze-qemu-needs-linux-test` (Linux-only `.ci` functional surface) | `.github/workflows/qemu-nightly.yml`, scheduled | advisory |
| `ze-qemu-integration-test` (Go `integration && linux` packages) | NOTHING automated | -- |
