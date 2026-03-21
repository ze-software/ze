# 159 — Hub Phase 2: Config Parsing

## Objective

Implement 3-section config parsing in the hub: `env { }` (global settings), `plugin { }` (process declarations), and remaining blocks (plugin configs stored for later routing).

## Decisions

- Hub stores config as `map[string]any` (nested maps), not as Go structs. Schema-agnostic storage allows JSON conversion and lets plugins define their own structure via YANG.
- Env block handling is hub-only: `api-socket`, `log-level`, `working-dir`. These settings affect the hub process itself.
- YANG values for config blocks deferred to Phase 3 — this phase just tokenizes and stores.
- Implementation Summary left blank — actual implementation tracked in 157-hub-separation-phases.

## Patterns

- Reuses existing `internal/config.NewTokenizer()` — no new parser needed for the 3-section config syntax.

## Gotchas

None documented — spec was a planning document; actual implementation tracked in 157-hub-separation-phases.

## Files

- `internal/component/hub/config.go` — 3-section config parser (env, plugin, blocks)
