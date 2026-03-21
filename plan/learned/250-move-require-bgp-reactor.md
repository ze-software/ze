# 250 — Move RequireBGPReactor

## Objective

Move `RequireBGPReactor()` from `internal/component/plugin/command.go` into `internal/component/bgp/handler/require.go` as package-private `requireBGPReactor()`, removing BGP-specific logic from generic plugin infrastructure.

## Decisions

- `RequireBGPReactor()` becomes package-private in `handler/` — only BGP handler code calls it.
- All 14 callers are in `internal/component/bgp/handler/`, so no external callers need updating.

## Patterns

None beyond standard function relocation.

## Gotchas

None noted (spec archived as skeleton without full Implementation Summary).

## Files

- `internal/component/bgp/handler/require.go` — `requireBGPReactor()` (package-private)
- `internal/component/plugin/command.go` — `RequireBGPReactor()` removed
