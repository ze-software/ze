# 081 — Update Text Parser

## Objective

Implement the `ParseUpdateText` parser for `update text attr set|add|del ... nlri <family> add|del ...` commands — foundation for the announce-family-first API refactor.

## Decisions

- `rd` and `label` are per-NLRI parameters parsed within the `nlri` section, NOT global attributes: routing distinguishers belong to individual prefixes, not to the entire update. Using them in `attr set` returns an "unknown attribute" error.
- AS-PATH is set-only despite being a slice: add/del would compromise path integrity; `ErrASPathNotAddable` returned for add/del attempts.
- `snapshot()` MUST deep copy all slices: attributes accumulate across sections; each `nlri` section captures the current state. Without deep copy, later modifications to the accumulator corrupt earlier snapshots.
- `applySet` merges only if field is explicitly set (pointer fields non-nil, slices non-nil, NextHop valid): allows partial attribute updates without clearing unset fields.
- Boundary keywords (`attr`, `nlri`, `watchdog`) terminate the current section — no explicit length needed.

## Patterns

- Parser state machine: `parsedAttrs` accumulator modified by `applySet/applyAdd/applyDel`, snapshot taken at each `nlri` boundary.
- `parseCommonAttribute` reused from `route.go` for standard attribute parsing; next-hop handled separately (not in `parseCommonAttribute`).

## Gotchas

- Multiple `add`/`del` mode switches within a single `nlri` section are allowed: `add X del Y add Z` → announce=[X,Z], withdraw=[Y]. The parser uses a simple mode variable, not a stack.

## Files

- `internal/plugin/update_text.go` — ParseUpdateText, parseAttrSection, parseNLRISection
- `internal/plugin/types.go` — NLRIGroup, UpdateTextResult
- `internal/plugin/errors.go` — parser error sentinels
