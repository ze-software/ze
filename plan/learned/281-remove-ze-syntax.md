# 281 — Remove ze:syntax (Standard YANG Config)

## Objective

Remove all custom `ze:syntax` YANG extensions from ze configuration and replace with standard YANG constructs, so the parser understands leaf-list, presence containers, and lists natively — not via per-node annotations.

## Decisions

- ALL `ze:syntax` annotations are display artifacts over standard YANG types: `value-or-array/bracket/multi-leaf` → `leaf-list`; `flex` → `presence container`; `family-block/allow-unknown-fields` → `list`. The core insight: the parser should handle standard YANG natively, not require extensions.
- Mandatory `add`/`del`/`eor` operation keyword in NLRI: `add`/`del`/`eor` serves as a structural boundary between family metadata (rd, label) and payload (prefixes, criteria). Making it mandatory removes the last NLRI parsing ambiguity and required updating 40+ encode test files.
- `freeform` is incompatible with schema validation: `parseFreeform()` stores entire word sequences as single opaque map keys — consumers must split with hardcoded iteration. This prevents schema-level validation and is the root cause of all `ze:syntax` workarounds.
- Scope expanded from Phases 1-6 to 1-11 during implementation: original plan only covered freeform removal. `flex` and leaf-list annotations are also `ze:syntax` extensions and were also removed in the same effort.
- `flow { route { match {} then {} } }` deleted from YANG: ExaBGP legacy that migration already converts to `update { nlri {} }`. Dead in ze-native configs. Migration now returns an error for configs containing `multi-session`, `operational`, or `aigp` capabilities.

## Patterns

- Parser principle for leaf-list: standard YANG `leaf-list` accepts `name value;` (single) or `name [ v1 v2 ];` (multiple) — no annotation needed.
- Parser principle for presence containers: accepts `name;` (flag), `name value;` (inline child), `name { children; }` (block). Same behavior as `flex`, now driven by the YANG `presence` statement.
- Scan/tokenize in `extractRoutesFromUpdateBlock`: `[` after operation keyword → bracket list; word → single structured NLRI. Clear dispatch grammar eliminates ambiguity.

## Gotchas

- Extended community `L` suffix parsing broke during NLRI restructuring (fixed in b4808c50) — unrelated to the ze:syntax removal but surfaced during the test run.
- Migration serializer had separate NLRI iteration logic from the YANG-aware serializer (fixed in ebd84522) — two code paths that diverged over time, both needed updating.
- Presence container flag-mode parsing needed a new parser path not in the original design (fixed in b4808c50).
- List inline syntax "last-child-absorbs-remaining" needed for NLRI content where payload is variable-length (fixed in b4808c50).
- `nlri-mandatory-add.ci` renamed to `nlri-requires-operation.ci` — better describes the rejection: the operation keyword (add/del/eor) is required, not just `add`.

## Files

- `internal/component/bgp/yang/ze-bgp-conf.yang` — dead nodes removed, freeform→list, ~35 annotations removed, dead import cleaned
- `internal/component/config/schema.go` — `Presence bool` added to `ContainerNode`
- `internal/component/config/yang_schema.go` — leaf-list default, presence detection, flex→presence mapping
- `internal/component/config/parser.go` — presence container parsing, enum leaf-list
- `internal/component/config/parser_list.go` — last-child-absorbs-remaining for inline list
- `internal/component/config/bgp_routes.go` — mandatory op enforcement, list iteration for NLRI
- `internal/component/bgp/reactor/config.go` — structured nexthop/add-path/process parsing
- `internal/exabgp/migrate.go` — `checkUnsupported()`, dead capability removal
- `docs/architecture/config/syntax.md` — standard YANG approach documented
- `test/encode/*.ci` (40 files) + `test/parse/*.ci` (16 files) + `test/plugin/*.ci` (16 files) — updated for new syntax
