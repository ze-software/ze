---
kind: directive
level:
stage:
---
**Go through `make`, or carry `GOCACHE` yourself.** `Makefile` exports
`GOCACHE := $(CURDIR)/cache/go-cache`, and that export reaches make RECIPES only. A
bare `go test` typed into a shell uses the user's own `~/.cache/go-build` instead,
so it rebuilds the world cold, shares nothing with `ze-verify`, and leaves the
project cache no warmer than it found it. `Makefile` also defines the canonical
invocation (`GO_TEST`, `GO_TEST_RACE`): the feature tags, the timeout, `GOMAXPROCS`
and `CGO_ENABLED=1` for race. A bare `go test` drops all of it
(`ai/rules/commands.md`, "Bare `go test` Lies").
