# 172 — API Capability Contract

## Objective

Implement a 5-stage plugin registration protocol where plugins proactively declare capabilities, address families, config hooks, and commands at startup — replacing an earlier confirmation model.

## Decisions

- `ConfigProvider` interface on capabilities: each capability self-describes its config values with RFC/draft-scoped keys (e.g., `rfc4724:restart-time`). New capabilities just implement the interface — no reactor changes needed per capability.
- Stage timeout is 5s per stage via `context.WithTimeout()`: each stage must complete within the deadline or the startup coordinator marks the plugin failed.
- Plugin capability injection uses a callback pattern: `Session.pluginCapGetter` is set by `Peer.runOnce()`, then `Session.sendOpen()` appends plugin bytes — decoupled from session creation timing.
- Functional tests for conflict/timeout/failed scenarios declared unnecessary: 22 unit tests cover the protocol thoroughly; functional tests would duplicate unit test coverage.

## Patterns

- `StageComplete()` + `WaitForStage()` barrier: all plugins must complete a stage before any proceeds to the next — ensures deterministic startup ordering.
- Config keys use RFC/draft scoping to prevent collisions between capability implementations (e.g., `draft-walton-bgp-hostname:hostname` vs `rfc4724:restart-time`).

## Gotchas

- All 8 error paths in the server must call `PluginFailed()` + `proc.Stop()` — missing one leaves the startup coordinator stuck waiting forever.
- Plugin `index` field tracks identity for `coordinator.PluginFailed()` — the coordinator needs a stable plugin ID, not just a pointer.

## Files

- `internal/plugin/registration.go` — declaration parsing, `CapabilityInjector`, `PluginRegistry`
- `internal/plugin/startup_coordinator.go` — stage barrier synchronization
- `internal/plugin/server.go` — coordinator + registry + capInjector wiring, `deliverConfig()`
- `internal/plugin/bgp/reactor/reactor.go` — `GetPeerCapabilityConfigs()` implementation
- `internal/plugin/bgp/capability/capability.go` — `ConfigProvider` interface on 8 capabilities
- `test/data/scripts/ze_bgp_api.py` — Python client with full 5-stage protocol
