# 757: Typed RouteResult for Plugin Responses

## Context
`DispatchNLRIGroups` returned a `map[string]any` with "announced", "withdrawn", and "warnings" keys. `extractUpdateRouteOutput` had a `mapUint32` helper with a type switch between `int` (DirectBridge in-process) and `float64` (JSON round-trip via socket), papering over the transport divergence.

## Decision
Introduced `plugin.RouteResult` struct with typed fields (`Announced uint32`, `Withdrawn uint32`, `Warnings []string`). `DispatchNLRIGroups` returns `*plugin.RouteResult` as `resp.Data`. Extraction uses a single type assertion.

Key choices:
- **Struct over map**: eliminates the `int`/`float64` ambiguity entirely. Both DirectBridge and JSON paths produce the same concrete type.
- **`uint32` over `int`**: route counts are always positive and bounded by NLRI count. Explicit `uint32` with `//nolint:gosec` annotation for the narrowing.
- **Deleted `mapUint32`**: dead code removal, not deprecation. No other callers existed.

## Consequences
- 4 files changed, net -10 lines. Simpler extraction logic.
- Tests assert on `result.Announced` directly instead of probing map keys.
- Pattern is reusable for other plugin response types that currently use `map[string]any`.

## Gotchas
- The `RouteResult` struct lives in `internal/component/plugin/types.go`, not in the bgp package, because the extraction happens in `plugin/server/dispatch.go`.

## Files

None recorded.
