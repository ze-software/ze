---
kind: note
level:
stage:
---
`./le verify worktree` is the ONLY acceptable pre-commit verification. Not `go test`. Not any subset.
During development: `./le job run label unit-pkg command go test PKG=<what you are changing>`, component groups
(`./le test-unit bgp`), and `./le test-unit` are fine for fast iteration. A BARE `go test`
is not: `internal/le/gotoolchain.Toolchain` gives native actions the repository
`GOCACHE`, while a shell run uses ambient toolchain state and shares nothing
with `./le verify current mode full`. It also drops the feature tags, which is
the separate lie recorded above.
