---
kind: directive
level: MUST
stage:
---
**Know what you are trading.** A `caps=net-admin` test does NOT run in the merge
gate: `make ze-verify` runs unprivileged, so the marker turns an opaque hang into
an honest skip there. Its home is `.github/workflows/qemu-nightly.yml`, which
runs `ze-qemu-needs-linux-test` on a schedule, so the marker RELOCATES the
coverage rather than deleting it. `TestCapabilityGatedTestsHaveAQemuHome`
(`scripts/dev/github_workflows_test.go`) fails if that link is ever broken:
marking tests with a capability nobody's CI has would be a coverage deletion
wearing a skip's clothing (`ai/rules/completion.md`). The nightly is advisory and
MAY run under TCG emulation, so it is slower than a merge gate and reports rather
than blocks; you MUST run the QEMU target locally when you add a test, and say so.
