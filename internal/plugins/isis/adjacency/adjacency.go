// Design: plan/learned/931-isis-5-adjacency.md -- IS-IS adjacency record and state.
// ISO/IEC 10589 section 8.2 (adjacency states).
//
// RFC: rfc/short/rfc5303.md -- P2P three-way adjacency (TLV 240), reported state
// RFC: rfc/short/rfc1195.md -- TLV 129/132 origination, IPv4 next-hop source
// RFC: rfc/short/rfc5308.md -- TLV 232/236 IPv6 next-hop source (isis-12)
//
// This package is the pure adjacency finite state machine and per-circuit
// neighbor table. It performs NO I/O: it imports the IS-IS types and the wire
// codec only for the parsed Hello fields the FSM consumes, never the transport.
// The circuit (sibling package) decodes a received Hello, fills a HelloInput,
// drives the FSM, and is the single writer of the table.

package adjacency

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// SNPALen is the length of a Subnetwork Point of Attachment (a 48-bit MAC),
// used as the LAN three-way echo key. Mirrors packet.SNPALen / transport.MACLen
// so the adjacency package does not depend on either for this small constant.
const SNPALen = 6

// SNPA is a neighbor's Subnetwork Point of Attachment (its source MAC on a
// LAN). It is comparable so it can be a map key and compared by ==.
type SNPA [SNPALen]byte

// State is the adjacency state (ISO/IEC 10589 section 8.2). The three states are
// Down, Initializing, and Up. A new adjacency starts Down; it advances to
// Initializing on the first valid Hello, and to Up once bidirectionality is
// proven (LAN: our SNPA echoed in the neighbor's TLV 6; P2P: RFC 5303 TLV 240
// reports Up and echoes our circuit, or the legacy implicit fall-back).
type State uint8

// Adjacency states (ISO/IEC 10589 section 8.2).
const (
	StateDown State = iota
	StateInitializing
	StateUp
)

// String renders the state as a stable lowercase token for CLI/JSON/events.
func (s State) String() string {
	switch s {
	case StateInitializing:
		return "initializing"
	case StateUp:
		return "up"
	default:
		return "down"
	}
}

// Level is the routing level an adjacency is formed at (1 or 2). It mirrors the
// PDU level; an L1L2 circuit may hold one adjacency per level to the same
// neighbor, so the level is part of the table key.
type Level uint8

// Adjacency levels.
const (
	Level1 Level = 1
	Level2 Level = 2
)

// String renders the level as the CLI token "l1"/"l2".
func (l Level) String() string {
	if l == Level2 {
		return "l2"
	}
	return "l1"
}

// Adjacency is one neighbor record on a circuit. It captures the FSM state, the
// neighbor identity and addresses, the area addresses (for the L1 match), the
// RFC 5303 reported P2P state, and the hold-timer expiry. The IPv4/IPv6
// interface addresses (from TLV 132/232) are stored here as the next-hop source
// the spec-isis-9 SPF consumes (Shared Contracts "Next-hop derivation for SPF").
type Adjacency struct {
	// SystemID is the neighbor's 6-octet System ID (the LAN table key).
	SystemID types.SystemID
	// SNPA is the neighbor's source MAC on a LAN (the three-way echo source).
	// Zero on a P2P circuit.
	SNPA SNPA
	// Level is the routing level this adjacency is formed at.
	Level Level
	// State is the current FSM state.
	State State
	// Areas are the neighbor's advertised area addresses (TLV 1), used for the
	// L1 area-address match.
	Areas []types.AreaID
	// IPv4 is the neighbor's IPv4 interface address (first TLV 132 entry),
	// stored as the SPF next-hop source. Invalid when the neighbor sent none.
	IPv4 netip.Addr
	// IPv6 is the neighbor's IPv6 interface address (first TLV 232 entry),
	// stored as the SPF next-hop source (isis-12 reads it). Invalid when absent.
	IPv6 netip.Addr
	// HoldTime is the neighbor's advertised holding time in seconds.
	HoldTime uint16
	// Priority is the neighbor's advertised DIS election priority (0..127) from
	// its LAN IIH (ISO/IEC 10589 clause 8.4.5). It is the candidate priority the
	// DIS election (isis-8) compares; 0 on a P2P adjacency (no DIS on P2P). The
	// LAN three-way identity (SNPA) breaks an equal-priority tie.
	Priority uint8
	// HoldExpiry is when the adjacency times out if no further Hello arrives. It
	// is reset on every accepted Hello.
	HoldExpiry time.Time
	// LastSeen is the time of the last accepted Hello from this neighbor.
	LastSeen time.Time

	// reportedState is the neighbor's RFC 5303 three-way state from its last
	// TLV 240 (P2P only). neighborSawUs reports whether the neighbor's TLV 240
	// echoed OUR System ID / circuit (proving it heard us). sawTLV240 records
	// whether the neighbor ever sent a TLV 240 at all, which selects the 3-way
	// path versus the legacy implicit fall-back (RFC 5303 sec 3.2).
	reportedState packet.AdjThreeWayState
	neighborSawUs bool
	sawTLV240     bool

	// deleteAt is when the record is purged after going Down. Zero while Up or
	// Initializing; set when the adjacency drops, to hold the record through the
	// grace period (absorbing transient flaps).
	deleteAt time.Time
}

// IsUp reports whether the adjacency is in the Up state.
func (a *Adjacency) IsUp() bool { return a.State == StateUp }
