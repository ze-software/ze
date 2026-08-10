---
kind: directive
level: MUST NOT
stage:
---
**What it does NOT read: test files.** `go build` MUST NOT compile `_test.go`, so a
test file committed without its fixture producer stays invisible here.
