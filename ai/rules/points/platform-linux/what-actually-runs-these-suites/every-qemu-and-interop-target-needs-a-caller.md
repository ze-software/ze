---
kind: directive
level: MUST
stage:
---
**Every registered `./le qemu` and `./le integration` action MUST have a real caller**: a workflow job, another native action, or an explicit manual classification. `TestQemuAndInteropTargetsHaveACaller` in `internal/le/workflowcheck/workflowcheck_test.go` derives actions and callers from the Go registries and workflows.
