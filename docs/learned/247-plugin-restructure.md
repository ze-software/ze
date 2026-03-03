# 247 — Plugin Restructure

## Objective

Restructure plugin code to cleanly separate generic plugin infrastructure (`internal/plugin/`) from BGP-specific code (`internal/plugins/bgp/`): rename 10 BGP plugins to `bgp-<name>`, move BGP engine directory, extract types, create sub-packages.

## Decisions

- Phases 1–2 are mechanical (git mv + import path updates).
- Watchdog RPC handlers depend on `RPCRegistration`/`CommandContext`/`ReactorInterface` — cannot move without circular imports; handler move deferred.
- 21/28 non-test files in `internal/plugin/` are now fully generic (zero BGP imports) after all phases.

## Patterns

None beyond standard Go package restructuring.

## Gotchas

- Handler move has 245+ type references; defer until ReactorInterface split reduces coupling (spec 244).
- Circular imports are the blocking constraint for moving handlers — check import graph before attempting.

## Files

- `internal/plugin/` — now generic infrastructure only
- `internal/plugins/bgp/` — BGP engine, handler, bgpserver sub-packages
- `internal/plugins/bgp-*/` — renamed BGP extension plugins
