// Design: docs/architecture/isis/isis-5-adjacency.md -- the P2P IIH echo of the neighbor's circuit.
//
// VALIDATES: RFC 5303 sec 3.2 -- the TLV 240 a point-to-point IIH carries
// echoes the Extended Local Circuit ID the neighbor reported, per circuit, so
// two circuits to two neighbors each name their own neighbor's circuit, and a
// neighbor whose circuit ID is unknown gets no neighbor echo at all.
// PREVENTS: the echo of a constant 0, which FRR's process_p2p_hello compares
// with its own circuit index and discards ("lists IS/Circuit different from us
// as neighbor"), so no adjacency forms on any FRR circuit whose index is not 0.

package circuit

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// echoNeighbor is one P2P neighbor of the echo tests: the System ID and the
// Extended Local Circuit ID it reports in its TLV 240.
type echoNeighbor struct {
	systemID          types.SystemID
	extendedCircuitID uint32
}

// echoNeighborHello encodes a P2P IIH from n carrying a TLV 240 in state Down.
// With withCircuit false the TLV 240 is the 1-octet form, which carries no
// Extended Local Circuit ID.
func echoNeighborHello(t *testing.T, n echoNeighbor, withCircuit bool) []byte {
	t.Helper()
	area := testArea(t)
	areaValue := []byte{byte(area.Len())}
	areaValue = append(areaValue, area.Bytes()...)

	threeWay := []byte{byte(packet.AdjThreeWayDown)}
	if withCircuit {
		cid := n.extendedCircuitID
		threeWay = append(threeWay, byte(cid>>24), byte(cid>>16), byte(cid>>8), byte(cid))
	}
	h := packet.P2PHello{
		CircuitType:    packet.CircuitL1,
		SystemID:       n.systemID,
		HoldingTime:    types.HoldingTime(30),
		LocalCircuitID: 1,
		TLVs: []packet.TLV{
			{Type: packet.TLVAreaAddresses, Value: areaValue},
			{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv4}},
			{Type: packet.TLVP2PThreeWay, Value: threeWay},
		},
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

// echoCircuit builds a P2P circuit with its own sender and circuit ID, fed one
// Hello from n, and fails unless the adjacency reaches Initializing.
func echoCircuit(t *testing.T, circuitID uint8, n echoNeighbor, withCircuit bool) (*Circuit, *fakeSender) {
	t.Helper()
	s := &fakeSender{mtu: 1500}
	c := p2pCircuit(t, s)
	c.localCircuitID = circuitID
	if tr := c.Receive(adjacency.SNPA{}, echoNeighborHello(t, n, withCircuit)); tr.State != adjacency.StateInitializing {
		t.Fatalf("circuit %d: neighbor IIH -> adjacency %v, want %v", circuitID, tr.State, adjacency.StateInitializing)
	}
	return c, s
}

// RFC requirement: RFC5303-3.2-6 positive -- once the neighbor's Extended Local
// Circuit ID is known, the sent IIH reports it in the Neighbor Extended Local
// Circuit ID field. Two circuits to two neighbors each echo their own
// neighbor's System ID and Extended Local Circuit ID (1 and 0x2a), not a
// shared or constant value.
func TestRFC5303SentIIHEchoesNeighborExtendedCircuitID(t *testing.T) {
	frrA := echoNeighbor{systemID: types.SystemID{0, 0, 0, 0, 0, 3}, extendedCircuitID: 1}
	frrB := echoNeighbor{systemID: types.SystemID{0, 0, 0, 0, 0, 0x15}, extendedCircuitID: 0x2a}

	eth1, s1 := echoCircuit(t, 7, frrA, true)
	eth2, s2 := echoCircuit(t, 8, frrB, true)

	for _, tc := range []struct {
		name string
		c    *Circuit
		s    *fakeSender
		want echoNeighbor
	}{
		{"eth1", eth1, s1, frrA},
		{"eth2", eth2, s2, frrB},
	} {
		tw := sentThreeWay(t, tc.c, tc.s)
		if !tw.HasNeighbor {
			t.Fatalf("%s: sent TLV 240 carries no neighbor echo", tc.name)
		}
		if tw.NeighborID != tc.want.systemID {
			t.Fatalf("%s: sent Neighbor System ID %v, want %v", tc.name, tw.NeighborID, tc.want.systemID)
		}
		if tw.NeighborCircuit != tc.want.extendedCircuitID {
			t.Fatalf("%s: sent Neighbor Extended Local Circuit ID %d, want %d",
				tc.name, tw.NeighborCircuit, tc.want.extendedCircuitID)
		}
	}
}

// RFC requirement: RFC5303-3.2-6 negative -- the neighbor fields are reported
// only when the neighbor's Extended Local Circuit ID is known: a neighbor whose
// TLV 240 is the 1-octet form gets our state with no Neighbor System ID and no
// Neighbor Extended Local Circuit ID, never an invented value.
func TestRFC5303SentIIHNoEchoWithoutNeighborExtendedCircuitID(t *testing.T) {
	n := echoNeighbor{systemID: types.SystemID{0, 0, 0, 0, 0, 3}, extendedCircuitID: 1}
	c, s := echoCircuit(t, 7, n, false)

	tw := sentThreeWay(t, c, s)
	if tw.State != packet.AdjThreeWayInitializing {
		t.Fatalf("sent three-way state %d, want %d", tw.State, packet.AdjThreeWayInitializing)
	}
	if tw.HasNeighbor {
		t.Fatalf("sent TLV 240 echoes neighbor %v circuit %d though the neighbor reported no circuit ID",
			tw.NeighborID, tw.NeighborCircuit)
	}
}
