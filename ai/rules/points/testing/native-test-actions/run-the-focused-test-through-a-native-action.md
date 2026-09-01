---
kind: directive
level: MUST
stage:
---
- **A commit owes the focused test for what it changed, run once; the full gate is owed before a PUSH.** That focused test MUST run through a native action: `./le job run label unit-pkg quiet command go test <package>`, a component group (`./le test-unit bgp`), or `./le test-unit`. Everything after `command` is the child's argv unchanged, so the `PKG=` spelling belongs to `./le fuzz` and `go test` refuses it as an import path.
- **A bare `go test` MUST NOT be used in its place.** `internal/le/gotoolchain.Toolchain` gives native actions the repository build cache and the feature tags, and a shell run has neither.
