// Design: docs/architecture/ospf/bfd-client.md -- the BFD engine's IPv6 single-hop
// transport (IPV6_UNICAST_HOPS=255 TX, IPV6_RECVHOPLIMIT cmsg GTSM RX). The engine carries an
// IPv6 single-hop session end-to-end; OSPF adds NO transport code.
// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- OSPFv3 is our OSPF; the BFD GTSM-255
// single-hop unicast session is independent of base OSPFv3's Hop-Limit-1 multicast.
// RFC: rfc/short/rfc5881.md -- BFD for IPv4/IPv6 single hop.
//
// The IPv6 (OSPFv3) BFD request builder. The shared client lifecycle lives in bfd_client.go;
// the ONLY per-address-family divergence is this builder (the link-local pair) versus the
// IPv4 builder (the on-subnet IPv4 pair), selected by codec.IsV6() in bfdRequestForFamily.
package ospf

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// bfdRequestForNeighborV6 builds the IPv6 (OSPFv3) single-hop request. RFC 5881 sec 6: the
// link-local source/destination pair -- Peer is the neighbor's IPv6 link-local, Local the
// interface's IPv6 link-local source (from the v3 transport / iface component, never the
// [4]byte IPv4 InterfaceAddress). RFC 5881 sec 2: this session is DISTINCT from any
// co-resident IPv4 (OSPFv2) session on the same link -- the differing address pair yields a
// different api.Key, so the two refcount independently on a dual-stack link.
//
// The engine enforces GTSM Hop-Limit 255 for Mode SingleHop; that is independent of base
// OSPFv3's Hop-Limit-1 multicast Hellos (learned 970): BFD is a separate single-hop unicast
// session. A8/R-9: Local may be the zero Addr if the interface link-local has not resolved
// (DAD); the engine then falls back to kernel source selection. In practice DAD completes
// long before a neighbor reaches Full (Hellos have already flowed).
func bfdRequestForNeighborV6(neighborLL, interfaceLL netip.Addr, ifname string, cfg bfdInterfaceConfig) api.SessionRequest {
	return bfdRequestForNeighbor(neighborLL, interfaceLL, ifname, cfg)
}
