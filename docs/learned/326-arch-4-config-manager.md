# 326 — Architecture Phase 4: Config Manager Implementation

## Objective

Build the standalone `internal/configmgr/` implementation satisfying `ze.ConfigProvider`. Config loading, root-based subtree queries, YANG schema registration, save, and watch notifications for reload.

## Decisions

- `internal/component/config/` YANG-aware parsing wiring deferred to Phase 5 — `plugin.Hub` has deep coupling to SchemaRegistry, SubsystemManager, and RPC dispatch. Same standalone-first approach as Phases 2 and 3.
- JSON-based `Load`/`Save` in this phase — simple stub that Phase 5 replaces with YANG-aware parsing
- Type named `ConfigManager` (not `Manager`) to avoid hook collision with `pluginmgr.Manager`
- Watch channels buffered at capacity 1 with "drop oldest, send newest" pattern — prevents blocking publisher; latest config is what matters
- Notifications collected under lock, sent outside lock — prevents deadlock if consumer calls back into ConfigManager
- `Get` returns empty map (not nil) for missing roots — satisfies `nilnil` linter (AC-3 changed from spec's `nil, nil`)
- `gosec` nolint for `os.ReadFile` — config path is inherently user-provided, not a security issue here
- Save uses `0o600` permissions — gosec requirement

## Patterns

- `maps.Copy` (Go 1.21+) for defensive copies of subtrees — callers cannot mutate internal config
- All three arch phases (Bus, PluginManager, ConfigProvider) follow the same pattern: satisfy interface fully, wire to existing internals in Phase 5

## Gotchas

- AC-3 changed: spec said `Get` returns `nil, nil` for missing root; linter requires non-nil return — returns empty map instead. Functionally equivalent for callers using `len(result) == 0`.

## Files

- `internal/configmgr/manager.go` — implementation (172 lines)
- `internal/configmgr/manager_test.go` — 13 tests including watch, multiple watchers, reload notification
