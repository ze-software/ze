// VALIDATES: RFC 5303 sec 3.1 -- Ze includes the Adjacency Three-Way State
// field in every P2P Three-Way Adjacency option (TLV 240) it originates, in
// both the full (with-neighbor) and the minimal (no-neighbor) forms.
// PREVENTS: emitting a TLV 240 that omits the mandatory state field, which a
// three-way-capable neighbor requires to run the RFC 5303 handshake.
package circuit

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// RFC requirement: RFC5303-3.1-4 positive -- "Any system that supports this
// mechanism MUST include the Adjacency Three-Way State field in this option"
// (RFC 5303 sec 3.1). threeWayTLV (internal/plugins/isis/circuit/hello.go:160)
// writes the state as the LEADING octet of every TLV 240 it builds. For each of
// the three adjacency states, and in the full 15-octet form (with the neighbor
// echo), the emitted option decodes back to exactly that state -- the field is
// present and carries the reported value.
func TestISISP2PThreeWayStateFieldIncluded(t *testing.T) {
	c := &Circuit{localCircuitID: 7}
	nbr := types.SystemID{0, 0, 0, 0, 0, 9}
	for _, state := range []packet.AdjThreeWayState{
		packet.AdjThreeWayUp, packet.AdjThreeWayInitializing, packet.AdjThreeWayDown,
	} {
		tlv := c.threeWayTLV(state, nbr, true)
		if tlv.Type != packet.TLVP2PThreeWay {
			t.Fatalf("state %d: TLV type = %d, want %d", state, tlv.Type, packet.TLVP2PThreeWay)
		}
		if len(tlv.Value) == 0 {
			t.Fatalf("state %d: TLV 240 value is empty -- the mandatory state field is missing", state)
		}
		dec, err := packet.DecodeP2PThreeWayTLV(tlv.Value)
		if err != nil {
			t.Fatalf("state %d: decode TLV 240: %v", state, err)
		}
		if !dec.HasNeighbor {
			t.Fatalf("state %d: expected the full 15-octet form (neighbor echo present)", state)
		}
		if dec.State != state {
			t.Fatalf("state %d: decoded state field = %d, want %d", state, dec.State, state)
		}
	}
}

// RFC requirement: RFC5303-3.1-4 negative -- the Adjacency Three-Way State field
// is MANDATORY in the option, not merely present alongside the OPTIONAL neighbor
// echo: even in the minimal no-neighbor form (5 octets, no System ID echo) the
// leading state octet is still emitted. Ze never originates a TLV 240 that drops
// the state field when the optional neighbor fields are absent (RFC 5303 sec 3.1).
func TestISISP2PThreeWayStateFieldPresentWithoutNeighbor(t *testing.T) {
	c := &Circuit{localCircuitID: 7}
	tlv := c.threeWayTLV(packet.AdjThreeWayDown, types.SystemID{}, false)

	dec, err := packet.DecodeP2PThreeWayTLV(tlv.Value)
	if err != nil {
		t.Fatalf("decode minimal TLV 240: %v", err)
	}
	if dec.HasNeighbor {
		t.Fatal("expected the minimal no-neighbor form, but the neighbor echo is present")
	}
	if dec.State != packet.AdjThreeWayDown {
		t.Fatalf("minimal form dropped or corrupted the mandatory state field: got %d, want Down (%d)", dec.State, packet.AdjThreeWayDown)
	}
}

// p2pHelloTLV240 decodes a built P2P IIH and returns its TLV 240 (P2P Three-Way
// Adjacency) option, reporting whether the option was present at all.
func p2pHelloTLV240(t *testing.T, pdu []byte) (packet.P2PThreeWayTLV, bool) {
	t.Helper()
	p, err := packet.DecodePDU(pdu)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	if p.P2PHello == nil {
		t.Fatal("decoded PDU is not a point-to-point IIH")
	}
	for _, tv := range p.P2PHello.TLVs {
		if tv.Type == packet.TLVP2PThreeWay {
			dec, err := packet.DecodeP2PThreeWayTLV(tv.Value)
			if err != nil {
				t.Fatalf("DecodeP2PThreeWayTLV: %v", err)
			}
			return dec, true
		}
	}
	return packet.P2PThreeWayTLV{}, false
}

// lanHelloHasTLV240 decodes a built LAN IIH and reports whether it carries a
// TLV 240 (which it never should: the option is point-to-point only).
func lanHelloHasTLV240(t *testing.T, pdu []byte) bool {
	t.Helper()
	p, err := packet.DecodePDU(pdu)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	if p.LANHello == nil {
		t.Fatal("decoded PDU is not a LAN IIH")
	}
	for _, tv := range p.LANHello.TLVs {
		if tv.Type == packet.TLVP2PThreeWay {
			return true
		}
	}
	return false
}

// newThreeWayCircuit builds a minimal point-to-point circuit for the emission
// helpers: an L1 circuit with the given assigned extended local circuit ID and an
// empty adjacency table.
func newThreeWayCircuit(localCircuitID uint8) *Circuit {
	c := &Circuit{
		localCircuitID: localCircuitID,
		systemID:       types.SystemID{0, 0, 0, 0, 0, 1},
		levels:         []adjacency.Level{adjacency.Level1},
	}
	c.table = adjacency.NewTable()
	return c
}

// RFC requirement: RFC5303-3.1-1 positive -- every point-to-point IIH Ze
// originates carries the TLV 240 Point-to-Point Three-Way Adjacency option:
// buildP2PHello (internal/plugins/isis/circuit/hello.go:204) always appends
// threeWayTLV, so the decoded P2P IIH contains the option.
// RFC requirement: RFC5303-3.1-1 negative -- the option is point-to-point only:
// a LAN IIH built by buildLANHello NEVER carries TLV 240, proving the emission is
// scoped to point-to-point IIHs rather than blanket-appended to every Hello.
func TestISISP2PIIHCarriesThreeWayOption(t *testing.T) {
	c := newThreeWayCircuit(7)

	p2p := c.buildP2PHello(packet.AdjThreeWayDown, types.SystemID{}, false, 0)
	if _, ok := p2pHelloTLV240(t, p2p); !ok {
		t.Fatal("point-to-point IIH does not carry the TLV 240 three-way option")
	}

	lan := c.buildLANHello(adjacency.Level1, nil, 0)
	if lanHelloHasTLV240(t, lan) {
		t.Fatal("LAN IIH wrongly carries the point-to-point-only TLV 240 option")
	}
}

// RFC requirement: RFC5303-3.2-1 positive -- the current three-way state is
// reported in the transmitted option: p2pThreeWayState
// (internal/plugins/isis/circuit/runtime.go:328) reports Up for an Up adjacency.
// RFC requirement: RFC5303-3.2-1 negative -- the field tracks the current state
// rather than a fixed value: an Initializing adjacency is reported as
// Initializing, not Up.
func TestISISThreeWayReportsCurrentState(t *testing.T) {
	peer := types.SystemID{0, 0, 0, 0, 0, 2}
	c := newThreeWayCircuit(7)
	c.table.Update(peer, adjacency.Level1, func(a *adjacency.Adjacency, _ bool) {
		a.SystemID = peer
		a.Level = adjacency.Level1
		a.State = adjacency.StateUp
	})

	state, _, _, _ := c.p2pThreeWayState()
	if state != packet.AdjThreeWayUp {
		t.Fatalf("Up adjacency reported as %d, want Up (%d)", state, packet.AdjThreeWayUp)
	}

	c.table.Update(peer, adjacency.Level1, func(a *adjacency.Adjacency, _ bool) {
		a.State = adjacency.StateInitializing
	})
	state, _, _, _ = c.p2pThreeWayState()
	if state != packet.AdjThreeWayInitializing {
		t.Fatalf("Initializing adjacency reported as %d, want Initializing (%d)", state, packet.AdjThreeWayInitializing)
	}
}

// RFC requirement: RFC5303-3.2-2 positive -- when no adjacency exists the state
// is reported as Down: p2pThreeWayState (internal/plugins/isis/circuit/runtime.go:330)
// starts at AdjThreeWayDown and only a non-Down adjacency raises it.
// RFC requirement: RFC5303-3.2-2 negative -- Down is specifically the
// no-adjacency case: once an adjacency is Up the reported state is NOT Down.
func TestISISThreeWayDownWhenNoAdjacency(t *testing.T) {
	c := newThreeWayCircuit(7)

	state, _, haveNeighbor, _ := c.p2pThreeWayState()
	if state != packet.AdjThreeWayDown {
		t.Fatalf("empty table reported as %d, want Down (%d)", state, packet.AdjThreeWayDown)
	}
	if haveNeighbor {
		t.Fatal("empty table must not echo a neighbor")
	}

	peer := types.SystemID{0, 0, 0, 0, 0, 2}
	c.table.Update(peer, adjacency.Level1, func(a *adjacency.Adjacency, _ bool) {
		a.SystemID = peer
		a.Level = adjacency.Level1
		a.State = adjacency.StateUp
	})
	state, _, _, _ = c.p2pThreeWayState()
	if state == packet.AdjThreeWayDown {
		t.Fatal("Up adjacency wrongly reported as three-way Down")
	}
}

// RFC requirement: RFC5303-3.2-3 positive -- the Extended Local Circuit ID field
// carries the value assigned to the circuit at creation: threeWayTLV
// (internal/plugins/isis/circuit/hello.go:161) emits c.localCircuitID.
// RFC requirement: RFC5303-3.2-3 negative -- the field reflects the per-circuit
// assignment, not a shared constant: two circuits with different assigned IDs
// emit different Extended Local Circuit ID values.
func TestISISThreeWayExtendedLocalCircuitID(t *testing.T) {
	c := newThreeWayCircuit(7)
	dec, ok := p2pHelloTLV240(t, c.buildP2PHello(packet.AdjThreeWayDown, types.SystemID{}, false, 0))
	if !ok {
		t.Fatal("point-to-point IIH does not carry TLV 240")
	}
	if !dec.HasCircuitID || dec.LocalCircuitID != 7 {
		t.Fatalf("Extended Local Circuit ID = %d (present=%v), want 7", dec.LocalCircuitID, dec.HasCircuitID)
	}

	other := newThreeWayCircuit(42)
	dec2, _ := p2pHelloTLV240(t, other.buildP2PHello(packet.AdjThreeWayDown, types.SystemID{}, false, 0))
	if dec2.LocalCircuitID == dec.LocalCircuitID {
		t.Fatalf("two circuits emitted the same Extended Local Circuit ID %d", dec2.LocalCircuitID)
	}
}

// RFC requirement: RFC5303-3.2-5 positive -- when the neighbor's System ID is
// known (adjacency Initializing or Up) it is reported in the Neighbor System ID
// field: threeWayTLV (internal/plugins/isis/circuit/hello.go:163) echoes it.
// RFC requirement: RFC5303-3.2-5 negative -- the echo is conditional on knowing
// the neighbor: with no neighbor known the option omits the Neighbor System ID
// field entirely.
func TestISISThreeWayReportsNeighborSystemID(t *testing.T) {
	peer := types.SystemID{0, 0, 0, 0, 0, 9}
	c := newThreeWayCircuit(7)

	dec, _ := p2pHelloTLV240(t, c.buildP2PHello(packet.AdjThreeWayUp, peer, true, 0))
	if !dec.HasNeighbor || dec.NeighborID != peer {
		t.Fatalf("Neighbor System ID echo = %v (present=%v), want %v", dec.NeighborID, dec.HasNeighbor, peer)
	}

	dec2, _ := p2pHelloTLV240(t, c.buildP2PHello(packet.AdjThreeWayDown, types.SystemID{}, false, 0))
	if dec2.HasNeighbor {
		t.Fatal("Neighbor System ID field emitted when no neighbor is known")
	}
}

// RFC requirement: RFC5303-3.2-10 positive -- a freshly created adjacency (record
// present, still Down) reports a three-way state of Down: p2pThreeWayState
// (internal/plugins/isis/circuit/runtime.go:336) skips a Down record, so the base
// "Up" action never starts a new adjacency at three-way Up.
// RFC requirement: RFC5303-3.2-10 negative -- Down is specific to the
// not-yet-established adjacency: an established Up adjacency reports Up, not Down.
func TestISISThreeWayNewAdjacencyStateDown(t *testing.T) {
	peer := types.SystemID{0, 0, 0, 0, 0, 2}
	c := newThreeWayCircuit(7)
	c.table.Update(peer, adjacency.Level1, func(a *adjacency.Adjacency, _ bool) {
		a.SystemID = peer
		a.Level = adjacency.Level1
		a.State = adjacency.StateDown
	})

	state, _, _, _ := c.p2pThreeWayState()
	if state != packet.AdjThreeWayDown {
		t.Fatalf("new (Down) adjacency reported as %d, want Down (%d)", state, packet.AdjThreeWayDown)
	}

	c.table.Update(peer, adjacency.Level1, func(a *adjacency.Adjacency, _ bool) {
		a.State = adjacency.StateUp
	})
	state, _, _, _ = c.p2pThreeWayState()
	if state != packet.AdjThreeWayUp {
		t.Fatalf("Up adjacency reported as %d, want Up (%d)", state, packet.AdjThreeWayUp)
	}
}
