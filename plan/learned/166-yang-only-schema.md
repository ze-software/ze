# 166 — YANG-Only Schema

## Objective

Eradicate all Go-based schema definitions so YANG becomes the sole source of schema truth. Delete `BGPSchema()`, the `syntaxHints` map, and all Go helper functions from the main parse path.

## Decisions

- Chose YANG extensions (`ze:syntax`, `ze:key-type`) over generating Go schema from YANG: extensions live in the model files, are processed by `yangToNode()` at load time, and require no build step.
- Dynamic module loading deferred: all YANG modules total 32 KB — negligible parse cost; infrastructure exists to add plugin YANG registration later if needed.
- `LegacyBGPSchema()` and its helpers (`peerFields()`, `routeAttributes()`, etc.) intentionally kept: ExaBGP migration tool requires them. The main `YANGSchema()` path does NOT call them.
- `value-or-array` syntax mode added beyond the original six: needed for `as-path [ ... ]` and community attrs where either a bare value or a bracketed list is valid.
- `sortedKeys()` helper added for deterministic field ordering: map iteration caused test flakiness.

## Patterns

- YANG `uses route-attributes` grouping eliminates ~25 lines of Go per usage site — prefer YANG groupings over Go helper functions for shared field sets.
- `ze:syntax "flex"` on a YANG leaf means the parser accepts flag (`;`), value (`X;`), or block (`{ }`). Encode the parsing mode in the model, not in code.

## Gotchas

- `goyang` entry tree vs. container: raw `yang.Container` doesn't have `Mandatory` resolved. Must call `Resolve()` then navigate via `ToEntry()` to get the processed tree with all fields set.
- Map iteration over YANG fields is non-deterministic: always sort keys before building the schema node slice.
- `add-path` was expected as an enum by one test but YANG defines it as a container with `send`/`receive` children — test had to be updated to match YANG structure.

## Files

- `internal/yang/modules/ze-extensions.yang` — custom syntax extensions
- `internal/yang/modules/ze-types.yang` — `route-attributes` grouping
- `internal/yang/modules/ze-bgp.yang` — complete BGP schema with extensions
- `internal/yang/modules/ze-hub.yang` — environment block schema
- `internal/component/config/yang_schema.go` — extension processing, deleted `syntaxHints`
- `internal/component/config/bgp.go` — deleted `BGPSchema()`, kept `LegacyBGPSchema()` + helpers
