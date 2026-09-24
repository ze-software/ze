// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP-TE raw IP transport (protocol 46)
// Related: build.go -- produces the message bytes Send transmits
// Related: wire.go -- DecodeMessage parses received payloads
//
// RFC 2205 Section 3.1: RSVP runs directly over IP as protocol 46 (not TCP or
// UDP). The transport is a thin seam over a raw IP socket so the signaling layer
// can be unit-tested against a fake without opening a privileged socket. The
// platform socket lives in transport_linux.go; transport_other.go stubs it.
package rsvpte

import (
	"net/netip"
)

// Packet is one received RSVP datagram with its IP source address.
type Packet struct {
	Src     netip.Addr
	Dst     netip.Addr
	IfIndex int
	TTL     uint8
	Payload []byte
}

// PathRoute separates the protocol destination/source from the immediate ERO
// next hop. TableID selects a particular established MPLS bypass, never a
// shared route to its endpoint. Zero sends the ordinary unlabelled PATH.
// Lookup is the native route's lookup destination; IfIndex pins its output link.
type PathRoute struct {
	Destination netip.Addr
	Source      netip.Addr
	NextHop     netip.Addr
	TableID     uint32
	IfIndex     int
	Lookup      netip.Addr
}

// RouteInfo is a native route toward one member of an ERO abstract node.
// NextHop is the adjacent forwarding address, not a remote route destination.
// MTU is the outgoing frame payload budget, before MPLS labels are subtracted.
type RouteInfo struct {
	NextHop netip.Addr
	IfIndex int
	Lookup  netip.Addr
	MTU     uint32
}

// InterfaceAddress distinguishes address ownership from usable source links.
// A down interface still owns its addresses but cannot supply a repair sender.
type InterfaceAddress struct {
	Address netip.Addr
	IfIndex int
	Up      bool
	MTU     uint32
}

// Transport sends and receives raw RSVP messages. Implementations are
// platform-specific; tests substitute a fake.
type Transport interface {
	// Send transmits a reply from the selected outgoing interface. Implementations
	// set RSVP_HOP to that interface without modifying the supplied message.
	Send(dst netip.Addr, msg []byte) error
	// SendPath carries PATH/PathTear/ResvConf to the selected hop while retaining
	// the protocol destination, sender source and Router Alert in the IP header.
	SendPath(route PathRoute, msg []byte) error
	// LocalAddresses returns an owned snapshot of IPv4 addresses assigned in
	// this transport's network namespace, not configured link prefixes.
	LocalAddresses() ([]InterfaceAddress, error)
	// ResolveRoute chooses a native next hop toward an abstract node. The
	// session destination is preferred when it belongs to that node.
	ResolveRoute(target netip.Prefix, destination netip.Addr, tableID uint32) (RouteInfo, error)
	// Recv returns the channel of received packets. The channel closes when
	// the transport is closed.
	Recv() <-chan Packet
	// Close releases the underlying socket and stops the receive loop.
	Close() error
}

// newTransport opens the platform raw-IP transport for protocol 46 and verifies
// that localAddr can be bound. Outgoing interfaces supply reply source addresses;
// PATH preserves its sender identity independently.
func newTransport(localAddr netip.Addr) (Transport, error) {
	return openRawTransport(localAddr)
}
