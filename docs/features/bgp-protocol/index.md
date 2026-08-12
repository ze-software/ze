# BGP Protocol

### Address Families

| Family | Config Name | AFI/SAFI | Encode | Decode | Route Config |
|--------|-------------|----------|--------|--------|--------------|
| IPv4 Unicast | `ipv4/unicast` | 1/1 | Yes | Yes | Yes |
| IPv6 Unicast | `ipv6/unicast` | 2/1 | Yes | Yes | Yes |
| IPv4 Multicast | `ipv4/multicast` | 1/2 | Yes | Yes | Yes |
| IPv6 Multicast | `ipv6/multicast` | 2/2 | Yes | Yes | Yes |
| IPv4 VPN | `ipv4/mpls-vpn` | 1/128 | Yes | Yes | Yes |
| IPv6 VPN | `ipv6/mpls-vpn` | 2/128 | Yes | Yes | Yes |
| IPv4 FlowSpec | `ipv4/flow` | 1/133 | Yes | Yes | Yes |
| IPv6 FlowSpec | `ipv6/flow` | 2/133 | Yes | Yes | Yes |
| IPv4 FlowSpec VPN | `ipv4/flow-vpn` | 1/134 | Yes | Yes | Yes |
| IPv6 FlowSpec VPN | `ipv6/flow-vpn` | 2/134 | Yes | Yes | Yes |
| IPv4 MPLS Label | `ipv4/mpls-label` | 1/4 | Yes | Yes | Yes |
| IPv6 MPLS Label | `ipv6/mpls-label` | 2/4 | Yes | Yes | Yes |
| L2VPN EVPN | `l2vpn/evpn` | 25/70 | Yes | Yes | Yes |
| L2VPN VPLS | `l2vpn/vpls` | 25/65 | Yes | Yes | Yes |
| BGP-LS | `bgp-ls/bgp-ls` | 16388/71 | No | Yes | No |
| BGP-LS VPN | `bgp-ls/bgp-ls-vpn` | 16388/72 | No | Yes | No |
| IPv4 MVPN | `ipv4/mvpn` | 1/5 | Yes | Yes | Partial |
| IPv6 MVPN | `ipv6/mvpn` | 2/5 | Yes | Yes | Partial |
| IPv4 RTC | `ipv4/rtc` | 1/132 | No | Yes | No |
| IPv4 MUP | `ipv4/mup` | 1/85 | Yes | Yes | Yes |
| IPv6 MUP | `ipv6/mup` | 2/85 | Yes | Yes | Yes |
| IPv4 SR-Policy | `ipv4/sr-policy` | 1/73 | Yes | Yes | Yes |
| IPv6 SR-Policy | `ipv6/sr-policy` | 2/73 | Yes | Yes | Yes |

<!-- source: internal/component/bgp/plugins/nlri/evpn/register.go -- EVPN family registration -->
<!-- source: internal/component/bgp/plugins/nlri/srpolicy/register.go -- SR-Policy family registration -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/register.go -- FlowSpec family registration -->
<!-- source: internal/component/bgp/plugins/nlri/vpn/register.go -- VPN family registration -->
<!-- source: internal/component/bgp/plugins/nlri/mup/register.go -- MUP family registration -->
<!-- source: internal/component/bgp/plugins/nlri/ls/register.go -- BGP-LS family registration -->
<!-- source: internal/component/bgp/plugins/nlri/labeled/register.go -- MPLS label family registration -->
<!-- source: internal/component/bgp/plugins/nlri/vpls/register.go -- VPLS family registration -->
<!-- source: internal/component/bgp/plugins/nlri/mvpn/register.go -- MVPN family registration -->
<!-- source: internal/component/bgp/plugins/nlri/rtc/register.go -- RTC family registration -->

### Capabilities

| Capability | Code | RFC | Description |
|------------|------|-----|-------------|
| Multiprotocol Extensions | 1 | RFC 4760 | Multi-protocol BGP (AFI/SAFI negotiation) |
| 4-byte ASN | 65 | RFC 6793 | 32-bit AS numbers |
| Route Refresh | 2 | RFC 2918 | Request full route re-advertisement |
| Enhanced Route Refresh | 70 | RFC 7313 | Bounded clear and re-send |
| ADD-PATH | 69 | RFC 7911 | Multiple paths per prefix |
| Extended Message | 6 | RFC 8654 | 65535-byte messages |
| Extended Next Hop | 5 | RFC 8950 | IPv6 next-hop for IPv4 NLRI |
| Graceful Restart | 64 | RFC 4724 | Session preservation across restarts (Restarting Speaker: R-bit via zefs marker on `ze signal restart`) |
| Long-Lived GR | 71 | RFC 9494 | Extended stale route retention with LLGR_STALE community and depreference |
| BGP Role | 9 | RFC 9234 | Peer relationship role |
| Hostname | 73 | RFC 8516 | FQDN capability |
| Software Version | 75 | draft | Software version advertisement |
| Link-Local Next Hop | 77 | RFC 2545 + draft | IPv6 link-local as next-hop |
| PATHS-LIMIT | 76 | draft-abraitis-idr-addpath-paths-limit | Per-family path count limit for ADD-PATH |

<!-- source: internal/core/bgp/capability/capability.go -- capability code constants -->
<!-- source: internal/core/bgp/capability/encoding.go -- EncodingCaps fields ASN4, ExtendedMessage, AddPathMode, ExtendedNextHop, PathsLimitSend, PathsLimitRecv -->
<!-- source: internal/core/bgp/capability/session.go -- SessionCaps fields RouteRefresh, EnhancedRouteRefresh, GracefulRestart -->
<!-- source: internal/component/bgp/plugins/role/register.go -- BGP Role capability plugin -->
<!-- source: internal/component/bgp/plugins/hostname/register.go -- Hostname capability plugin -->
<!-- source: internal/component/bgp/plugins/softver/register.go -- Software Version capability plugin -->
<!-- source: internal/component/bgp/plugins/llnh/register.go -- Link-Local NH capability plugin -->

### Multipath installation

When several BGP candidates tie under multipath selection, Ze carries the
winner and its equal-cost sibling next hops into the shared Loc-RIB. The system
RIB then emits one ECMP group to the active FIB backend. Membership-only
changes, such as one equal-cost peer disappearing, update the installed group
without requiring the winning peer to change.

`show rib` reports the primary `next-hop` and the additional `ecmp-paths`.
Together they are the complete installed next-hop set.

<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- SelectMultipath, mirrorToLocRIB -->
<!-- source: internal/core/rib/locrib/candidate.go -- Path.ECMP -->
<!-- source: internal/core/rib/locrib/manager.go -- siblingNextHops -->

### Long-Lived Graceful Restart readvertisement

RFC 9494 stale-route handling is applied per destination on both normal
forwarding and RIB readvertisement. An LLGR-capable peer receives the stale
route unchanged. A non-LLGR eBGP peer receives a withdrawal. A non-LLGR iBGP
peer receives the route with `NO_EXPORT` and `LOCAL_PREF=0`, allowing partial
deployments to retain reachability without preferring stale information.

Only the LLGR filter runs during stale readvertisement. Other export policy has
already run on the original announcement and is not applied twice.

<!-- source: internal/component/bgp/plugins/gr/gr_egress.go -- LLGREgressFilter -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- sendStaleReadvertise -->

### Path Attributes

| Attribute | Code | JSON Key | Description |
|-----------|------|----------|-------------|
| ORIGIN | 1 | `origin` | igp / egp / incomplete |
| AS_PATH | 2 | `as-path` | AS path segments |
| NEXT_HOP | 3 | `next-hop` | Next hop IP address |
| MED | 4 | `med` | Multi-Exit Discriminator |
| LOCAL_PREF | 5 | `local-preference` | Local preference |
| ATOMIC_AGGREGATE | 6 | `atomic-aggregate` | Atomic aggregate flag |
| AGGREGATOR | 7 | `aggregator` | Aggregator ASN:IP |
| COMMUNITY | 8 | `community` | Standard communities |
| ORIGINATOR_ID | 9 | `originator-id` | Route reflector originator |
| CLUSTER_LIST | 10 | `cluster-list` | Route reflector cluster list |
| MP_REACH_NLRI | 14 | none | Multiprotocol reachable NLRI |
| MP_UNREACH_NLRI | 15 | none | Multiprotocol unreachable NLRI |
| EXTENDED_COMMUNITY | 16 | `extended-community` | Extended communities |
| LARGE_COMMUNITY | 32 | `large-community` | Large communities (RFC 8092) |
| PREFIX_SID | 40 | `prefix-sid` | Segment Routing prefix SID |

<!-- source: internal/core/bgp/attribute/attribute.go -- attribute code constants -->
<!-- source: internal/core/bgp/attribute/origin.go -- ORIGIN -->
<!-- source: internal/core/bgp/attribute/aspath.go -- AS_PATH -->
<!-- source: internal/core/bgp/attribute/community.go -- Communities, ExtendedCommunities, LargeCommunities -->
