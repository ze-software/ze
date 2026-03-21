# 017 — Family-Specific Parsers

## Objective

Add family-specific keyword validation to the API route parser so that invalid keyword combinations (e.g., `rd` for ipv4/unicast) return errors instead of being silently ignored.

## Decisions

- Whitelist approach: three keyword sets (UnicastKeywords, MPLSKeywords, VPNKeywords) each define exactly which attributes are valid for that family — unknown keywords always error.
- Shared `parseCommonAttribute()` for attributes common to all families (origin, med, local-preference, as-path, community, large-community) — DRY.
- `split` keyword valid for unicast but NOT for VPN (VPN uses `rd`/`rt`/`label` instead).
- FlowSpec, VPLS, L2VPN/EVPN keyword validation explicitly deferred to a later spec (`route-families.md`).

## Patterns

- Three-layer parse: family-specific validator → shared common attribute parser → error on unknown.

## Gotchas

- Silent ignore was the previous behavior — any keyword not recognized was simply skipped, making configuration errors invisible.

## Files

- `internal/component/plugin/route_keywords.go` — UnicastKeywords, MPLSKeywords, VPNKeywords
- `internal/component/plugin/route.go` — parseRouteAttributes(), parseLabeledUnicastAttributes(), parseL3VPNAttributes(), parseCommonAttribute()
- `internal/component/plugin/route_parse_test.go`, `handler_test.go` — 40+ tests for keyword rejection
