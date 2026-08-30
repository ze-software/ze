---
kind: directive
level: MUST
stage:
---
**Every registered `./le qemu` and `./le integration` action MUST be given a real caller in the same change**: a workflow job, another native action, or an explicit manual classification. No gate checks this direction today. `TestEveryWorkflowNativeActionExists` (`internal/le/workflowcheck/workflowcheck_test.go`) checks only the other one, that every action a workflow names is registered, so an action nobody calls stays green and runs nowhere. Which workflow job runs each action is `docs/architecture/testing/ci-workflows.md`.
