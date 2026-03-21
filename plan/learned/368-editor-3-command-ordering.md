# 368 — Editor Command Ordering (RequiresSelector)

## Objective
Enforce that peer-mutating commands require an explicit peer selector — prevent `bgp peer eorr` (no selector) from being accepted.

## Decisions
- Added `RequiresSelector` field to `RPCRegistration` struct — opt-in per handler
- Enforcement in `Dispatch()` at dispatcher level — before handler is called
- Wildcard `*` counts as explicit selector
- Read-only commands (summary, capabilities) don't require selector

## Patterns
- `RegisterWithOptions` method for new registrations with additional fields
- `hasExplicitSelector` tracked during peer-selector extraction in `Dispatch()`
- Error message includes command syntax hint: `"bgp peer <address> <command>"`

## Gotchas
- Test handlers returning `nil, nil` trigger `nilnil` linter — use `return &plugin.Response{Status: "done"}, nil`
- TDD verification: reject tests correctly FAILED before implementation (handler was called), then PASSED after

## Files
- `internal/component/plugin/server/handler.go` — RequiresSelector field
- `internal/component/plugin/server/command.go` — RegisterWithOptions, Dispatch enforcement
- `internal/component/bgp/handler/bgp.go`, `refresh.go`, `raw.go`, `update_text.go`, `bgp_summary.go` — RequiresSelector on mutating handlers
