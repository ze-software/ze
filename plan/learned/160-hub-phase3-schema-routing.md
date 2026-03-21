# 160 — Hub Phase 3: Schema Routing

## Objective

Integrate SchemaRegistry with hub for config routing using a VyOS-inspired live/edit config model where plugins query the hub for config data rather than the hub pushing config.

## Decisions

- Pull model (hub notifies, plugins query): hub sends `config verify` / `config apply` signals; plugins respond by querying `query config live|edit path "..."`. Hub never pushes config data unprompted.
- Priority ordering for verify/apply: plugins with lower priority numbers are notified first. Default priority 1000.
- Shared diff library (`internal/component/config/diff/`) for plugins to compute live→edit diffs — diff responsibility belongs to plugins, not hub. Hub serves raw config, plugins decide what changed.
- Sub-root handler routing: handler declaring `bgp.peer.capability.graceful-restart` receives only that subtree when `bgp` root handler also exists.
- Implementation Summary left blank — actual implementation tracked in 157-hub-separation-phases.

## Patterns

- `query config live|edit path "<path>"` → `@serial done data '<json>'` — text protocol with JSON data payload. Hub responds with subtree JSON or error if path not found.

## Gotchas

None documented — spec was a planning document; actual implementation tracked in 157-hub-separation-phases.

## Files

- `internal/component/hub/schema.go` — schema handling, JSON conversion
- `internal/component/config/diff/diff.go` — shared diff library for plugins
