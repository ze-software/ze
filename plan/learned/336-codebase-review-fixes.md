# 336 — Codebase Review: Critical Fixes

## Objective

Full codebase review identified 14 issues across 5 categories. This entry covers the critical and high-severity bug fixes: panics in production code, a data race, an architecture violation, and dead code removal.

## Decisions

- Replaced panics in `attrpool/pool.go` with error returns — `NewWithIdx()` and `Intern()` now return `(T, error)` instead of panicking on invalid input
- Fixed data race on `p.cmd` in `process.go` by capturing the value under mutex before passing to the `monitor()` goroutine
- Removed 8 deprecated API functions: 6 transaction methods from `ReactorAPI` (superseded by CommitManager) and 2 capability methods (superseded by per-peer variants)
- Fixed architecture violation: `handler/update_text_nlri.go`, `handler/update_text_vpls.go`, and `format/text.go` now use `registry.EncodeNLRIByFamily()` instead of directly importing plugin packages
- Plugins `bgp-nlri-labeled` and `bgp-nlri-vpls` gained `encode.go` files to register their encoders via the registry

## Patterns

- Registry-based plugin lookup is the only correct way for infrastructure to access plugin functionality — direct imports create hidden coupling
- Panics are acceptable in `init()` and tests, never in runtime code paths — always return errors
- When a goroutine needs a field that another goroutine writes, capture it under the same lock before spawning

## Gotchas

- The deprecated transaction API was still in the `ReactorAPI` interface — removing the methods required updating the interface definition in `types/reactor.go`, not just the implementation
- Plugin encoder registration required both a new `encode.go` in each plugin AND updating `register.go` to wire the encoder into the registry

## Files

- `internal/component/bgp/attrpool/pool.go` — panic → error returns
- `internal/component/plugin/process/process.go` — data race fix, runner exit code logging
- `internal/component/bgp/reactor/reactor_api.go` — removed 6 deprecated transaction methods
- `internal/component/bgp/types/reactor.go` — removed transaction methods from interface
- `internal/component/plugin/server/server.go` — removed deprecated `GetPluginCapabilities()`
- `internal/component/plugin/registration.go` — removed deprecated `GetCapabilities()`
- `internal/component/bgp/handler/update_text_nlri.go` — registry lookup
- `internal/component/bgp/handler/update_text_vpls.go` — registry lookup
- `internal/component/bgp/format/text.go` — registry lookup
- `internal/component/bgp/plugins/bgp-nlri-labeled/encode.go` — new encoder registration
- `internal/component/bgp/plugins/bgp-nlri-vpls/encode.go` — new encoder registration
