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
- **A resolved virtual link creates a synthetic interface** with network type
  virtual, area 0 and MTU-ignore. It sends through a routed transport and runs
  the normal neighbor state machine.
  <!-- source: internal/plugins/ospf/transport_iface.go -- Transport -->
- **A received virtual-link packet is demultiplexed by source Router ID,
  backbone area and arrival on a REAL enrolled transit interface whose area is
  the virtual link's transit area. It is NOT demultiplexed by ifindex**, because
  it arrives on the physical transit interface. This is a fallback inside the
  area acceptance check.
  <!-- source: internal/plugins/ospf/virtual_link.go -- receiveTargetLocked, virtualLinkTargetLocked -->
- **IPv6 virtual links resolve GLOBAL source and destination addresses from the
  transit area's Intra-Area-Prefix-LSAs** (RFC 5340 Section 2.9), not from the
  link-local next hop.
  <!-- source: internal/plugins/ospf/virtuallink_v6.go -- v6ResolveVirtualEndpointLocked -->
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
