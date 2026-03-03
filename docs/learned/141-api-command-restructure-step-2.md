# 141 — API Command Restructure Step 2: Plugin Namespace

## Objective

Move session lifecycle commands from `session api ready/ping/bye` to `plugin session ready/ping/bye` and add plugin introspection commands (`plugin help`, `plugin command list/help/complete`). Remove `session reset`.

## Decisions

- Handlers added to existing `plugin.go` (which already had plugin-related parsing functions) rather than creating a new file — the existing file was the right home.
- `session sync enable/disable` and `session api encoding` left at old paths — Step 4 moves these to `bgp plugin` namespace.
- Functional test `plugin-session.ci` not created — existing unit tests provided sufficient coverage.

## Patterns

- Plugin introspection (`plugin command list`) returns only plugin-registered commands, not builtins. Separation of built-in vs. plugin commands at the command registry level.

## Gotchas

None.

## Files

- `internal/plugin/plugin.go` — `RegisterPluginHandlers()`, introspection handlers
- `internal/plugin/session.go` — `handlePluginSession*` handlers, old session handlers removed
- `internal/plugin/handler.go` — `RegisterPluginHandlers()` call added
- `internal/plugin/rib/rib.go` — `session api ready` → `plugin session ready`
