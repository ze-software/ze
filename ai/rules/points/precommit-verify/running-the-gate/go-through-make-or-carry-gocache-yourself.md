---
kind: directive
level: MUST
stage:
---
**You MUST go through `make`, or carry `GOCACHE` yourself.** `Makefile` exports
`GOCACHE := $(CURDIR)/cache/go-cache`, and that export reaches make RECIPES only. A
bare `go test` typed into a shell uses the user's own `~/.cache/go-build` instead,
so it rebuilds the world cold, shares nothing with `ze-precommit-verify`, and leaves the
project cache no warmer than it found it. `Makefile` also defines the canonical
invocation (`GO_TEST`, `GO_TEST_RACE`): the feature tags, timeout, and
`GOMAXPROCS`. `GO_TEST` explicitly uses `CGO_ENABLED=0`. The test-only
`GO_TEST_RACE` explicitly uses `CGO_ENABLED=1` with `-race` on Linux and Darwin.
Its race-built test executables never ship or serve as release/build evidence.
A bare `go test` drops all of it (`ai/rules/commands.md`, "Bare `go test` Lies").
