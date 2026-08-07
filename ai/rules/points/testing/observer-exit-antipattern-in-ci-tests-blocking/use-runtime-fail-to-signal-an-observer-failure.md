---
kind: directive
level:
stage:
---
**Use `runtime_fail` instead.** `test/scripts/ze_api.py` provides
`runtime_fail(message)` which emits the `ZE-OBSERVER-FAIL` sentinel that the
runner detects via `validateLogging` (`internal/test/runner/runner_validate.go`).
