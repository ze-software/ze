---
kind: directive
level: MUST NOT
stage:
---
**`ze-qemu-integration-test` is NOT automated:** its only caller is `make ze-release-evidence-verify` (`mk/test-release.mk`), which a person runs before a release. Its own cost keeps it out. You MUST NOT assume CI catches a broken Go `integration && linux` package for you.
