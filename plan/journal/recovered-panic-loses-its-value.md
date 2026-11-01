# A recovered panic loses its value and stack

A recover turns a panic into an error so the process keeps running, and the
error it builds names only that a panic happened. The panic value and the stack
are dropped at the recover, so the report that reaches the operator or a test
harness cannot say what failed or where, and the defect cannot be debugged from
it. The process-wide boundary (`taskguard.Task.Contain`) keeps both and writes a
crash record; an ad-hoc recover beside it does not.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-01 | spec-plugin-smoke-run | SDK bridge callbacks (`bridgeCallbackPanicError`, `pkg/plugin/sdk/sdk_dispatch.go`) | `handleBridgeCallback` and `handleBridgeExecuteCommand` recover a panic and call `bridgeCallbackPanicError(r)`, which ignores its argument and returns only "bridge callback panic" wrapped on `rpc.ErrBridgeFailed`. The value and the stack are lost and no log line is written, so a crash caught there cannot be debugged from its report. | open: the error carries the panic value, the stack is logged, and a smoke case proves both reach the report |
