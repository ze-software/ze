# 123 — JSON Validation: FlowSpec Extension

## Objective

Extend the JSON validation framework (spec 121) to cover FlowSpec families (`ipv4/flowspec`, `ipv6/flowspec`) in addition to unicast families.

## Decisions

- FlowSpec NLRI contains rule components (destination-ipv4, tcp-flags, protocol, etc.) not simple prefixes — chose to preserve these as an object under the `nlri` key rather than a flat array.
- FlowSpec "no-nexthop" (rate-limit, discard actions): `next-hop` is omitted from the `nlri` object, not set to empty string.

## Patterns

- The plugin format for FlowSpec uses `"nlri": {"destination-ipv4": [...], "tcp-flags": [...], "string": "..."}` — components as named arrays inside an object, not a flat NLRI list.
- `isFlowSpecFamily()` helper detects FlowSpec-specific families for routing to the FlowSpec transformer vs. unicast transformer.

## Gotchas

- Same withdraw format ambiguity as spec 121: `[{"nlri":"..."}]` vs `["..."]` — both must be handled.

## Files

- `internal/test/runner/json.go` — `isFlowSpecFamily()`, `transformFlowspecAnnounce()`, `transformFlowspecWithdraw()`
- `internal/test/runner/json_test.go` — 6 additional FlowSpec test cases
