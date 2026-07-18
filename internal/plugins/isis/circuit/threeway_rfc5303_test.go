// VALIDATES: RFC 5303 sec 3.1 -- Ze includes the Adjacency Three-Way State
// field in every P2P Three-Way Adjacency option (TLV 240) it originates, in
// both the full (with-neighbor) and the minimal (no-neighbor) forms.
// PREVENTS: emitting a TLV 240 that omits the mandatory state field, which a
// three-way-capable neighbor requires to run the RFC 5303 handshake.
package circuit

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/isis/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/isis/types"
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
