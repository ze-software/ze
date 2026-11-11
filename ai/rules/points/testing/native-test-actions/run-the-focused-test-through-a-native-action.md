---
kind: directive
level: MUST
stage:
---
- **A commit and a push owe the focused test for what they changed, run once; the full gate is a release check and is owed by neither.** That focused test MUST run through a native action: `./le test unit pkg path <dir>/ [run <regex>] [vet]`, which carries the feature tags and the race detector, or a component group (`./le test unit bgp`).
- **A package run the short action does not express (`-v`, `-count`, a benchmark, a tag the feature set lacks) MUST go through `./le job run label unit-pkg quiet command go test -race -tags "$tags" <package>`, with the tags written out.** Everything after `command` is the child's argv unchanged, so the `PKG=` spelling belongs to `./le test fuzz` and `go test` refuses it as an import path.
- **A bare `go test` MUST NOT be used in its place.** `internal/le/go/toolchain.Toolchain` gives native actions the repository build cache and the feature tags, and a shell run has neither.
