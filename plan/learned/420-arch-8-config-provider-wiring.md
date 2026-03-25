# 420 -- Arch-8: ConfigProvider Wiring

## Context

The Engine used a nop `stubConfigProvider()` that returned empty maps. The real `config.Provider` existed (built in arch-4) but wasn't wired. `LoadReactorWithPlugins` bundled config loading and reactor creation in one monolithic function, preventing ConfigProvider from being the config authority.

## Decisions

- Added `Provider.SetRoot(name, tree)` to accept pre-parsed config trees — chosen over `Provider.Load(path)` because config is already parsed by the YANG pipeline. SetRoot notifies watchers, same as Load.
- Decomposed `LoadReactorWithPlugins` into `LoadConfig` (returns tree + plugins) and `CreateReactor` (creates reactor from tree) — chosen over a single function because the caller needs to populate ConfigProvider between parsing and reactor creation.
- `LoadReactorWithPlugins` preserved as a convenience wrapper around LoadConfig + CreateReactor — backward compatibility for existing callers.
- EOR Bus notification via package-level `eorBus` variable in `events.go` — chosen over threading Bus through `onMessageReceived` → `onEORReceived` call chain (would change 4 function signatures). SetBus on EventDispatcher sets the package var.

## Consequences

- ConfigProvider is the config authority — reactor and future consumers read from it.
- `LoadConfig` + `CreateReactor` split enables future work where PluginManager reads config between parsing and reactor creation.
- EOR events now publish to Bus — cross-component consumers can react to end-of-RIB markers.
- `nopConfigProvider` stub removed from `cmd/ze/hub/main.go`.

## Gotchas

- `Provider.SetRoot(nil)` stores empty map (not nil) — defensive, prevents nil deref in Get().
- EOR detection is 3 calls deep from reactor (`onMessageReceived` → `onEORReceived`) — package-level var was the pragmatic solution to avoid invasive signature changes.
- `tree.ToMap()` returns `map[string]any` where values may not be `map[string]any` — non-map roots are logged and skipped.

## Files

- `internal/component/config/provider.go` — SetRoot method
- `internal/component/config/provider_test.go` — 4 SetRoot tests
- `internal/component/bgp/config/loader.go` — LoadConfig + CreateReactor decomposition
- `cmd/ze/hub/main.go` — real ConfigProvider, LoadConfig + CreateReactor
- `internal/component/bgp/server/event_dispatcher.go` — Bus field, SetBus
- `internal/component/bgp/server/events.go` — eorBus package var, bgp/eor publish
