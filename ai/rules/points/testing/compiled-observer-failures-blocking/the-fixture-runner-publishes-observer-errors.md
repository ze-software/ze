---
kind: directive
level: MUST
stage:
---
**A failing observer MUST return an error.** `fixture.Run` passes it to
`fixture.ReportFailure`, which emits the `ZE-OBSERVER-FAIL` sentinel that
`checkObserverSentinel` in `internal/test/runner/runner_validate.go` detects.
