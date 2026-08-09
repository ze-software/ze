// Design: docs/architecture/ospf/ospf-af-unify.md -- RawPacket is the address-family-neutral received
// OSPF datagram handed from a transport (ospf/transport on IPv4, ospfv3/transport on
// IPv6) up to the shared engine. It is the superset of both: Dst/HopLimit are populated
// by the IPv6 transport (RFC 5340 binds the checksum to src/dst and requires Hop Limit 1)
// and left zero by the IPv4 transport. This leaf imports nothing from the engine, so both
// transports can alias it without violating dependency direction.

package wire

import "net/netip"

// RawPacket is one received OSPF datagram: the upper-layer payload plus the metadata the
// engine needs to dispatch and (for IPv6) verify it.
type RawPacket struct {
	IfIndex  int
	Src      netip.Addr
	Dst      netip.Addr
	HopLimit uint8
	Payload  []byte
}
