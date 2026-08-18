---
kind: table
level:
stage:
---
| Do | Don't |
|----|-------|
| Let the second invocation block | Kill the running verify |
| If the run is yours (same tree), read `tmp/ze-verify.log` instead of re-running | Delete the lockfile |
| If "waiting for lock" appears, do other work | Start `go test` / `golangci-lint` / `bin/ze-test` in parallel (bypasses lock) |
