---
kind: directive
level: MAY
stage:
---
**Test code and the build harness are outside this rule.** A `_test.go` anywhere, `test/`, `internal/test/`, `Makefile`, `mk/` and `scripts/dev/` all drive a developer machine or a CI runner, where the toolchain is present by construction and calling it is often the point: a test MAY run whatever it needs to set up or observe. What is governed is what Ze SHIPS and runs on an appliance: non-test code under `cmd/`, `internal/`, `pkg/` and `scripts/evidence/`, in every build and on every platform. A diagnostic Ze ships is Ze, and it runs where no toolchain exists.
