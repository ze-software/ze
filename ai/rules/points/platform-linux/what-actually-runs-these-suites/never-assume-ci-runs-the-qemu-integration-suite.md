---
kind: directive
level: MUST NOT
stage:
---
**`ze-qemu-integration-test` is still NOT automated:** its only caller is `make ze-release-evidence` (`mk/test-release.mk`), which a person runs before a release. The QEMU labs in the rows above ARE automated since 2026-08-12, so the old reason for this one -- that hosted runners do not reliably provide nested virt / KVM -- no longer holds: `.github/workflows/qemu-nightly.yml` measured a usable `/dev/kvm` on `ubuntu-latest` (run 30249183064, 2026-07-27) and falls back to TCG when it is absent. What keeps this target out is its own cost. You MUST NOT assume CI catches a broken Go `integration && linux` package for you.
