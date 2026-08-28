---
kind: directive
level: MUST
stage:
---
**`./le repository-tracked-build check` is the only gate that compiles what git holds,
and it compiles no `_test.go`. Its green therefore says nothing about the test
build. Before you treat work as committable, you MUST also compile the test
binaries of every package you touched, without running them.**
