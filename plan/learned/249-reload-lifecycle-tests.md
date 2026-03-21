# 249 — Reload Lifecycle Tests

## Objective

Umbrella spec summarising the config reload pipeline implementation (sub-specs 222–234); all sub-specs complete.

## Decisions

- Reload pipeline is generic: coordinator works with `map[string]any`, never touches BGP-specific types.
- Reactor is called directly by the coordinator (not via RPC) since the reactor IS the engine process.

## Patterns

- Reload pipeline: SIGHUP → coordinator → diff config → notify subsystems → subsystems apply changes.
- Generic coordinator: coordinator uses `map[string]any` config representation; subsystems interpret their own slice.

## Gotchas

None.

## Files

- Umbrella only — see sub-specs 222–234 for implementation details.
- `internal/component/config/` — config coordinator
- `internal/component/bgp/reactor/` — BGP reload handler
