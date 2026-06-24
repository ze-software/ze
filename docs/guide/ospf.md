# OSPF

Ze includes an experimental native OSPF engine under the `ospf` config root. The same engine drives OSPFv2 for IPv4 and OSPFv3 for the `address-family ipv6` subsection: interface and neighbor state, LSDB flooding, SPF, ABR/NSSA policy, redistribution, and route installation are shared; the wire codec, raw transport, and prefix strategy are address-family specific.
<!-- source: internal/plugins/ospf/register.go -- registerOSPF -->
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- ospf config root -->
<!-- source: internal/plugins/ospf/codec_v6.go -- v6Codec -->
<!-- source: internal/plugins/ospf/afstrategy_v6.go -- v6Strategy -->
<!-- source: internal/plugins/ospf/v3/transport/transport.go -- Transport -->

SPF builds one graph per area from Router-LSAs and Network-LSAs, enforces the RFC 2328 two-way check, derives next-hops from Router-LSA link data, merges equal-cost next-hops, and inserts one `locrib.Path` per next-hop with OSPF admin distance 110. The kernel FIB path is Loc-RIB -> sysrib -> fibkernel, not redistribution events.
<!-- source: internal/plugins/ospf/spf/graph.go -- BuildGraph -->
<!-- source: internal/plugins/ospf/spf/spf.go -- Compute -->
<!-- source: internal/plugins/ospf/spf/route.go -- BuildRoutes -->
<!-- source: internal/plugins/ospf/spf/install.go -- Installer Apply -->

ABR support treats a router as area-border only when it is active in the backbone and at least one other area. It originates Type 3 Summary-Network LSAs and Type 4 Summary-ASBR LSAs, applies configured area ranges with optional cost or `not-advertise`, withdraws stale self-originated summaries, accepts backbone summaries when calculating on an ABR, and computes inter-area costs as cost-to-advertising-ABR plus the summary metric.
<!-- source: internal/plugins/ospf/spf/interarea.go -- IsABR and ComputeInterArea -->
<!-- source: internal/plugins/ospf/spf/summary.go -- OriginateSummaries -->
<!-- source: internal/plugins/ospf/lsdb/origination.go -- OriginateSummary and FlushStaleSummaryLSAs -->

## AS-External routes and redistribution

A router becomes an AS Boundary Router (ASBR) when it originates external LSAs, either by redistributing routes from another protocol or through `default-information originate`. OSPFv2 uses Type 5 AS-External-LSAs; OSPFv3 uses AS-External-LSAs (`0x4005`) in normal areas and NSSA-LSAs (`0x2007`) in attached NSSA areas. The ASBR sets the E-bit in its Router-LSA; the ABR and ASBR roles are independent. AS-scoped Type 5 / `0x4005` LSAs live in the AS-wide store, separate from per-area LSDBs.
<!-- source: internal/plugins/ospf/lsdb/origination.go -- OriginateExternal, PurgeExternal -->
<!-- source: internal/plugins/ospf/origination_v6_external.go -- v6InjectExternal, v6OriginateExternalLSA -->
<!-- source: internal/plugins/ospf/origination_v6_nssa.go -- v6OriginateNSSALSA -->

Redistribution uses the shared `redistribute` orchestrator. `redistribute { destination ospf { import connected static bgp } }` injects each accepted route as an external LSA in the configured address family; `redistribute { destination bgp { import ospf } }` exports OSPF-selected routes back out. OSPF self-import is a runtime no-op (loop prevention: the import source `ospf` equals the importing protocol). The per-source metric, metric-type (E1/E2), and route tag come from the `ospf` container's `redistribute` list; an unenrolled source falls back to metric 20, type-2.
<!-- source: internal/plugins/ospf/redistribute/consumer.go -- Consumer InjectRoute, WithdrawRoute -->
<!-- source: internal/plugins/ospf/redistribute/source.go -- Source OnSPFChange exports OSPF routes -->
<!-- source: internal/plugins/ospf/redist_wiring.go -- engine InjectExternal, externalParams -->

Received external LSAs are resolved by the external SPF stage, which runs after the intra-area and inter-area route tables are built. Each external is resolved against its ASBR (or a non-zero forwarding address, re-resolved through the route table; unreachable externals are skipped). Type 1 (E1) cost is the distance to the forwarding target plus the advertised metric; type 2 (E2) cost is the advertised metric only, tie-broken by the forwarding distance. A type 1 external always wins over a type 2 regardless of cost, and any external ranks below an intra-area or inter-area route for the same prefix. The winning path installs as one `locrib.Path` with admin distance 110.
<!-- source: internal/plugins/ospf/spf/external.go -- ComputeExternal, ComputeExternalWith, betterExternal -->
<!-- source: internal/plugins/ospf/afstrategy_v6.go -- v6ExternalReader -->

`default-information originate` advertises a Type 5 default (`0.0.0.0/0`). With `always` it originates unconditionally; otherwise it originates only while a non-OSPF default route exists in the Loc-RIB (OSPF's own default does not satisfy the condition). The engine re-evaluates the condition at config-apply and live on Loc-RIB default-route changes, withdrawing the Type 5 when the condition lapses.
<!-- source: internal/plugins/ospf/default.go -- applyDefaultInformation, hasNonOSPFDefault, watchDefaultRoute -->

## Stub and NSSA areas

A stub area (`area-type stub`) carries no AS-External information: the E-bit is clear in every Hello and Router-LSA originated within it (a Hello whose E-bit does not match is dropped and no adjacency forms), Type 4 ASBR-Summary and Type 5 AS-External LSAs are neither accepted from nor flooded into the area, and each attached ABR injects a single Type 3 default (`0.0.0.0/0`; OSPFv3 injects the equivalent `::/0` as an Inter-Area-Prefix-LSA) at the configured `default-cost`. A totally-stubby area (`no-summary true`) additionally suppresses every inter-area Type 3 summary except that default, so a spoke holds only intra-area routes plus one default.
<!-- source: internal/plugins/ospf/spf/area_type.go -- applyAreaTypePolicy -->
<!-- source: internal/plugins/ospf/origination_v6_stub.go -- v6ApplyAreaTypePolicy -->
<!-- source: internal/plugins/ospf/lsdb/flooding.go -- shouldDropByArea, eligibleInterface -->
<!-- source: internal/plugins/ospf/iface/iface.go -- validateHelloLocked E/N-bit match -->

An NSSA (`area-type nssa`, RFC 3101) is stub-like but permits local external redistribution. The N-bit must match between neighbours (the E-bit stays clear as in a stub). An NSSA ASBR originates Type 7/NSSA LSAs for the routes it redistributes; in OSPFv3 the P-bit lives in the prefix options and a non-zero IPv6 forwarding address is carried in the external body. A router that cannot inject a Type 5 directly (no normal-area attachment) sets the P-bit so its Type 7 is translatable; one that can originate a Type 5 directly clears it. The ABR may also originate a Type 7 default (`nssa { default-originate true }`).
<!-- source: internal/plugins/ospf/lsdb/nssa.go -- OriginateNSSA, PurgeNSSA -->
<!-- source: internal/plugins/ospf/origination_v6_nssa.go -- v6OriginateNSSALSA -->
<!-- source: internal/plugins/ospf/nssa.go -- applyNSSADefaults -->

Among the ABRs attached to an NSSA, exactly one is elected the Type 7 to Type 5 translator (RFC 3101 §3.5): the translator-candidate ABR with the highest Router ID. Each ABR whose role is not `never` advertises the Nt-bit in its Router-LSA to stand as a candidate; a `never` ABR clears the Nt-bit and is excluded from the election, so it cannot wedge translation off for a willing lower-Router-ID candidate. The role is configurable per area (`nssa { translate-role candidate|always|never }`), sticky across a `stability-interval`. The elected translator re-originates each P=1, non-zero-FA Type 7 as a Type 5 onto the backbone (P cleared, Advertising Router set to the translator, forwarding address / metric / tag preserved), counted by `ze_ospf_nssa_translations_total{area}`. A non-elected ABR does not translate, so no duplicate Type 5 reaches the backbone. When the same external prefix is known via a Type 7 (P=1), a Type 5, and a Type 7 (P=0), the external route computation prefers them in that order (RFC 3101 §2.5) ahead of the §16.4 cost.
<!-- source: internal/plugins/ospf/nssa.go -- electNSSATranslator, translateNSSA, translatorEffective -->
<!-- source: internal/plugins/ospf/spf/external_nssa.go -- RFC 3101 sec 2.5 source preference -->
<!-- source: internal/plugins/ospf/spf/external.go -- ComputeExternal Type 7 candidates, betterExternal -->

## OSPFv3 Link-LSAs

OSPFv3 Link-LSAs (`0x0008`) are stored in a link-scoped LSDB keyed by the receiving interface, not in the area or AS-wide stores. Ze originates one self Link-LSA per active OSPFv3 interface, carrying the interface link-local address and routable IPv6 prefixes, floods it only on that link, and releases the link store when the interface is removed. During database exchange, link-scoped LSAs participate in DD summaries and LS Request lookup using the interface context.
<!-- source: internal/plugins/ospf/lsdb/link_scope.go -- installLink, LookupLinkLSA, ReleaseLink -->
<!-- source: internal/plugins/ospf/origination_v6_link.go -- v6OriginateLinkLSA -->

## Authentication

OSPFv2 packets can be authenticated per interface with a key chain (RFC 2328 Appendix D, RFC 5709, RFC 7474). Four schemes are supported, selected by the key algorithm and the chain's `extended-sequence` flag: AuType 0 (none), AuType 1 (`simple` 8-byte cleartext password), AuType 2 (`md5` keyed-MD5 or HMAC-SHA-1/256/384/512), and AuType 3 (the same HMAC-SHA algorithms with a 64-bit extended cryptographic sequence number). Every outgoing packet is signed and every incoming packet is verified before it reaches the ISM/NSM/LSDB; a packet that fails is dropped before any protocol processing and increments `ze_ospf_auth_failures_total{interface,reason}`.
<!-- source: internal/plugins/ospf/packet/auth_verify.go -- Sign, Verify -->
<!-- source: internal/plugins/ospf/auth_wiring.go -- signPacket (TX), verifyPacket (RX) -->

For cryptographic auth the OSPF common-header Checksum is zero (the appended digest provides integrity) and the OSPF Packet Length covers only the header and body; the digest (and, for AuType 3, a leading 64-bit sequence number) is appended after the body and counted only in the IP length. All digest and password comparisons are constant-time. The cryptographic sequence number is non-decreasing per neighbour, key-id, and packet type, so a replayed packet is rejected; the send counter is seeded from a monotonic clock so it does not regress across a restart (a peer enforcing a strictly-increasing sequence keeps the adjacency, RFC 7474). For AuType 3 the IP source address is bound into the digest (RFC 7474 §5 initialises the first four octets of Apad to the source address) so a spoofed source fails verification.
<!-- source: internal/plugins/ospf/auth_keystore.go -- verify replay check, signKey sequence -->

Keys are organised as named chains for hitless rotation: a chain holds multiple keys, the first is used to sign, and any chain key is accepted on receive during an overlap window. A chain is bound per interface, or an interface set to `authentication { mode inherit }` uses the area-level default chain. Secrets are stored `$9$`-encoded and never appear in plaintext in `show configuration` or backups.
<!-- source: internal/plugins/ospf/auth_keystore.go -- configure, inherit resolution, decodeSecret -->

Operational state is available through the `show ip ospf` command tree:

```text
show ip ospf
show ip ospf neighbor
show ip ospf interface
show ip ospf database [router|network|summary|asbr-summary|external|nssa-external]
show ip ospf route
show ip ospf spf
show ip ospf border-routers
```

`show ip ospf` is the process summary (router-id, ABR/ASBR status, areas, and the active stub-router / max-metric state). `show ip ospf database` lists every LSA; the per-type subviews filter to one LS Type (1/2/3/4/5/7). `show ip ospf route` reports area, prefix, metric, route type, origin router, and next-hop set. `show ip ospf spf` reports per-area last run, duration, node count, pending state, and current throttle delay. `show ip ospf border-routers` reports reachable ABRs and ASBRs with their area, metric, and next-hop set.
<!-- source: internal/plugins/ospf/register.go -- OnExecuteCommand show ip ospf route/spf/border-routers -->
<!-- source: internal/plugins/ospf/cmd_show.go -- ze-show:ospf-* RPC proxies -->
<!-- source: internal/plugins/ospf/show_summary.go -- processSummary -->

The runtime can be reset without reconfiguring via `clear ip ospf process` (tear down adjacencies and re-run SPF), `clear ip ospf neighbor` (re-form adjacencies), and `clear ip ospf counters` (reset the SPF-run log). The neighbor and database views are also available in the web UI at `/ospf` and `/ospf/database`, with live updates over SSE.
<!-- source: internal/plugins/ospf/clear.go -- clearProcess/clearNeighbors/clearCounters -->
<!-- source: internal/component/web/handler_ospf.go -- OSPF neighbor/database web views -->
<!-- source: internal/plugins/ospf/spf/computer.go -- SPFSnapshot -->
<!-- source: internal/plugins/ospf/spf/route.go -- Snapshot -->
<!-- source: internal/plugins/ospf/spf/interarea.go -- BorderRouterSnapshot -->
