// Design: docs/architecture/isis/isis-5-adjacency.md -- point-to-point adjacency formation.
// Related: protocol_rfc1195_test.go -- the ISH transmit and receive units.
//
// VALIDATES: how Ze handles a point-to-point neighbor that never sends an ISO
// 9542 ISH. The ISH only classifies the neighbor (ISO/IEC 10589 section 10.3);
// the adjacency itself is made by the IIH exchange, so a neighbor that sends
// IIHs alone still completes the RFC 5303 three-way handshake to Up.
// PREVENTS: an adjacency that waits for, or requires, an ISH the neighbor may
// never send.
package isis

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// noISHPeerIIH encodes the neighbor's P2P IIH carrying a 15-octet TLV 240 with
// state and, when echo is set, Ze's system ID as the Neighbor System ID.
func noISHPeerIIH(t *testing.T, state packet.AdjThreeWayState, echo types.SystemID) []byte {
	t.Helper()
	threeWay := make([]byte, 0, 15)
	threeWay = append(threeWay, byte(state), 0, 0, 0, 7)
	threeWay = append(threeWay, echo[:]...)
	threeWay = append(threeWay, 0, 0, 0, 0)
	h := packet.P2PHello{
		CircuitType:    packet.CircuitL1,
		SystemID:       onLinkPeer,
		HoldingTime:    30,
		LocalCircuitID: 7,
		TLVs: []packet.TLV{
			{Type: packet.TLVAreaAddresses, Value: []byte{3, 0x49, 0, 1}},
			{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv4}},
			{Type: packet.TLVP2PThreeWay, Value: threeWay},
		},
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

// RFC requirement: RFC1195-4.4-2 negative -- a point-to-point neighbor that
// never sends an ISH is not refused: its first IIH (three-way Down) makes an
// Initializing adjacency and its next IIH (three-way Initializing, naming Ze)
// brings it Up, with no ISH received on the circuit at any point.
func TestRFC1195NeighborWithoutISHStillFormsAdjacency(t *testing.T) {
	eng, _ := protocolEngine(t, protocolP2PConfig)
	c := protocolLiveCircuit(t, eng, "eth0")
	local := types.SystemID{0, 0, 0, 0, 0, 1}
	if eng.cfg.NETs[0].SystemID() != local {
		t.Fatalf("setup: local system ID %v, want %v", eng.cfg.NETs[0].SystemID(), local)
	}

	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: noISHPeerIIH(t, packet.AdjThreeWayDown, types.SystemID{})})
	rows := c.Table().Snapshot()
	if len(rows) != 1 || rows[0].State != "initializing" {
		t.Fatalf("first IIH without a prior ISH: %+v, want one initializing adjacency", rows)
	}

	eng.dispatch.dispatch(transport.RawFrame{IfIndex: c.IfIndex(), PDU: noISHPeerIIH(t, packet.AdjThreeWayInitializing, local)})
	rows = c.Table().Snapshot()
	if len(rows) != 1 || rows[0].State != "up" {
		t.Fatalf("IIH exchange without any ISH: %+v, want one up adjacency", rows)
	}
}
