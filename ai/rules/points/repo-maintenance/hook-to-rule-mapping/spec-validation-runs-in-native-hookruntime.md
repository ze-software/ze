---
kind: note
level:
stage:
---
`hookValidateSpec` in `internal/le/hookruntime/lifecycle.go` validates the Wiring
Test table and returns the hook protocol verdict. `runLifecycleHook` dispatches
the `validate-spec` action, and `./le hook-check unit` covers its accepted and
refused forms.
