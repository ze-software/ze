# 301 — RIB-04: Plugin Dependency Declarations

## Objective

Fix silent degradation in bgp-rr when bgp-adj-rib-in is absent (replayDisabled permanently set, late-connecting peers miss routes) by adding a two-layer dependency system: Go registry for pre-startup auto-loading + protocol stage 1 for runtime validation.

## Decisions

- Two-layer system: Go registry `Dependencies []string` for pre-startup auto-loading of internal plugins; protocol `DeclareRegistrationInput.Dependencies` for runtime validation of all plugin types
- Internal plugins (goroutines): auto-spawned by config loader as `Internal: true` — full safety net, operator never needs to configure them explicitly
- External plugins (forked binaries): declare deps via protocol only, validate-only — cannot be auto-spawned, operator must configure them explicitly
- Iterative loop-until-stable for dependency expansion over topological sort — simpler, handles transitive deps naturally
- `expandDependencies()` wired into ALL 3 loading paths (LoadReactor, LoadReactorWithPlugins, LoadReactorFileWithPlugins) — production, test, and config-file loading all benefit
- Fail loudly at stage 1 on missing dependency instead of silent degradation — matches ze's fail-early philosophy; replayDisabled was the original bug
- Stage 1 validation in `server_startup.go:handleProcessStartupRPC` (production path), NOT `subsystem.go:completeProtocol` — subsystem.go is NOT the production path

## Patterns

- DFS cycle detection in `detectCycles()` alongside iterative expansion — separation of concerns
- `hasConfiguredPlugin()` helper on Server checks `s.config.Plugins` — config slice built before startup, includes auto-added deps
- `PluginNames()` accessor on Reactor exposes plugin names for integration testing (added during implementation, not in original spec)
- hugeParam lint threshold bumped 256→280 after Registration struct grew by 24 bytes (Dependencies slice header)

## Gotchas

- Wrong production path (same mistake as spec rib-01 mistake log): spec originally targeted `subsystem.go:completeProtocol` for stage 1 validation. Production plugins go through `server_startup.go:handleProcessStartupRPC`. `RunningPlugins` in subsystem.go is never set in production. Caught during Critical Review. Rule: grep ALL implementations of a protocol step, identify which one the consumer actually calls.
- `bgp-rr/server_test.go` needed no changes — no existing tests asserted on `replayDisabled` (the field was hidden behind atomics with no test hook)

## Files

- `internal/plugin/registry/registry.go` — Dependencies field, validation, `ResolveDependencies()`, `detectCycles()`
- `internal/plugin/server_startup.go` — stage 1 dependency validation (production path)
- `internal/plugin/server.go` — `hasConfiguredPlugin()` helper
- `internal/config/loader.go` — `expandDependencies()` + wired into 3 loading paths
- `internal/plugins/bgp-rr/register.go` — `Dependencies: []string{"bgp-adj-rib-in"}`
- `internal/plugins/bgp-rr/server.go` — `replayDisabled` completely removed
- `internal/plugins/bgp/reactor/reactor.go` — `PluginNames()` accessor
- `.golangci.yml` — hugeParam threshold 256→280
