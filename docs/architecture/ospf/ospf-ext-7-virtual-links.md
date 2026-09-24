# OSPF virtual links

A virtual link repairs a partitioned or non-contiguous backbone across a transit
area (RFC 2328 Section 15, RFC 5340). It is a synthetic backbone
point-to-point interface whose packets are ROUTED across the transit area, so
they are unicast with a TTL or hop limit above 1.

## Decisions

- **The V-bit goes in the TRANSIT area's Router-LSA** (RFC 2328 Appendix A.4.2
  and Section 16.3 transit capability). The Type-4 virtual-link RECORD goes in
  the BACKBONE Router-LSA.
  <!-- source: internal/plugins/ospf/virtual_link.go -- virtualLinkRuntime, configureVirtualLinks -->
- **A resolved virtual link creates a synthetic virtual interface** with
  point-to-point state, area 0 and MTU-ignore. Its network type remains
  `virtual` through Hello delivery and neighbor configuration, so Database
  Description summaries omit AS-external and Type-11 opaque LSAs.
  <!-- source: internal/plugins/ospf/transport_iface.go -- Transport -->
- **LSDB flooding uses the same routed sender as database exchange.** Updates,
  retransmissions and acknowledgments must reach the far endpoint over the
  physical transit interface. Sending them directly to the synthetic interface
  leaves the peer's database stale even after the adjacency reaches Full.
  AS-external and Type-11 opaque LSAs remain excluded from virtual-link flooding.
  <!-- source: internal/plugins/ospf/instance.go -- newEngineWithCodecAF -->
  <!-- source: internal/plugins/ospf/virtual_link.go -- virtualAwareSender.SendPacket -->
  <!-- source: internal/plugins/ospf/lsdb/flooding.go -- eligibleInterface -->
- **A received virtual-link packet is demultiplexed by source Router ID,
  backbone area and arrival on a REAL enrolled transit interface whose area is
  the virtual link's transit area. It is NOT demultiplexed by ifindex**, because
  it arrives on the physical transit interface. This is a fallback inside the
  area acceptance check.
  <!-- source: internal/plugins/ospf/virtual_link.go -- receiveTargetLocked, virtualLinkTargetLocked -->
- **A transit cost or next-hop change re-originates the backbone Router-LSA.**
  The existing adjacency stays up, and its interface reports the new cost.
  A path costing more than 65535 takes the virtual link down.
  <!-- source: internal/plugins/ospf/virtual_link.go -- onVirtualLinksResolved -->
  <!-- source: internal/plugins/ospf/neighbor/table.go -- databaseSummaryLocked -->
- **The IPv4 packet destination is the far endpoint's own address.** SPF
  derives it from that endpoint's reciprocal link on a shortest transit path.
  The first next hop is retained separately for forwarding across the transit area.
  <!-- source: internal/plugins/ospf/spf/transitarea.go -- virtualEndpointAddress -->
- **IPv6 endpoints advertise global LA host prefixes.** A configured transit
  area includes one local `/128` with the LA bit in its router-referencing
  Intra-Area-Prefix-LSA (RFC 5340 Section 4.4.3.9). Endpoint selection requires
  those address prefixes from both routers; a subnet address cannot substitute.
  Each completed SPF rechecks the addresses even when its transit path is
  unchanged. Losing either address takes the virtual interface down.
  <!-- source: internal/plugins/ospf/origination_v6.go -- v6OriginateSelf -->
  <!-- source: internal/plugins/ospf/virtuallink_v6.go -- v6ResolveVirtualEndpointLocked -->
- **A synthetic interface is never a FIB egress.** An OSPFv3 virtual first hop
  retains the global endpoint address without an `OnLink` flag, allowing recursive
  resolution through the transit area's physical route. The transit next hop
  itself retains its physical interface, even when another link uses the same
  link-local address.
  <!-- source: internal/plugins/ospf/afstrategy_v6.go -- v6NextHop -->
- **Reload stops removed or reconfigured virtual interfaces.** Unchanged
  configurations retain their interface and adjacency. Start and stop operations
  are serialized with SPF callbacks, and retained OSPFv3 links keep distinct
  Interface IDs when another link is added.
  <!-- source: internal/plugins/ospf/virtual_link.go -- configureVirtualLinks, stopVirtualInterfaces -->
- **Authentication is inherited from the transit area.** Sends sign on the
  transit egress and receives verify on the transit interface. There is no
  separate virtual-interface key registration.

## Constraints on callers

- The dispatcher order is fixed: the Instance ID drop runs FIRST, then the area
  acceptance check with the virtual fallback, then authentication, then the
  handler. The address-family bit gate, opaque delivery and IPsec are preserved
  by that order.
- The routed send is part of the engine Transport interface in both address
  families. The v4 backend ignores the source and raises the TTL. The v6 backend
  uses the global source and raises the hop limit.
  <!-- source: internal/plugins/ospf/spf/transitarea.go -- TransitCapability, resolveVirtualNeighbors -->

## Traps

- **The virtual demultiplex must not hijack a packet on a genuine backbone
  interface, and must not accept one on an unknown or non-enrolled ifindex.** It
  requires a real enrolled interface whose area is the virtual link's transit
  area. The base Type 1 to 7 demultiplex is the thing at risk here.
- **The IPv6 half was initially dead**: the routed send rejected a zero source
  and the endpoint resolution did not exist. A link-local placeholder is not an
  implementation.
- The Section 16.3 transit-area pass is improve-only. It never worsens a
  reachable route.
- Adding a slice to the immutable config snapshot pushed it past the pass-by-value
  size threshold. The threshold was raised, following the existing precedent,
  rather than converting the call sites to pointers.
