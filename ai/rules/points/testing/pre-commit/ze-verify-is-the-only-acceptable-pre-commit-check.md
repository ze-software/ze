---
kind: note
level:
stage:
---
`make ze-precommit-verify` is the ONLY acceptable pre-commit verification. Not `go test`. Not any subset.
During development: `make ze-unit-pkg-test PKG=<what you are changing>`, component groups
(`make ze-unit-bgp-test`), and `make ze-unit-test` are fine for fast iteration. A BARE `go test`
is not: the Makefile exports `GOCACHE` to `cache/go-cache` into its own recipes only, so a
shell run uses `~/.cache/go-build`, rebuilds cold, and shares nothing with `ze-precommit-verify`. It
also drops the feature tags, which is the separate lie recorded above.
