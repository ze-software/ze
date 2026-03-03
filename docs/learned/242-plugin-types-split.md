# 242 — Plugin Types Split

## Objective

Extract 30 BGP-specific types from `internal/plugin/types.go` into `internal/plugins/bgp/types/` as phase 3 of the plugin restructure.

## Decisions

- Mechanical move: 30 types relocated, all import paths updated.
- `LargeCommunity` alias kept in new package (re-exported from attribute package) rather than updating all call sites.

## Patterns

- Type alias for re-export: `type LargeCommunity = attribute.LargeCommunity` avoids mass call-site updates during mechanical moves.

## Gotchas

- Hook-driven workflow creates a chicken-and-egg during type moves: the duplicate-type lint check blocks adding types to the new file while the old file still exports them. Disable or sequence carefully.

## Files

- `internal/plugins/bgp/types/` — BGP-specific plugin types (30 types)
- `internal/plugin/types.go` — now contains only generic plugin types
