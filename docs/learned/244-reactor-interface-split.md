# 244 — Reactor Interface Split

## Objective

Split the monolithic `ReactorInterface` (68 methods) into `ReactorLifecycle` (17 generic methods) and `BGPReactor` (38 BGP-specific methods) to remove BGP coupling from generic plugin infrastructure.

## Decisions

- "Expand then contract" pattern: add `RequireBGPReactor()` first while keeping old types, migrate all callers, then narrow type signatures — prevents goimports cascade during cross-file refactoring.
- BGP callback methods stay as direct methods on `server_bgp.go` via `ServerBGPCallbacks` struct rather than adding to interface.
- `rib_handler.go` stayed in `internal/plugin/` — belongs to separate rib-command-unification spec.

## Patterns

- "Expand then contract": when narrowing types across many files, add the new narrower helper first, migrate callers, then delete the wide type.
- `ServerBGPCallbacks` struct: BGP-specific callbacks injected into generic server without widening the interface.

## Gotchas

- Typed nil `*mockReactor` is non-nil as an interface — use `plugin.ReactorLifecycle` as the parameter type in test helpers, not `*mockReactor`.
- goimports cascade: touching interface definitions triggers import rewrites across all implementors; sequence edits carefully.

## Files

- `internal/plugin/server.go` — ReactorLifecycle interface
- `internal/plugins/bgp/bgpserver/` — BGPReactor interface, ServerBGPCallbacks
