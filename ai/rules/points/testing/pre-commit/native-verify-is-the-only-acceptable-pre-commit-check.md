---
kind: note
level:
stage:
---
A commit owes NO verification pass at all (`ai/rules/pre-release.md`). `./le verify worktree` is the full gate and it is owed before a PUSH. What a commit owes is the focused test for what you changed, run once.
Run that focused test through a native action: `./le job run label unit-pkg command go test PKG=<what you are changing>`, a component group
(`./le test-unit bgp`), or `./le test-unit`. A BARE `go test`
is not: `internal/le/gotoolchain.Toolchain` gives native actions the repository
`GOCACHE`, while a shell run uses ambient toolchain state and shares nothing
with `./le verify current mode full`. It also drops the feature tags, which is
the separate lie recorded above.
