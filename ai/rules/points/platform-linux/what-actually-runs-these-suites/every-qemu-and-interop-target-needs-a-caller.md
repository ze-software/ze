---
kind: directive
level: MUST
stage:
---
**Every `ze-qemu-*-test` and `ze-*-interop-test` target MUST have a caller that runs on its own**: a workflow job, a script, or another make target. A `.PHONY` line, a `make help` entry, and a paragraph in `docs/` are mentions, not callers. `TestQemuAndInteropTargetsHaveACaller` (`scripts/dev/github_workflows_test.go`) derives the targets from `mk/*.mk` and the callers from actual invocation. **A target that is deliberately manual MUST be listed in that test's `manualQemuTargets` with the reason no pipeline runs it**. "Expensive" describes every target in the class and is not a reason.
