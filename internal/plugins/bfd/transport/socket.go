// Design: rfc/short/rfc5881.md -- single-hop UDP encapsulation
// Design: rfc/short/rfc5883.md -- multi-hop UDP encapsulation
//
// Package transport provides UDP I/O for BFD Control packets and an
// in-memory loopback implementation used by engine-level tests.
//
// The Transport interface lets the engine treat single-hop, multi-hop,
// and loopback paths uniformly. Each Transport reads incoming packets
// from a single goroutine that pushes onto a buffered channel; the
// engine's express loop drains the channel and dispatches by
// discriminator.
//
// Sockets and goroutines are owned by the Transport. Stop drains and
// releases them.
package transport

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
)

// Inbound is a single received BFD packet handed to the engine.
//
// Bytes points into a per-Transport pool buffer that is recycled when
// the engine releases the Inbound. Callers MUST NOT retain Bytes
// across the engine's processing of this Inbound.
type Inbound struct {
	// From is the source IP address as observed on the wire. The port
	// is intentionally absent: BFD demultiplexing uses the address only.
	From netip.Addr

	// VRF is the routing instance the packet arrived in.
	VRF string

	// Interface is the ingress interface name (single-hop only). Empty
	// for multi-hop.
	Interface string

	// Mode records whether the packet arrived on the single-hop or
	// multi-hop port.
	Mode api.HopMode

	// TTL is the IP TTL/hop-limit at receive. Used by the engine to
	// enforce GTSM (single-hop) or min-TTL (multi-hop). Zero means
	// the transport could not extract the TTL.
	TTL uint8

	// Bytes holds the wire-encoded BFD Control packet, including any
	// authentication section.
	Bytes []byte

	// release returns Bytes to the transport pool. The engine MUST
	// call Release exactly once.
	release func()
}

// Release returns the Inbound's buffer to its transport pool. The engine
// MUST call Release exactly once after parsing.
func (in *Inbound) Release() {
	if in.release != nil {
		in.release()
		in.release = nil
	}
}

// Outbound is a single BFD packet the engine wants to send.
type Outbound struct {
	To        netip.Addr
	VRF       string
	Interface string
	Mode      api.HopMode
	Bytes     []byte
}

// Transport is the wire layer between the engine and the network. It
// is satisfied by the UDP transports in this package and by the
// in-memory loopback used in tests.
//
// Caller MUST call Start exactly once before sending or receiving and
// MUST call Stop exactly once when finished. Stop blocks until the
// receive goroutine has exited and any allocated sockets are closed.
type Transport interface {
	Start() error
	Stop() error
	Send(Outbound) error
	RX() <-chan Inbound
}
