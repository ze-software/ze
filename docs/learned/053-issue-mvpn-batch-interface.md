# 053 — MVPN Route Grouping Missing Attribute Key

## Objective

Fix a VPN isolation bug: `groupMVPNRoutesByNextHop` grouped routes solely by next-hop, so routes with different Route Targets (ExtCommunityBytes) were batched into one UPDATE, causing the second route's RT to be silently overwritten by the first.

## Decisions

- Added `mvpnRouteGroupKey()` matching the established unicast `routeGroupKey` pattern (16+ fields)
- Excluded IsIPv6 (pre-separated before grouping), RouteType (per-NLRI, multiple allowed in one UPDATE), and RD (per-NLRI) from the key — these are NOT shared attributes
- Deleted `groupMVPNRoutesByNextHop` entirely (no-layering rule: replace, don't keep both)
- Added OriginatorID and ClusterList to MVPNRoute struct to support RFC 4456 reflector attrs in grouping

## Patterns

- `routes[0]` for shared attributes in `BuildMVPN` is correct only when the caller guarantees identical attributes — the fix is in the grouping function, not the builder

## Gotchas

- VPN isolation failure: routes for different customers with same next-hop but different Route Targets were sent with the wrong RT — a correctness bug, not just performance
- Other multi-route builders (FlowSpec, MUP, VPLS, VPN, LabeledUnicast) are single-route APIs — no grouping bug possible there; only MVPN used the buggy slice pattern

## Files

- `internal/reactor/peersettings.go` — MVPNRoute struct (added OriginatorID, ClusterList)
- `internal/reactor/peer.go` — `mvpnRouteGroupKey()`, `groupMVPNRoutesByKey()`, deleted old function
- `internal/reactor/peer_test.go` — grouping tests
