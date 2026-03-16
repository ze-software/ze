# 245 — RIB Command Unification

## Objective

Unify two parallel RIB command sets by removing engine-side builtin data handlers so plugin-registered commands take over naturally.

## Decisions

- Removed 4 builtin data handlers: `handleRIBShowIn`, `handleRIBShowOut`, `handleRIBClearIn`, `handleRIBClearOut`.
- Meta-commands (show, clear routing) remain in the engine; data handlers move fully to plugin.
- Dispatcher's builtin→subsystem→plugin priority chain handles unification: removing builtins makes commands fall through to plugin.

## Patterns

- Priority chain unification: instead of registering both builtin and plugin handlers, simply remove the builtin and let the dispatcher's natural priority chain route to the plugin.

## Gotchas

- `TestCommandTree` in `cmd/ze/cli/main_test.go` must be updated when commands are removed from builtins — not listed in the original plan.

## Files

- `internal/component/bgp/handler/` — RIB command handlers removed
- `internal/component/bgp/plugins/rib/` — plugin now owns RIB data commands
- `cmd/ze/cli/main_test.go` — command tree test updated
