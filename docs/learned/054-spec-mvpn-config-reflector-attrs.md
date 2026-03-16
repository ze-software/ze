# 054 — MVPN Config Route Reflector Attributes

## Objective

Expose the OriginatorID and ClusterList fields added to MVPNRoute (spec-053) at the config layer, so users can configure RFC 4456 route reflector attributes for MVPN routes.

## Decisions

- Mechanical refactor following the existing VPLSRouteConfig pattern exactly
- IPv6 addresses in OriginatorID/ClusterList are silently ignored (matches VPLSRoute convention for consistency)
- FlowSpec/MUP config intentionally lacks these fields — only their Build* builders use them, not config-originated routes

## Patterns

- Config uses string fields for IP addresses (`OriginatorID string`, `ClusterList string`)
- ClusterList is space-separated IP string in config, parsed to `[]uint32`
- Conversion follows VPLSRoute: `ip.As4()` → pack to uint32

## Gotchas

- None.

## Files

- `internal/component/config/bgp.go` — MVPNRouteConfig (added OriginatorID, ClusterList string fields)
- `internal/component/config/loader.go` — `convertMVPNRoute()` (added parsing following VPLSRoute pattern)
- `internal/component/config/loader_test.go` — 4 new conversion tests
