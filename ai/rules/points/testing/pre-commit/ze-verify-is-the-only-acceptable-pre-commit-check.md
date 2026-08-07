---
kind: note
level:
stage:
---
`make ze-verify` is the ONLY acceptable pre-commit verification. Not `go test`. Not any subset.
During development: `make ze-test-pkg PKG=<what you are changing>`, component groups
(`make ze-test-bgp`), and `make ze-unit-test` are fine for fast iteration. A BARE `go test`
is not: the Makefile exports `GOCACHE` to `cache/go-cache` into its own recipes only, so a
shell run uses `~/.cache/go-build`, rebuilds cold, and shares nothing with `ze-verify`. It
also drops the feature tags, which is the separate lie recorded above.
