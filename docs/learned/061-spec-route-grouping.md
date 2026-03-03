# 061 — Route Grouping for Efficient UPDATE Packing

## Objective

Reduce UPDATE count from O(routes) to O(routes/capacity) when replaying adj-rib-out by grouping routes with identical attributes into shared UPDATEs, reusing the existing `BuildGroupedUnicastWithLimit` pattern.

## Decisions

- Reuse existing `BuildGroupedUnicastWithLimit` for IPv4 unicast; add new `BuildGroupedMPReachWithLimit` for MP families (IPv6, VPN)
- Group key = `attrHash + family`; PathID excluded from hash — routes with different path IDs but same attrs CAN share one UPDATE
- `Attributes.Hash()` uses FNV-64a over all shared attribute fields — no cryptographic strength needed, just collision resistance for grouping
- For MP families: NEXT_HOP goes in MP_REACH_NLRI, NOT as a separate attribute (this is the RFC 4760 requirement — omitting it from base attrs is mandatory)
- `group-update false` config option allows one-route-per-UPDATE legacy mode
- Re-encoding from parsed attributes (not wire bytes) is acceptable — grouping is already the slow path (can't use zero-copy)

## Patterns

- `rib.GroupByAttributesTwoLevel()` already existed and was used rather than writing a new grouper
- `toRIBRouteUnicastParams()` adapter converts rib.Route → UnicastParams for the existing builder

## Gotchas

- Routes with different families MUST NOT be grouped — IPv4 vs IPv6 use different encoding (IPv4 in UPDATE body, IPv6 in MP_REACH_NLRI)
- MP_REACH_NLRI extended flag (0x90 vs 0x80) depends on value length: >255 bytes requires extended length encoding

## Files

- `internal/reactor/reactor.go` — `sendRoutesWithLimit()`, `groupRoutesByAttributes()`, `toRIBRouteUnicastParams()`
- `internal/bgp/message/update_build.go` — `BuildGroupedMPReachWithLimit()`, `buildMPReachUpdate()`, `packGroupedAttributesBase()`
- `internal/bgp/attribute/attributes.go` — `Hash()`
