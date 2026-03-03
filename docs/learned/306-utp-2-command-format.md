# 306 — UTP-2 Text Command Format Unification

## Objective

Remove the accumulator model (`set`/`add`/`del` on attributes, mid-stream modification) from the `update text` command parser and introduce short/long keyword aliases so API output is compact and config output is readable.

## Decisions

- Short/long alias pattern: API output uses short forms (`next`, `path`, `pref`, `s-com`, `l-com`, `e-com`); both forms accepted on input — enables compact wire-speed output and human-readable config
- Shared keyword tables in `textparse/keywords.go` rather than unifying the tokenizers — `TextScanner` (raw string scanning, no quotes) and `tokenize()` (quoted input splitting) serve fundamentally different needs and must stay separate
- Old bracket syntax (`as-path [65001 65002]`) still accepted for transition (AC-3) — forward-only compatibility
- `set` keyword on attributes returns an error with a migration hint — explicit rejection rather than silent ignore
- Attributes after the first `nlri` section are rejected — enforces "attributes precede all nlri sections" invariant

## Patterns

- EVPN, FlowSpec, and VPLS parsers already used flat keyword-value parsing internally — the accumulator model existed only at the top level; removal was isolated to `ParseUpdateText()` and `parsedAttrs`
- `FormatRouteCommand()` in `shared/format.go` is an internal producer of update text commands for route replay — must be updated in lockstep with the parser; this was not in the original spec and was discovered during functional testing

## Gotchas

- 64 `.ci` functional test files used old `set` syntax in Python plugin scripts inside `tmpfs=` blocks — missed in the initial batch migration; required a second pass
- `testHasMED()` helper in the test file became unused after test migration — caught by linter

## Files

- `internal/plugins/bgp/textparse/keywords.go` — shared keyword constants, alias map, resolution
- `internal/plugins/bgp/handler/update_text.go` — accumulator removed, flat parsing, alias resolution
- `internal/plugins/bgp/handler/update_text_nlri.go` — path-id as per-NLRI-section modifier
- `internal/plugins/bgp/format/text.go` — short aliases in API text output
- `internal/plugins/bgp-rs/server.go` — shared keyword tables from textparse/
- `internal/plugin/bgp/shared/format.go` — FormatRouteCommand updated to flat grammar (not in original plan)
