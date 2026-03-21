# 399 — RIB Register Builtin Commands Race

## Objective

Fix intermittent `TestProcessInternalPluginStop` failure caused by a data race on unsynchronized package-level state in the RIB plugin's command registration.

## Decisions

- Replaced `bool` guard (`builtinsRegistered`) with `sync.Once` in `registerBuiltinCommands()` -- the bool+map pair was unprotected against concurrent access from multiple plugin goroutines.
- Added `Wait()` to `TestProcessInternalPlugin` which previously called `Stop()` without waiting, leaking a goroutine that could race with subsequent tests.
- Increased `TestProcessInternalPluginStop` Wait timeout from 500ms to 2s to match `TestProcessShutdown` and tolerate CI scheduling delays.

## Patterns

- Tests that call `Start()` and `Stop()` on a Process MUST also `Wait()` to prevent goroutine leaks that cause cross-test data races.
- Package-level mutable state accessed from plugin runner goroutines must use `sync.Once` or a mutex, not a plain bool guard -- even if "read-only after startup" the startup itself can race.
- Reproducing intermittent test failures requires running the failing test together with its neighbors (`-run 'TestA$|TestB'`) at high `-count`, not just the failing test in isolation.

## Gotchas

- The race only manifested when `TestProcessInternalPlugin` and `TestProcessInternalPluginStop` ran in sequence with `-count` > ~200. Running `TestProcessInternalPluginStop` alone never failed because there was no leaked goroutine from a prior test.
- The `registerBuiltinCommands()` comment said "Read-only after startup; no mutex needed" but two concurrent startups (from leaked + new goroutine) broke that assumption.
- The race detector message pointed at `rib_commands.go:84` (bool read) vs `:87` (bool write), not at the map -- fixing the bool guard with `sync.Once` also protects the map writes inside it.

## Files

- `internal/component/bgp/plugins/rib/rib_commands.go` -- `builtinsRegistered bool` replaced with `builtinsOnce sync.Once`
- `internal/component/plugin/process/process_test.go` -- added `Wait()` to `TestProcessInternalPlugin`, increased timeout in `TestProcessInternalPluginStop`
