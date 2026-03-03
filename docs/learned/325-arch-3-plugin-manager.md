# 325 — Architecture Phase 3: Plugin Manager Implementation

## Objective

Build the standalone `internal/pluginmgr/` implementation satisfying `ze.PluginManager`. Plugin registration, lifecycle tracking (register → start → stop), capability collection, plugin queries.

## Decisions

- Server restructuring deferred to Phase 5 — existing Server has deep BGP coupling through ReactorLifecycle (16 methods) and BGPHooks. Same pragmatic standalone-first approach as Phase 2 Bus.
- `NewManager()` constructor following Bus naming pattern (not `New()`)
- `started` bool field prevents registration after `StartAll()` — explicit guard, no silent ignore
- `AddCapability()` method added (not in interface) for Stage 3 of 5-stage protocol and testing — deliberate addition beyond the interface
- Capabilities returned as defensive copy — callers cannot mutate internal state
- Stores `ze.Bus` and `ze.ConfigProvider` references from `StartAll()` — Phase 5 wires these into actual plugin startup

## Patterns

- Each phase builds a standalone component that fully satisfies its interface; integration is Phase 5's job
- `pluginState` internal type tracks per-plugin config + running flag — private implementation detail, not exported
- Thread-safe via `sync.RWMutex` — reads (Plugin, Plugins, Capabilities) use RLock; writes (Register, StartAll, StopAll) use full lock

## Gotchas

- None.

## Files

- `internal/pluginmgr/manager.go` — implementation (143 lines)
- `internal/pluginmgr/manager_test.go` — 10 tests covering full lifecycle
