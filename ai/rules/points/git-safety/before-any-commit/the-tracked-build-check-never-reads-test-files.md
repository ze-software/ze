---
kind: directive
level:
stage:
---
**What it does NOT read: test files.** `go build` never compiles `_test.go`, so a
test file committed without its fixture producer stays invisible here.
