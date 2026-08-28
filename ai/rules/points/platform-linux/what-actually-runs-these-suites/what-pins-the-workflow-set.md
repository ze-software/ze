---
kind: note
level:
stage:
---
`internal/le/workflowcheck/workflowcheck_test.go` pins the workflow set: that the nightly is
scheduled-only, runs fuzz and integration by native action name, is advisory,
does not smuggle in the QEMU action, that `verify.yml` stays a fast
push/pull_request gate, that every `./le` action named by a workflow is
registered, and that no `.woodpecker` pipeline remains.
