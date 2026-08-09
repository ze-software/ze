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
	Payload []byte
}

// Transport sends and receives raw RSVP messages. Implementations are
// platform-specific; tests substitute a fake.
type Transport interface {
	// Send transmits an already-encoded RSVP message to dst.
	Send(dst netip.Addr, msg []byte) error
	// Recv returns the channel of received packets. The channel closes when
	// the transport is closed.
	Recv() <-chan Packet
	// Close releases the underlying socket and stops the receive loop.
	Close() error
}

// newTransport opens the platform raw-IP transport for protocol 46, binding to
// localAddr as the source. It is implemented in transport_linux.go and stubbed
// in transport_other.go.
func newTransport(localAddr netip.Addr) (Transport, error) {
	return openRawTransport(localAddr)
}
