# RFC 7999 BLACKHOLE

## Meta

| Field | Value |
|-------|-------|
| Name | RFC 7999 BLACKHOLE |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/plugins/rib |
| Real-path tests | test/plugin/bgp-blackhole-honor.ci, test/plugin/community-blackhole-noexport.ci, test/plugin/fib-blackhole.ci |
| Interop | bgp/bgp-rfc7999-blackhole-frr |
| RFCs | rfc7999 |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; the blackhole communities and prefixes leaves are augmented as the anchors name |
| Defect review | 2026-10-07: no plan/immediate spec found; rfc/short/rfc7999.md reads Partial while the audit counted its requirements covered |
| Extra criteria | supported: a honored route becomes a VPP drop through the VPP backend = test/vpp/vpp-blackhole-drop.ci |

## Description

Per-session honoring of the BLACKHOLE community, off by default. The peer names the community it uses (`blackhole communities`, the well-known 65535:666 or its own) and the prefixes it is authorized to blackhole within (`blackhole prefixes`); RFC 7999 Section 3.3 states both conditions, so a peer with a community and no covering prefix blackholes nothing. A honored route becomes a discard route directly: a Linux FIB blackhole or a VPP drop, with no next-hop to allocate and no static route to pre-create. The same list governs the send side, which is RFC 7999 Section 3.1's requirement that the two networks agree before the community is advertised: `send bgp <sel> blackhole` and `send bgp <sel> unicast <prefix> community 65535:666` reach only the sessions that agreed, and a session that named its own value alone is left out rather than sent the prefix untagged. The leaves are augmented at the bgp, group and peer levels. <!-- source: internal/component/bgp/plugins/rib/yang/ze-rib.yang -- grouping blackhole-honor-fields --> <!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- blackholeRouteTypeForBest -->
