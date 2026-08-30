---
kind: directive
level: MUST NOT
stage:
---
**While another verify holds the slot, the second invocation MUST be left to
block.**

| Do | MUST NOT |
|----|----------|
| Let the second invocation block | Kill the running verify |
| When the run is yours in the same tree, read `tmp/ze-verify.log` rather than re-running | Delete a job entry to take the slot |
| When the wait message appears, do other work | Start `go test`, `golangci-lint` or a `ze-test` binary in parallel, which bypasses admission |
