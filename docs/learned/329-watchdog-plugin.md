# 329 — Watchdog Plugin Extraction

## Objective

Extract all watchdog state and logic from the engine reactor into a new `bgp-watchdog` plugin, completing the "Engine = Protocol, API = Policy" architecture target.

## Decisions

- Chose `update text` commands over internal wire builders: makes the plugin polyglot-compatible and removes wire encoding concerns from the plugin entirely.
- Per-peer config WatchdogGroups and global API pools extracted together — they share command handler, adapter, and reconnect path; splitting them would leave the reconnect path broken.
- Plugin parses config JSON tree independently (same pattern as bgp-gr), no shared types needed between engine and plugin.
- Plugin commands registered with full `bgp watchdog` prefix, not just `watchdog` — dispatcher uses longest-prefix match on the full command string including domain.
- Global API pool commands (bgp watchdog route add) deferred: config-based per-peer pools cover the use case; pool data structure is in place for future addition.

## Patterns

- Engine-to-plugin state extraction pattern: delete reactor fields, delete handler RPCs, delete interface methods, delete config routing — all in one pass after plugin is working.
- `nhop set self` in text commands handles per-peer next-hop resolution transparently — no special plugin logic needed.
- `OnStarted` callback is safe for goroutine launch after 5-stage startup completes.

## Gotchas

- Plugin command names must include domain prefix for dispatcher matching. Commands registered as `watchdog announce` will NOT be found when the dispatcher receives `bgp watchdog announce`. Register as `bgp watchdog announce`.
- `buildStaticRouteWithdraw` became dead code after extraction (sole caller was `WithdrawWatchdog`). Always grep for dead code after removing callers.
- ExaBGP wrapper sends `bgp watchdog announce/withdraw` via `ze-system:dispatch` RPC — needed updating when the command prefix changed.

## Files

- `internal/component/bgp/plugins/watchdog/` — new plugin (watchdog.go, pool.go, config.go, command.go, server.go, register.go)
- `internal/component/bgp/reactor/watchdog.go` — deleted, moved to plugin
- `internal/component/bgp/handler/route_watchdog.go` — deleted, moved to plugin OnExecuteCommand
