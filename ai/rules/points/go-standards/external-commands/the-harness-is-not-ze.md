---
kind: directive
level: MAY
stage:
---
**Test code and native developer tooling are outside this rule.** A `_test.go` anywhere, `test/`, `internal/test/`, and `internal/le/` drive a developer machine or CI runner, where the toolchain is present by construction and calling it is often the point: a test MAY run what it needs to set up or observe. What is governed is what Ze ships and runs on an appliance: non-test product code under `cmd/`, `internal/`, and `pkg/`. A diagnostic Ze ships is Ze, and it runs where no toolchain exists.
