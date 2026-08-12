---
kind: directive
level: MUST
stage:
---
**Every `ze-qemu-*-test` and `ze-*-interop-test` target MUST have a caller that runs on its own** -- a workflow job, a script, or another make target. A `.PHONY` line, a `make help` entry and a paragraph in `docs/` are mentions, not callers. Seven of the ten QEMU targets sat with no caller at all until 2026-08-12: `ze-qemu-ldp-frr-test` drove a real FRR ldpd peer and ran nowhere, and `internal/plugins/ldp` was EXCLUDED from `ZE_QEMU_INTEGRATION_PKGS` in its favor, so the package stopped compiling under the `integration` tag and no gate could see it. `TestQemuAndInteropTargetsHaveACaller` (`scripts/dev/github_workflows_test.go`) derives the targets from `mk/*.mk` and the callers from actual invocation. **A target that is deliberately manual MUST be listed in that test's `manualQemuTargets` with the reason no pipeline runs it**; "expensive" describes every target in the class and is not a reason.
