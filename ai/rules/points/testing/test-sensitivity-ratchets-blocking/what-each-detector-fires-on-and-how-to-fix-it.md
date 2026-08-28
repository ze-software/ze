---
kind: table
level:
stage:
---
| Detector | Fires when | Fix |
|----------|-----------|-----|
| assert-nothing | A `Test*` function has no reachable `Error`/`Fatal`/`Fail` call, no assertion-library call, no compile-time `var _ T = ...` assertion, and no `panic` | Add a real assertion, or annotate: `// test-asserts-nothing: <why the oracle is implicit>` |
| tag-orphan | A `_test.go` build constraint needs a `ze_*` tag absent from the native action population derived by `internal/le/testsensitivity.TagUniverse` | Add the tag to `feature-gates.txt` when it is a real feature, or delete the unreachable test |
