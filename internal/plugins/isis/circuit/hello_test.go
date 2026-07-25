// Design: plan/spec-isis-5-adjacency.md -- IIH origination + padding + hold time.
//
// VALIDATES: the advertised hold time = hello-interval * hold-multiplier
// (clamped to the 16-bit range, boundary cases included); the originated LAN IIH
// carries TLV 1 / 129 / 132 / 6 and the P2P IIH carries TLV 1 / 129 / 132 / 240;
// the Padding TLV 8 fills the PDU to the interface MTU during construction
// (BEFORE auth), and the transport sees only the final padded bytes.
// PREVENTS: a zero/oversized hold time, a missing origination TLV, or padding
// that does not reach the MTU.

package circuit

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// fakeSender captures sent PDUs so tests can inspect the encoded IIH.
type fakeSender struct {
	mtu  int
	sent []sentPDU
}

type sentPDU struct {
	name  string
	level transport.Level
	pdu   []byte
	both  bool
}

func (s *fakeSender) SendPDU(name string, level transport.Level, pdu []byte) error {
	s.sent = append(s.sent, sentPDU{name: name, level: level, pdu: append([]byte(nil), pdu...)})
	return nil
}

func (s *fakeSender) SendPDUBothLevels(name string, pdu []byte) error {
	s.sent = append(s.sent, sentPDU{name: name, pdu: append([]byte(nil), pdu...), both: true})
	return nil
}

func (s *fakeSender) InterfaceMTU(string) (int, bool) {
	if s.mtu == 0 {
		return 0, false
	}
	return s.mtu, true
}

func testArea(t *testing.T) types.AreaID {
	t.Helper()
	a, err := types.AreaIDFromBytes([]byte{0x49, 0x00, 0x01})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func lanCircuit(t *testing.T, s Sender) *Circuit {
	t.Helper()
	return New(Config{
		Name:          "eth0",
		IfIndex:       3,
		SystemID:      types.SystemID{0, 0, 0, 0, 0, 1},
		SNPA:          adjacency.SNPA{0x02, 0, 0, 0, 0, 1},
		Areas:         []types.AreaID{testArea(t)},
		IPv4:          netip.MustParseAddr("192.0.2.1"),
		Kind:          adjacency.KindBroadcast,
		Levels:        []adjacency.Level{adjacency.Level1},
		HelloInterval: 10,
		HoldMult:      3,
		Priority:      64,
	}, s, nil)
}

func p2pCircuit(t *testing.T, s Sender) *Circuit {
	t.Helper()
	c := lanCircuit(t, s)
	c.kind = adjacency.KindP2P
	c.snpa = adjacency.SNPA{}
	c.localCircuitID = 7
	return c
}

// TestISISHoldTimeFromMultiplier: hold time = interval * multiplier, with the
// boundary cases from the spec (0 clamps up, overflow clamps down).
func TestISISHoldTimeFromMultiplier(t *testing.T) {
	cases := []struct {
		interval uint16
		mult     uint8
		want     uint16
	}{
		{10, 3, 30},         // typical
		{1, 1, 1},           // smallest valid product
		{0, 3, 1},           // zero interval clamps to MinHoldTime
		{10, 0, 1},          // zero multiplier clamps to MinHoldTime
		{65535, 255, 65535}, // overflow clamps to MaxHoldTime
		{65535, 1, 65535},   // last value that fits
		{300, 255, 65535},   // 76500 overflows -> clamp
	}
	for _, c := range cases {
		if got := HoldTime(c.interval, c.mult); got != c.want {
			t.Errorf("HoldTime(%d,%d) = %d, want %d", c.interval, c.mult, got, c.want)
		}
	}
}

// decodeSent decodes the last sent PDU into its typed form.
func decodeSent(t *testing.T, s *fakeSender) packet.PDU {
	t.Helper()
	if len(s.sent) == 0 {
		t.Fatal("no PDU sent")
	}
	p, err := packet.DecodePDU(s.sent[len(s.sent)-1].pdu)
	if err != nil {
		t.Fatalf("DecodePDU: %v", err)
	}
	return p
}

func hasTLV(tlvs []packet.TLV, typ uint8) bool {
	for _, t := range tlvs {
		if t.Type == typ {
			return true
		}
	}
	return false
}

// TestISISIIHOriginationTLVs: the originated LAN IIH carries TLV 1/129/132/6 and
// the P2P IIH carries TLV 1/129/132/240.
//
// RFC requirement: RFC3787-x-2 positive -- the originated IIH (both the LAN and
// the P2P form) carries a Protocols Supported TLV (129) and an IP Interface
// Address TLV (132), the mixed-environment interoperability TLVs RFC 3787 sec
// 9/10 (RFC 1195) require in every Hello. The 129 value advertising the IPv4
// NLPID is asserted by TestISISHelloTLV132RequiresInterfaceAddr.
func TestISISIIHOriginationTLVs(t *testing.T) {
	t.Run("LAN", func(t *testing.T) {
		s := &fakeSender{mtu: 1500}
		c := lanCircuit(t, s)
		if err := c.SendHello(); err != nil {
			t.Fatal(err)
		}
		p := decodeSent(t, s)
		if p.LANHello == nil {
			t.Fatalf("expected a LAN IIH, got %+v", p)
		}
		for _, typ := range []uint8{packet.TLVAreaAddresses, packet.TLVProtocolsSupported, packet.TLVIPInterfaceAddress, packet.TLVISNeighbors} {
			if !hasTLV(p.LANHello.TLVs, typ) {
				t.Errorf("LAN IIH missing TLV %d", typ)
			}
		}
		if p.LANHello.PDUType != packet.PDUTypeL1LANHello {
			t.Errorf("LAN PDU type = %v, want L1 LAN hello", p.LANHello.PDUType)
		}
		if got := p.LANHello.HoldingTime.Seconds(); got != 30 {
			t.Errorf("advertised hold time = %d, want 30", got)
		}
	})

	t.Run("P2P", func(t *testing.T) {
		s := &fakeSender{mtu: 1500}
		c := p2pCircuit(t, s)
		if err := c.SendHello(); err != nil {
			t.Fatal(err)
		}
		p := decodeSent(t, s)
		if p.P2PHello == nil {
			t.Fatalf("expected a P2P IIH, got %+v", p)
		}
		for _, typ := range []uint8{packet.TLVAreaAddresses, packet.TLVProtocolsSupported, packet.TLVIPInterfaceAddress, packet.TLVP2PThreeWay} {
			if !hasTLV(p.P2PHello.TLVs, typ) {
				t.Errorf("P2P IIH missing TLV %d", typ)
			}
		}
		if !s.sent[len(s.sent)-1].both {
			t.Error("P2P Hello should be sent to both level groups")
		}
	})
}

// RFC requirement: RFC3787-x-2 negative -- the origination of the IP Interface
// Address TLV (132) is bounded to a REAL interface address: a circuit with no
// IPv4 address omits TLV 132 (returns a zero Type-0 TLV the caller drops) rather
// than emitting a garbage/zero one, while the Protocols Supported TLV (129) still
// advertises the IPv4 NLPID (0xCC). This pins that the mixed-environment TLVs are
// generated from live interface state, not fabricated (RFC 3787 sec 9/10, RFC 1195).
func TestISISHelloTLV132RequiresInterfaceAddr(t *testing.T) {
	c := &Circuit{} // no IPv4 interface address configured

	if tlv := c.ipv4InterfaceAddrTLV(); tlv.Type != 0 {
		t.Fatalf("TLV 132 emitted without an interface address: type=%d value=% x", tlv.Type, tlv.Value)
	}

	ps := c.protocolsSupportedTLV()
	if ps.Type != packet.TLVProtocolsSupported {
		t.Fatalf("Protocols Supported TLV type = %d, want %d", ps.Type, packet.TLVProtocolsSupported)
	}
	var hasIPv4 bool
	for _, nlpid := range ps.Value {
		if nlpid == packet.NLPIDIPv4 {
			hasIPv4 = true
		}
	}
	if !hasIPv4 {
		t.Fatalf("TLV 129 does not advertise the IPv4 NLPID %#02x: % x", packet.NLPIDIPv4, ps.Value)
	}
}

// TestISISAreaAddressesTLVManyAreasNoPanic: a circuit configured with far more
// area addresses than fit one TLV value must not panic when building TLV 1. The
// value buffer is a fixed [255]byte; without the bounds check, the 19th
// max-length (13-octet) area overflows it. The built TLV value must stay within
// MaxTLVValueLen and carry only the entries that fit (ISO/IEC 10589 clause 9.8).
func TestISISAreaAddressesTLVManyAreasNoPanic(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := lanCircuit(t, s)

	// 30 distinct maximum-length (13-octet) area addresses: 30*14 = 420 octets of
	// entries, well over the 255-octet TLV value limit.
	areas := make([]types.AreaID, 0, 30)
	for i := range 30 {
		raw := []byte{0x49, byte(i >> 8), byte(i), 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		a, err := types.AreaIDFromBytes(raw)
		if err != nil {
			t.Fatalf("AreaIDFromBytes: %v", err)
		}
		areas = append(areas, a)
	}
	c.areas = areas

	// Must not panic, and the value must fit one TLV.
	tlv := c.areaAddressesTLV()
	if tlv.Type != packet.TLVAreaAddresses {
		t.Fatalf("TLV type = %d, want %d", tlv.Type, packet.TLVAreaAddresses)
	}
	if len(tlv.Value) > packet.MaxTLVValueLen {
		t.Fatalf("TLV 1 value length = %d, exceeds MaxTLVValueLen %d", len(tlv.Value), packet.MaxTLVValueLen)
	}
	// The whole Hello path (which calls areaAddressesTLV via originationTLVs) must
	// also stay panic-free with the oversized area set.
	if err := c.SendHello(); err != nil {
		t.Fatalf("SendHello with many areas: %v", err)
	}

	// Every entry the TLV carries must be a well-formed length-prefixed Area
	// Address that decodes (no truncated trailing entry).
	if _, err := packet.DecodeAreaAddressesTLV(tlv.Value); err != nil {
		t.Fatalf("built TLV 1 value does not decode: %v", err)
	}
}

// TestISISHelloPaddedToMTU: the constructed Hello is padded so the FRAMED Hello
// (LLC header + IS-IS PDU) fills the interface MTU exactly (AC-11). The PDU itself
// is padded to MTU - LLCHeaderLen, NOT to the raw MTU: the transport prepends an
// LLC header, so a PDU padded to the full MTU yields an oversized frame the kernel
// rejects (EMSGSIZE), silently dropping every Hello on a real socket. This test
// pins the correct, frame-aware pad target so that interop regression cannot
// return (it was a real ze bug found against FRR isisd).
func TestISISHelloPaddedToMTU(t *testing.T) {
	const mtu = 1497
	const wantPDU = mtu - transport.LLCHeaderLen // LLC + PDU must fit the link MTU
	s := &fakeSender{mtu: mtu}
	c := lanCircuit(t, s)
	if err := c.SendHello(); err != nil {
		t.Fatal(err)
	}
	last := s.sent[len(s.sent)-1]
	if len(last.pdu) != wantPDU {
		t.Fatalf("padded PDU length = %d, want %d (MTU %d minus LLC header %d)", len(last.pdu), wantPDU, mtu, transport.LLCHeaderLen)
	}
	// The framed Hello (LLC header + PDU) must not exceed the interface MTU, or the
	// kernel rejects the send on a real socket.
	if framed := transport.LLCHeaderLen + len(last.pdu); framed > mtu {
		t.Fatalf("framed Hello (LLC %d + PDU %d = %d) exceeds MTU %d", transport.LLCHeaderLen, len(last.pdu), framed, mtu)
	}
	// The PDU must still decode (padding is well-formed TLV 8s) and carry at
	// least one Padding TLV.
	p, err := packet.DecodePDU(last.pdu)
	if err != nil {
		t.Fatalf("padded PDU does not decode: %v", err)
	}
	if !hasTLV(p.LANHello.TLVs, packet.TLVPadding) {
		t.Error("padded LAN IIH carries no Padding TLV 8")
	}
}

// TestISISPadHelloNoMTU: with no known MTU the Hello is sent unpadded (still a
// valid PDU), never larger than the input.
func TestISISPadHelloNoMTU(t *testing.T) {
	pdu := []byte{1, 2, 3, 4}
	if got := padHello(pdu, 0); len(got) != len(pdu) {
		t.Errorf("padHello with mtu=0 changed length: %d -> %d", len(pdu), len(got))
	}
	if got := padHello(pdu, 2); len(got) != len(pdu) {
		t.Errorf("padHello with mtu<len changed length: %d -> %d", len(pdu), len(got))
	}
}
