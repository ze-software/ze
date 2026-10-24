---
kind: directive
level: MUST
stage:
---
- **A commit and a push owe the focused test for what they changed, run once; the full gate is a release check and is owed by neither.** That focused test MUST run through a native action: `./le job run label unit-pkg quiet command go test <package>` or a component group (`./le test unit bgp`). Everything after `command` is the child's argv unchanged, so the `PKG=` spelling belongs to `./le test fuzz` and `go test` refuses it as an import path.
- **A bare `go test` MUST NOT be used in its place.** `internal/le/go/toolchain.Toolchain` gives native actions the repository build cache and the feature tags, and a shell run has neither.
