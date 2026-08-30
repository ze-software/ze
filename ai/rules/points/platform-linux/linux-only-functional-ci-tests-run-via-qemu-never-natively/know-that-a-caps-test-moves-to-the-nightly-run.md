---
kind: directive
level: MUST
stage:
---
**A `caps=` marker RELOCATES coverage; it MUST NOT delete it.** `./le verify worktree` runs unprivileged, so a `caps=net-admin` test does not run in the merge gate. Its home is the scheduled QEMU nightly, and `TestCapabilityGatedTestsHaveANativeVMHome` (`internal/le/workflowcheck/workflowcheck_test.go`) fails when that link is broken: a capability nobody's CI has would be a coverage deletion wearing a skip's clothing (`ai/rules/completion.md`). The nightly reports rather than blocks, so you MUST run the QEMU target locally when you add such a test, and MUST say so. The workflow map is `docs/architecture/testing/ci-workflows.md`.
