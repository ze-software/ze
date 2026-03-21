# 270 — Subscription Summary Format

## Objective

Add a `"summary"` subscription format that extracts lightweight NLRI metadata (section presence + MP family names) from UPDATE messages at near-zero cost — a few byte reads instead of full attribute/NLRI parsing.

## Decisions

- Short-circuit in `FormatMessage` BEFORE `filter.ApplyToUpdate` — avoids the full filter/parse cycle for plugins that only need presence information.
- `scanMPFamilies` walks attribute headers only (not values), reading 3 bytes (AFI+SAFI) for codes 14/15 — no NLRI decode, no attribute value parsing.
- `message.id` is always present in summary output even when 0, unlike other formats — this is an intentional divergence documented in the implementation notes.
- Non-UPDATE messages with format `"summary"` fall through to parsed behavior — no summary fields for OPEN/NOTIFICATION.
- Attribute codes 14/15 referenced via `attribute.AttrMPReachNLRI` / `attribute.AttrMPUnreachNLRI` constants — no magic numbers.

## Patterns

- Per RFC 4760, at most one MP_REACH_NLRI and one MP_UNREACH_NLRI per UPDATE; fields are scalar strings, not arrays.
- `wire.ParseUpdateSections` gives section offsets at near-zero cost (lazy-cached in WireUpdate) — reused as-is.
- Pre-existing lint issues in `text.go` (7x `WriteString(Sprintf(...))` → `Fprintf`) fixed during implementation.

## Gotchas

- None.

## Files

- `internal/component/bgp/format/summary.go` — `formatSummary`, `scanMPFamilies`, `buildSummaryJSON` (133 lines, created)
- `internal/component/bgp/format/summary_test.go` — 10 unit tests (351 lines, created)
- `internal/component/plugin/types.go` — `FormatSummary` constant added
- `internal/component/bgp/format/text.go` — summary dispatch + 7 pre-existing lint fixes
- `test/plugin/summary-format.ci` — functional end-to-end test (166 lines, created)
