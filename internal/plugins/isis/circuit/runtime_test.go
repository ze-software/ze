// Design: plan/spec-isis-5-adjacency.md -- circuit RX dispatch + P2P IIH signing.
//
// VALIDATES: a P2P IIH is signed with the chain of the NEGOTIATED adjacency
// level, not the circuit's first configured level. On an L1L2 P2P circuit
// c.levels[0] is always Level1, so signing with c.levels[0] would sign an
// L2-negotiated session with the L1 key; this asserts an L2-negotiated P2P
// session signs with the L2 chain (and an L1-negotiated one with L1, and the
// no-neighbor case with the circuit's preferred level).
// PREVENTS: regression to signing every P2P Hello with Level1 (RFC 5303 sec 3:
// the P2P IIH is level-agnostic on the wire, so the chain is chosen by the
// negotiated level).

package circuit

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/isis/adjacency"
	"codeberg.org/thomas-mangin/ze/internal/plugins/isis/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/isis/transport"
	"codeberg.org/thomas-mangin/ze/internal/plugins/isis/types"
)

// recordSigner records the levels SendHello asks it to sign at, so a test can
// assert which IIH chain the negotiated P2P level selected.
type recordSigner struct {
	levels []adjacency.Level
}

func (r *recordSigner) sign(level adjacency.Level, pdu []byte) []byte {
	r.levels = append(r.levels, level)
	return pdu
}

// captureSigner records the exact bytes SendHello hands to the signer, so a test
// can assert the signer runs over the already-PADDED IIH (RFC 5304 sec 2).
type captureSigner struct {
	pdu []byte
}

func (s *captureSigner) sign(_ adjacency.Level, pdu []byte) []byte {
	s.pdu = append([]byte(nil), pdu...)
	return pdu
}

// TestISISHelloSignedOverPaddedPDU: the IIH signer runs over the PADDED Hello, not
// the unpadded one. SendHello builds and pads the IIH to MTU - LLC and THEN calls
// the signer (circuit/runtime.go:273-276), so the HMAC covers the padding. This
// asserts the bytes handed to the signer are the padded bytes: the full pad-target
// length AND a Padding TLV 8 present.
// PREVENTS: reordering to sign-before-pad, which would compute the digest over the
// unpadded PDU and break RFC 5304 sec 2 ("The HMAC-MD5 result for the IS-IS Hello
// PDUs SHALL be calculated after the packet is padded to the MTU size").
func TestISISHelloSignedOverPaddedPDU(t *testing.T) {
	const mtu = 1497
	const wantPDU = mtu - transport.LLCHeaderLen // LLC + PDU must fill the link MTU
	s := &fakeSender{mtu: mtu}
	c := lanCircuit(t, s)
	cs := &captureSigner{}
	c.SetSigner(cs.sign)

	if err := c.SendHello(); err != nil {
		t.Fatal(err)
	}
	if cs.pdu == nil {
		t.Fatal("signer was never invoked")
	}
	// RFC requirement: RFC5304-2-4 positive -- the PDU the signer receives is
	// already padded to the MTU (length == MTU - LLC), so the HMAC is computed
	// AFTER padding, not before (RFC 5304 sec 2).
	if len(cs.pdu) != wantPDU {
		t.Fatalf("signer received a %d-octet PDU, want %d (padded to MTU %d - LLC %d); signing ran before padding",
			len(cs.pdu), wantPDU, mtu, transport.LLCHeaderLen)
	}
	// The bytes the signer saw carry a Padding TLV 8: padding preceded signing.
	p, err := packet.DecodePDU(cs.pdu)
	if err != nil {
		t.Fatalf("captured PDU does not decode: %v", err)
	}
	if !hasTLV(p.LANHello.TLVs, packet.TLVPadding) {
		t.Fatal("signer received an UNPADDED IIH (no Padding TLV 8): the HMAC would not cover the padding (RFC 5304 sec 2)")
	}
}

// l1l2P2PCircuit builds a point-to-point circuit configured for BOTH levels
// (Levels[0] == Level1), the case where signing with c.levels[0] is always L1.
func l1l2P2PCircuit(t *testing.T) *Circuit {
	t.Helper()
	c := p2pCircuit(t, &fakeSender{mtu: 1500})
	c.levels = []adjacency.Level{adjacency.Level1, adjacency.Level2}
	return c
}

// buildPeerP2PHello encodes a P2P IIH from the peer with the given circuit type
// and a TLV 240 echoing OUR System ID (so the three-way handshake completes and
// the adjacency reaches Up at the negotiated level). An L2-only circuit type
// (CircuitL2) negotiates Level2; CircuitL1L2 negotiates Level1.
func buildPeerP2PHello(t *testing.T, ct packet.CircuitType, ourSystemID types.SystemID) []byte {
	t.Helper()
	area, _ := types.AreaIDFromBytes([]byte{0x49, 0x00, 0x01})
	areaVal := []byte{byte(area.Len())}
	areaVal = append(areaVal, area.Bytes()...)

	// TLV 240: state Up + extended local circuit ID + neighbor echo (our System ID)
	// + neighbor extended circuit ID. The neighbor echo proves the peer heard us.
	tw := []byte{byte(packet.AdjThreeWayUp), 0, 0, 0, 0x09}
	tw = append(tw, ourSystemID[:]...)
	tw = append(tw, 0, 0, 0, 0)

	h := packet.P2PHello{
		CircuitType:    ct,
		SystemID:       types.SystemID{0, 0, 0, 0, 0, 2},
		HoldingTime:    types.HoldingTime(30),
		LocalCircuitID: 9,
		TLVs: []packet.TLV{
			{Type: packet.TLVAreaAddresses, Value: areaVal},
			{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv4}},
			{Type: packet.TLVP2PThreeWay, Value: tw},
		},
	}
	buf := make([]byte, h.EncodedLen())
	return buf[:h.WriteTo(buf, 0)]
}

// TestISISP2PHelloSignedAtNegotiatedLevel: on an L1L2 P2P circuit, the IIH is
// signed with the NEGOTIATED adjacency level. An L2-only neighbor negotiates
// Level2, so the Hello must be signed with the L2 chain -- NOT c.levels[0]
// (Level1). An L1L2 neighbor negotiates Level1, and with no neighbor heard the
// circuit signs with its preferred level (L1 for an L1L2 circuit).
func TestISISP2PHelloSignedAtNegotiatedLevel(t *testing.T) {
	t.Run("L2-only neighbor signs with L2 chain", func(t *testing.T) {
		c := l1l2P2PCircuit(t)
		sg := &recordSigner{}
		c.SetSigner(sg.sign)

		// Drive an L2-only P2P Hello to Up: p2pLevel(CircuitL2) -> Level2.
		pdu := buildPeerP2PHello(t, packet.CircuitL2, c.systemID)
		if tr := c.Receive(adjacency.SNPA{}, pdu); tr.State != adjacency.StateUp {
			t.Fatalf("L2 P2P Hello -> state %v, want up", tr.State)
		}
		if got, ok := c.Table().Lookup(types.SystemID{0, 0, 0, 0, 0, 2}, adjacency.Level2); !ok || got.Level != adjacency.Level2 {
			t.Fatalf("expected an L2 adjacency, lookup ok=%v adj=%+v", ok, got)
		}

		if err := c.SendHello(); err != nil {
			t.Fatal(err)
		}
		if len(sg.levels) != 1 {
			t.Fatalf("signer called %d times, want 1 (one P2P IIH)", len(sg.levels))
		}
		if sg.levels[0] != adjacency.Level2 {
			t.Fatalf("P2P IIH signed at level %v, want Level2 (negotiated level)", sg.levels[0])
		}
	})

	t.Run("L1L2 neighbor signs with L1 chain", func(t *testing.T) {
		c := l1l2P2PCircuit(t)
		sg := &recordSigner{}
		c.SetSigner(sg.sign)

		// p2pLevel(CircuitL1L2) -> Level1 (L1 preferred when both support it).
		pdu := buildPeerP2PHello(t, packet.CircuitL1L2, c.systemID)
		if tr := c.Receive(adjacency.SNPA{}, pdu); tr.State != adjacency.StateUp {
			t.Fatalf("L1L2 P2P Hello -> state %v, want up", tr.State)
		}

		if err := c.SendHello(); err != nil {
			t.Fatal(err)
		}
		if len(sg.levels) != 1 || sg.levels[0] != adjacency.Level1 {
			t.Fatalf("P2P IIH signed at %v, want Level1 (negotiated)", sg.levels)
		}
	})

	t.Run("no neighbor signs with preferred level", func(t *testing.T) {
		c := l1l2P2PCircuit(t)
		sg := &recordSigner{}
		c.SetSigner(sg.sign)

		// No adjacency yet: fall back to the circuit's preferred P2P level (L1 for
		// an L1L2 circuit). This must NOT panic and must pick a deterministic level.
		if err := c.SendHello(); err != nil {
			t.Fatal(err)
		}
		if len(sg.levels) != 1 || sg.levels[0] != adjacency.Level1 {
			t.Fatalf("no-neighbor P2P IIH signed at %v, want preferred Level1", sg.levels)
		}
	})

	t.Run("L2-only circuit prefers L2", func(t *testing.T) {
		c := p2pCircuit(t, &fakeSender{mtu: 1500})
		c.levels = []adjacency.Level{adjacency.Level2}
		sg := &recordSigner{}
		c.SetSigner(sg.sign)

		if err := c.SendHello(); err != nil {
			t.Fatal(err)
		}
		if len(sg.levels) != 1 || sg.levels[0] != adjacency.Level2 {
			t.Fatalf("L2-only P2P circuit signed at %v, want Level2", sg.levels)
		}
	})
}
