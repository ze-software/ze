# 1173 -- cgo-race-requires-cgo

## Context

`Makefile:16 export CGO_ENABLED := 0` (commit 10708d7dc, "build: disable CGO globally") was
added so release binaries link statically. But a Makefile `export` reaches EVERY recipe, and
`go test -race` links the race runtime through cgo. So every `-race` target aborted with
`-race requires cgo; enable cgo by setting CGO_ENABLED=1`: `ze-unit-test`,
`ze-unit-test-race-changed`, `ze-test-*`, `ze-race-reactor`, `ze-chaos-unit-test`, and the
native `ze-integration-*`/`ze-stress-web-test` race targets.

## Decisions

- Couple cgo to race at the invocation, not globally. Added `GO_TEST_RACE` /
  `GO_TEST_CORE_RACE` (`GOMAXPROCS=... CGO_ENABLED=1 go test -tags '...' -race`) and routed
  EVERY `-race` site through them across `Makefile`, `mk/test-unit.mk`, `mk/test-chaos.mk`,
  and `mk/test-integration.mk`. Non-race runs (`ze-unit-test-cached`) stay CGO-free; release
  builds stay static. Chose this over dropping the global export because static release
  binaries were the point of 10708d7dc.

## Consequences

- `make ze-unit-test-race-changed` (the race stage inside `ze-verify`) runs again instead of
  exiting 2 the moment a working tree has an uncommitted `.go` change.
- Any NEW `-race` target must use `$(GO_TEST_RACE)` / `$(GO_TEST_CORE_RACE)`, or set
  `CGO_ENABLED=1` inline (as the native integration targets now do), never bare
  `$(GO_TEST) -race`.

## Gotchas

- **The gate was green on a clean tree and red only with uncommitted Go.** `ze-unit-test-race-changed`
  maps changed `.go` files to component groups and prints "No changed .go files -- skipping -race
  pass" when there are none. So a committed tree skipped the broken step entirely and `ze-verify`
  passed; the failure surfaced only mid-development and was repeatedly filed as a "pre-existing
  unrelated functional failure." When a gate fails ONLY with uncommitted changes, suspect a recipe a
  clean tree skips, not a flake.
- A global `export CGO_ENABLED := 0` and `go test -race` are mutually exclusive by construction;
  grepping `-- -race` across `Makefile` + `mk/*.mk` is the way to find every site, including the
  `$(GO) test ... -race` ones that do not go through `$(GO_TEST)`.

## Files

- `Makefile` (`GO_TEST_RACE`, `GO_TEST_CORE_RACE`; `ze-race-reactor`, `ze-unit-test-changed`).
- `mk/test-unit.mk` (`ze-unit-test`, `-cover`, `-race-changed`, `ze-test-{bgp,core,plugins,config,cli,rest}`).
- `mk/test-chaos.mk` (`ze-chaos-unit-test`).
- `mk/test-integration.mk` (`ze-stress-web-test`, `ze-integration-{iface,fib,firewall,traffic,gtsm,as112}-test` -- inline `CGO_ENABLED=1`).
