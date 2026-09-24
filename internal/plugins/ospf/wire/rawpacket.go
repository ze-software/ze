// Design: docs/architecture/ospf/ospf-af-unify.md -- RawPacket is the address-family-neutral received
// OSPF datagram handed from a transport (ospf/transport on IPv4, ospfv3/transport on
// IPv6) up to the shared engine. Both transports retain Dst/HopLimit from the IP
// header. IPv6 uses them for checksum verification and the link-local hop-limit check.
// This leaf imports nothing from the engine, so both transports can alias it without
// violating dependency direction.

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
