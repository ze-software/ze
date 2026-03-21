# 246 — API Types Extraction

## Objective

Extract `RawMessage` and `ContentConfig` from `internal/component/plugin/types.go`, and move `update_text.go`, `update_wire.go`, `route_watchdog.go` to `handler/`.

## Decisions

- All work was completed during spec 244, not as a standalone spec — audit filled retroactively.
- Type aliases (`type X = bgptypes.X`) allow gradual migration without updating all callers at once.
- RPCProviders injection now owns 21 handler RPCs (up from 6 initially).

## Patterns

- Type alias for zero-cost migration: `type RawMessage = bgptypes.RawMessage` in the old location redirects callers without requiring mass updates.

## Gotchas

- Retroactive spec: if work is completed during another spec, audit immediately and move to done/ in the same commit as the code.

## Files

- `internal/component/bgp/types/` — RawMessage, ContentConfig, RIBStatsInfo
- `internal/component/bgp/handler/` — update_text.go, update_wire.go, route_watchdog.go
