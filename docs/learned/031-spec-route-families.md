# 031 — Route Family Keyword Validation

## Objective

Add keyword validation for remaining BGP address families (FlowSpec, VPLS, L2VPN/EVPN) to reject invalid route attributes early with clear errors.

## Decisions

Mechanical implementation — keyword sets defined for each family and validated in the announce handlers. No significant design decisions.

## Patterns

None beyond the existing per-family keyword set pattern already in place for unicast and VPN families.

## Gotchas

None.

## Files

- `internal/plugin/route.go` — `FlowSpecKeywords`, `VPLSKeywords`, `L2VPNKeywords` sets + handler validation
- `internal/plugin/route_parse_test.go` — validation tests
