// Design: docs/architecture/isis/isis-5-adjacency.md -- the P2P IIH as it leaves the circuit.
//
// VALIDATES: the fields of the IIH that SendHello hands to the transport, not
// the helper values that feed it. The RFC 5303 TLV 240 state and neighbor echo
// follow the adjacency as received Hellos move it; a point-to-point IIH is
// signed only after padding (RFC 5310); the Hello TLV 232 carries the sending
// circuit's own link-local address and TLV 129 carries NLPID 142 (RFC 5308).
// PREVENTS: a send path that ignores the computed three-way state, an encoder
// that emits a fixed address, and a wrong IPv6 NLPID constant that every
// comparison against packet.NLPIDIPv6 would still accept.

package circuit

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// sentPeerID is the neighbor System ID the three-way tests drive.
var sentPeerID = types.SystemID{0, 0, 0, 0, 0, 2}

// peerP2PHelloThreeWay encodes a P2P IIH from sentPeerID carrying a TLV 240 with
// the given state, echoing echo as the Neighbor System ID when echo is non-nil.
func peerP2PHelloThreeWay(t *testing.T, state packet.AdjThreeWayState, echo *types.SystemID) []byte {
	t.Helper()
	area := testArea(t)
	areaValue := []byte{byte(area.Len())}
	areaValue = append(areaValue, area.Bytes()...)

	threeWay := []byte{byte(state), 0, 0, 0, 0x09}
	if echo != nil {
		threeWay = append(threeWay, echo[:]...)
		threeWay = append(threeWay, 0, 0, 0, 0)
	}
	h := packet.P2PHello{
		CircuitType:    packet.CircuitL1,
		SystemID:       sentPeerID,
		HoldingTime:    types.HoldingTime(30),
		LocalCircuitID: 9,
		TLVs: []packet.TLV{
			{Type: packet.TLVAreaAddresses, Value: areaValue},
			{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv4}},
			{Type: packet.TLVP2PThreeWay, Value: threeWay},
		},
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

// sentThreeWay sends one Hello on c and returns the TLV 240 of the P2P IIH the
// transport received (the last PDU, after the ISH that precedes it).
func sentThreeWay(t *testing.T, c *Circuit, s *fakeSender) packet.P2PThreeWayTLV {
	t.Helper()
	if err := c.SendHello(adjacency.Level1); err != nil {
		t.Fatal(err)
	}
	last := s.sent[len(s.sent)-1]
	if !last.both {
		t.Fatal("the last PDU sent is not the point-to-point IIH (sent to both levels)")
	}
	tw, ok := p2pHelloTLV240(t, last.pdu)
	if !ok {
		t.Fatal("the sent point-to-point IIH carries no TLV 240")
	}
	return tw
}

// receiveThreeWay feeds c one peer IIH and fails unless the adjacency lands in want.
func receiveThreeWay(t *testing.T, c *Circuit, state packet.AdjThreeWayState, echo *types.SystemID, want adjacency.State) {
	t.Helper()
	if tr := c.Receive(adjacency.SNPA{}, peerP2PHelloThreeWay(t, state, echo)); tr.State != want {
		t.Fatalf("peer IIH with three-way state %d -> adjacency %v, want %v", state, tr.State, want)
	}
}

// RFC requirement: RFC5303-3.2-1 positive -- the IIH SendHello transmits reports
// the adjacency's current three-way state: Initializing after the peer's first
// Hello (no echo of us), Up once the peer echoes our System ID.
// RFC requirement: RFC5303-3.2-1 negative -- the transmitted field is not a
// fixed value: the same circuit sends Initializing and then Up, never Up while
// the adjacency is still Initializing.
func TestRFC5303SentIIHReportsCurrentThreeWayState(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := p2pCircuit(t, s)

	receiveThreeWay(t, c, packet.AdjThreeWayDown, nil, adjacency.StateInitializing)
	if tw := sentThreeWay(t, c, s); tw.State != packet.AdjThreeWayInitializing {
		t.Fatalf("sent three-way state %d while the adjacency is Initializing, want Initializing (%d)",
			tw.State, packet.AdjThreeWayInitializing)
	}

	ours := c.systemID
	receiveThreeWay(t, c, packet.AdjThreeWayInitializing, &ours, adjacency.StateUp)
	if tw := sentThreeWay(t, c, s); tw.State != packet.AdjThreeWayUp {
		t.Fatalf("sent three-way state %d while the adjacency is Up, want Up (%d)", tw.State, packet.AdjThreeWayUp)
	}
}

// RFC requirement: RFC5303-3.2-2 positive -- with no adjacency the transmitted
// IIH reports Down: before any peer Hello, and again after the circuit tears the
// adjacency down.
// RFC requirement: RFC5303-3.2-2 negative -- Down is reserved for the
// no-adjacency case: while an adjacency exists the transmitted state is not Down.
func TestRFC5303SentIIHReportsDownWithoutAdjacency(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := p2pCircuit(t, s)

	if tw := sentThreeWay(t, c, s); tw.State != packet.AdjThreeWayDown {
		t.Fatalf("sent three-way state %d with no adjacency, want Down (%d)", tw.State, packet.AdjThreeWayDown)
	}

	ours := c.systemID
	receiveThreeWay(t, c, packet.AdjThreeWayInitializing, &ours, adjacency.StateUp)
	if tw := sentThreeWay(t, c, s); tw.State == packet.AdjThreeWayDown {
		t.Fatal("sent three-way state Down while an Up adjacency exists")
	}

	c.Teardown()
	if tw := sentThreeWay(t, c, s); tw.State != packet.AdjThreeWayDown {
		t.Fatalf("sent three-way state %d after the adjacency went down, want Down (%d)", tw.State, packet.AdjThreeWayDown)
	}
}

// RFC requirement: RFC5303-3.2-5 positive -- once the neighbor is known (three-way
// Initializing, then Up) the transmitted IIH carries its System ID in the
// Neighbor System ID field.
// RFC requirement: RFC5303-3.2-5 negative -- with no neighbor known the
// transmitted IIH carries no Neighbor System ID field.
func TestRFC5303SentIIHEchoesKnownNeighbor(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := p2pCircuit(t, s)

	if tw := sentThreeWay(t, c, s); tw.HasNeighbor {
		t.Fatalf("sent IIH echoes neighbor %v before any neighbor is known", tw.NeighborID)
	}

	receiveThreeWay(t, c, packet.AdjThreeWayDown, nil, adjacency.StateInitializing)
	if tw := sentThreeWay(t, c, s); !tw.HasNeighbor || tw.NeighborID != sentPeerID {
		t.Fatalf("Initializing: sent Neighbor System ID %v (present=%v), want %v", tw.NeighborID, tw.HasNeighbor, sentPeerID)
	}

	ours := c.systemID
	receiveThreeWay(t, c, packet.AdjThreeWayInitializing, &ours, adjacency.StateUp)
	if tw := sentThreeWay(t, c, s); !tw.HasNeighbor || tw.NeighborID != sentPeerID {
		t.Fatalf("Up: sent Neighbor System ID %v (present=%v), want %v", tw.NeighborID, tw.HasNeighbor, sentPeerID)
	}
}

// TestRFC5310P2PHelloSignedOverPaddedPDU: the point-to-point IIH producer
// (sendP2PHello) pads before it signs, as sendLANHello does. The signer must
// receive the PDU already filled to MTU - LLC with a Padding TLV 8 inside.
//
// RFC requirement: RFC5310-3.2-5 positive -- the point-to-point IIH handed to the
// CRYPTO_AUTH signer is already padded to the MTU (length MTU - LLC, Padding TLV 8 present).
// RFC requirement: RFC5310-3.4-2 positive -- the point-to-point IIH authentication
// data is computed over the padded Hello: the signer sees MTU - LLC octets with a Padding TLV 8.
func TestRFC5310P2PHelloSignedOverPaddedPDU(t *testing.T) {
	const mtu = 1497
	const wantPDU = mtu - transport.LLCHeaderLen
	c := p2pCircuit(t, &fakeSender{mtu: mtu})
	cs := &captureSigner{}
	c.SetSigner(cs.sign, nil)

	if err := c.SendHello(adjacency.Level1); err != nil {
		t.Fatal(err)
	}
	if cs.pdu == nil {
		t.Fatal("signer was never invoked for the point-to-point IIH")
	}
	if len(cs.pdu) != wantPDU {
		t.Fatalf("signer received a %d-octet point-to-point IIH, want %d: it was signed before padding", len(cs.pdu), wantPDU)
	}
	p, err := packet.DecodePDU(cs.pdu)
	if err != nil {
		t.Fatalf("captured PDU does not decode: %v", err)
	}
	if p.P2PHello == nil {
		t.Fatal("the signer did not receive a point-to-point IIH")
	}
	if !hasTLV(p.P2PHello.TLVs, packet.TLVPadding) {
		t.Fatal("signer received a point-to-point IIH without Padding TLV 8: signed before padding")
	}
}

// sentTLV sends one level-1 Hello on c and returns the value of TLV typ in the
// LAN IIH the transport received, failing when the TLV is absent.
func sentTLV(t *testing.T, c *Circuit, s *fakeSender, typ uint8) []byte {
	t.Helper()
	if err := c.SendHello(adjacency.Level1); err != nil {
		t.Fatal(err)
	}
	p := decodeSent(t, s)
	if p.LANHello == nil {
		t.Fatal("expected a LAN IIH")
	}
	for _, tl := range p.LANHello.TLVs {
		if tl.Type == typ {
			return tl.Value
		}
	}
	t.Fatalf("sent IIH carries no TLV %d", typ)
	return nil
}

// RFC requirement: RFC5308-3-1 positive -- the Hello TLV 232 carries the
// link-local address assigned to the circuit sending it: two circuits with
// different link-local addresses each send exactly their own (16 octets, no
// other address), so a fixed or shared address cannot pass.
func TestRFC5308HelloTLV232IsTheSendingCircuitLinkLocal(t *testing.T) {
	for _, local := range []string{"fe80::1", "fe80::a:b"} {
		s := &fakeSender{mtu: 1500}
		c := dualStackLAN(t, s)
		c.ipv6LinkLocal = netip.MustParseAddr(local)

		want := c.ipv6LinkLocal.As16()
		if got := sentTLV(t, c, s, packet.TLVIPv6InterfaceAddress); !bytes.Equal(got, want[:]) {
			t.Fatalf("circuit with link-local %s sent TLV 232 % x, want exactly % x", local, got, want)
		}
	}
}

// RFC requirement: RFC5308-4-1 positive -- a dual-stack circuit's Hello TLV 129
// carries the literal IPv6 NLPID octet 142 (0x8E) beside IPv4's 0xCC, checked
// against the wire octets rather than the packet.NLPIDIPv6 constant.
func TestRFC5308HelloNLPIDIs142(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := dualStackLAN(t, s)
	if got := sentTLV(t, c, s, packet.TLVProtocolsSupported); !bytes.Equal(got, []byte{0xCC, 142}) {
		t.Fatalf("dual-stack TLV 129 = % x, want cc 8e", got)
	}
}
