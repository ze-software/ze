---
kind: table
level:
stage:
---
| Detector | Fires when | Fix |
|----------|-----------|-----|
| assert-nothing | A `Test*` function has no reachable `Error`/`Fatal`/`Fail` call, no assertion-library call, no compile-time `var _ T = ...` assertion, and no `panic` | Add a real assertion, or annotate: `// test-asserts-nothing: <why the oracle is implicit>` |
| tag-orphan | A `_test.go` build constraint needs a `ze_*` tag that no `go test -tags` in `Makefile` or `mk/*.mk` supplies | Add the tag to a `go test` invocation, or delete the file |
