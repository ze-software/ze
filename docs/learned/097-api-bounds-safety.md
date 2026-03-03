# 097 — API UPDATE Builder Bounds Safety

## Objective

Add maxSize enforcement to UpdateBuilder methods so that API-generated UPDATEs respect the negotiated message size limit (4096 standard, 65535 with RFC 8654 Extended Message).

## Decisions

- Single-route builders get `*WithMaxSize` variants that return error on overflow — cannot split atomic routes, so error is the only option.
- Multi-route builders get `*WithLimit` variants that split automatically across multiple UPDATEs.
- Caller provides `maxSize` — the builder does not know the peer's Extended Message state.
- `BuildGroupedUnicastWithLimit` already existed; only `BuildMVPNWithLimit` was needed for multi-route.

## Patterns

- FlowSpec is atomic per RFC 5575: single rule cannot be split even if too large — error not split.
- Two conventions: `*WithMaxSize` for atomic (single route, error on overflow) vs `*WithLimit` for batched (slice of routes, splits).

## Gotchas

None.

## Files

- `internal/bgp/message/update_build.go` — added `BuildFlowSpecWithMaxSize`, `BuildMVPNWithLimit`, and `*WithMaxSize` variants for all single-route builders
- `internal/bgp/message/update_build_test.go` — 16 new bound tests
