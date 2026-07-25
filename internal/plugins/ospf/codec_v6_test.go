// VALIDATES: spec-ospf-af-unify -- the OSPFv3 v6Codec satisfies the engine Codec
// interface and projects the OSPFv3 16-byte common header onto the neutral Header
// (Instance ID surfaced, 32-bit Router/Area IDs), and delegates the IPv6 upper-layer
// checksum to ospfv3/packet. PREVENTS: the codec seam silently being v4-only -- this is
// the proof that the wire codec is pluggable per address family.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFCodecInterfaceV6Adapter(t *testing.T) {
	// A valid OSPFv3 common header on the wire (RouterID 1.1.1.1, backbone, Instance 7).
	src := ospfv3packet.Header{
		Type:       ospfv3packet.PacketTypeHello,
		Length:     ospfv3packet.CommonHeaderLen,
		RouterID:   ospfv3types.RouterID{1, 1, 1, 1},
		AreaID:     ospfv3types.AreaID{0, 0, 0, 0},
		InstanceID: 7,
	}
	buf := make([]byte, ospfv3packet.CommonHeaderLen)
	src.WriteTo(buf, 0)

	var c Codec = v6Codec{}
	got, err := c.DecodeHeader(buf)
	if err != nil {
		t.Fatalf("v6 DecodeHeader: %v", err)
	}
	if got.Type != PacketTypeHello {
		t.Errorf("type = %d, want PacketTypeHello (%d)", got.Type, PacketTypeHello)
	}
	if got.RouterID != (types.RouterID{1, 1, 1, 1}) {
		t.Errorf("router id = %v, want 1.1.1.1", got.RouterID)
	}
	if got.InstanceID != 7 {
		t.Errorf("instance id = %d, want 7 (OSPFv3 demux field)", got.InstanceID)
	}

	// VerifyChecksum must delegate to ospfv3/packet (bound to src/dst, RFC 5340 A.3.1).
	a := netip.MustParseAddr("fe80::1")
	b := netip.MustParseAddr("ff02::5")
	if c.VerifyChecksum(buf, a, b) != ospfv3packet.VerifyPacketChecksum(a, b, buf) {
		t.Fatalf("v6 VerifyChecksum disagrees with ospfv3packet.VerifyPacketChecksum")
	}
}

func TestOSPFCodecDecodeHelloV6(t *testing.T) {
	h := ospfv3packet.Hello{
		InterfaceID:        ospfv3types.InterfaceID(42),
		Priority:           1,
		Options:            ospfv3types.OptE | ospfv3types.OptN | ospfv3types.OptV6,
		HelloInterval:      10,
		RouterDeadInterval: 40,
		DR:                 ospfv3types.RouterID{10, 0, 0, 1},
		BDR:                ospfv3types.RouterID{10, 0, 0, 2},
		Neighbors:          []ospfv3types.RouterID{{2, 2, 2, 2}},
	}
	p := ospfv3packet.Packet{Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeHello, RouterID: ospfv3types.RouterID{1, 1, 1, 1}}, Hello: &h}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)

	got, err := v6Codec{}.DecodeHello(buf)
	if err != nil {
		t.Fatalf("DecodeHello: %v", err)
	}
	if got.InterfaceID != 42 {
		t.Errorf("InterfaceID = %d, want 42 (replaces the v2 Network Mask)", got.InterfaceID)
	}
	if got.NetworkMask != ([4]byte{}) {
		t.Errorf("v6 NetworkMask must be zero, got %v", got.NetworkMask)
	}
	if !got.Options.Has(types.OptionE) {
		t.Errorf("E-bit not mapped from OSPFv3 OptE")
	}
	if !got.Options.Has(types.OptionNP) {
		t.Errorf("N-bit not mapped from OSPFv3 OptN")
	}
	if got.HelloInterval != 10 || got.DeadInterval != 40 || got.Priority != 1 {
		t.Fatalf("decoded v6 Hello = %+v", got)
	}
	if got.DR != ([4]byte{10, 0, 0, 1}) || got.BDR != ([4]byte{10, 0, 0, 2}) {
		t.Errorf("DR/BDR = %v/%v, want 10.0.0.1/10.0.0.2 (OSPFv3 Router-ID DR/BDR)", got.DR, got.BDR)
	}
	if len(got.Neighbors) != 1 || got.Neighbors[0] != (types.RouterID{2, 2, 2, 2}) {
		t.Errorf("neighbors = %v, want [2.2.2.2]", got.Neighbors)
	}
}

func v6Hdr() ospfv3packet.LSAHeader {
	return ospfv3packet.LSAHeader{
		Age:               10,
		Type:              ospfv3types.LSType(0x2001), // Router-LSA (scope-typed 16-bit)
		LinkStateID:       ospfv3types.LinkStateID{0, 0, 0, 1},
		AdvertisingRouter: ospfv3types.RouterID{2, 2, 2, 2},
		Sequence:          ospfv3types.LSSequenceNumber(5),
		Checksum:          0xabcd,
		Length:            20,
	}
}

func assertNeutralHdr(t *testing.T, h types.LSAHeader) {
	t.Helper()
	if h.Age != types.LSAge(10) || h.Type != types.LSType(0x2001) ||
		h.LinkStateID != (types.LinkStateID{0, 0, 0, 1}) || h.AdvertisingRouter != (types.RouterID{2, 2, 2, 2}) ||
		uint32(h.Sequence) != 5 || h.Checksum != 0xabcd || h.Length != 20 {
		t.Fatalf("converted v6 LSA header = %+v", h)
	}
}

func TestOSPFCodecDecodeDBDescV6(t *testing.T) {
	dd := ospfv3packet.DBDesc{
		Options:      ospfv3types.OptE | ospfv3types.OptR,
		InterfaceMTU: 1500,
		Flags:        ospfv3packet.DDFlagInit | ospfv3packet.DDFlagMore | ospfv3packet.DDFlagMaster,
		DDSequence:   42,
		Headers:      []ospfv3packet.LSAHeader{v6Hdr()},
	}
	p := ospfv3packet.Packet{Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeDBDesc, RouterID: ospfv3types.RouterID{1, 1, 1, 1}}, DBDesc: &dd}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)

	got, err := v6Codec{}.DecodeDBDesc(buf)
	if err != nil {
		t.Fatalf("DecodeDBDesc: %v", err)
	}
	if got.InterfaceMTU != 1500 || got.DDSequence != 42 ||
		got.Flags != (ospfv3packet.DDFlagInit|ospfv3packet.DDFlagMore|ospfv3packet.DDFlagMaster) {
		t.Fatalf("decoded v6 DBDesc = %+v", got)
	}
	if !got.Options.Has(types.OptionE) {
		t.Errorf("E-bit not mapped from OSPFv3 OptE")
	}
	if len(got.Headers) != 1 {
		t.Fatalf("headers = %d, want 1", len(got.Headers))
	}
	assertNeutralHdr(t, got.Headers[0])
}

func TestOSPFCodecDecodeLSReqV6(t *testing.T) {
	r := ospfv3packet.LSReq{Requests: []ospfv3packet.LSRequestEntry{{
		Type:              ospfv3types.LSType(0x2001),
		LinkStateID:       ospfv3types.LinkStateID{0, 0, 0, 5},
		AdvertisingRouter: ospfv3types.RouterID{3, 3, 3, 3},
	}}}
	p := ospfv3packet.Packet{Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeLSReq, RouterID: ospfv3types.RouterID{1, 1, 1, 1}}, LSReq: &r}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)

	got, err := v6Codec{}.DecodeLSReq(buf)
	if err != nil {
		t.Fatalf("DecodeLSReq: %v", err)
	}
	if len(got.Requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(got.Requests))
	}
	e := got.Requests[0]
	if e.Type != types.LSType(0x2001) || e.LinkStateID != (types.LinkStateID{0, 0, 0, 5}) || e.AdvertisingRouter != (types.RouterID{3, 3, 3, 3}) {
		t.Fatalf("converted v6 LSReq entry = %+v", e)
	}
}

func TestOSPFCodecDecodeLSUpdateV6(t *testing.T) {
	// A real OSPFv3 Router-LSA so the encoder backfills a valid Length + Fletcher checksum.
	lsa := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age:               10,
			Type:              ospfv3types.LSTypeRouter,
			LinkStateID:       ospfv3types.LinkStateID{0, 0, 0, 1},
			AdvertisingRouter: ospfv3types.RouterID{2, 2, 2, 2},
			Sequence:          ospfv3types.InitialSequenceNumber,
		},
		Router: &ospfv3packet.RouterLSA{
			Flags:   ospfv3packet.RouterFlagB,
			Options: ospfv3types.OptV6 | ospfv3types.OptR,
			Links: []ospfv3packet.RouterLink{{
				Type:                ospfv3packet.RouterLinkTypeP2P,
				Metric:              10,
				InterfaceID:         ospfv3types.InterfaceID(1),
				NeighborInterfaceID: ospfv3types.InterfaceID(2),
				NeighborRouterID:    ospfv3types.RouterID{10, 0, 0, 2},
			}},
		},
	}
	u := ospfv3packet.LSUpdate{LSAs: []ospfv3packet.LSA{lsa}}
	p := ospfv3packet.Packet{Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeLSUpdate, RouterID: ospfv3types.RouterID{1, 1, 1, 1}}, LSUpdate: &u}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)

	got, err := v6Codec{}.DecodeLSUpdate(buf)
	if err != nil {
		t.Fatalf("DecodeLSUpdate: %v", err)
	}
	if len(got.LSAs) != 1 {
		t.Fatalf("LSAs = %d, want 1", len(got.LSAs))
	}
	nl := got.LSAs[0]
	assertNeutralHdr2(t, nl.Header)
	// The typed body stays undecoded: it is the AFPrefixStrategy boundary, not the codec's job.
	if nl.Router != nil || nl.Network != nil {
		t.Errorf("v6 LSUpdate codec must not eagerly decode the typed LSA body")
	}
	// The neutral packet.LSA carries the complete v6 LSA, so the OSPFv2 LSA Fletcher checksum
	// verifies it unchanged (RFC 5340 A.4.2.1: byte-identical to OSPFv2). This is the proof the
	// LSDB's existing packet.LSA.VerifyChecksum accepts v6 LSAs without an AF-aware checksum path.
	if !nl.VerifyChecksum() {
		t.Fatalf("neutral v6 LSA failed the OSPFv2 Fletcher checksum verify (RawBytes len=%d)", len(nl.RawBytes))
	}
}

// assertNeutralHdr2 checks the converted header of the encoder-built Router-LSA above
// (distinct from assertNeutralHdr, which checks the fixed v6Hdr() fixture).
func assertNeutralHdr2(t *testing.T, h types.LSAHeader) {
	t.Helper()
	if h.Type != types.LSType(0x2001) ||
		h.LinkStateID != (types.LinkStateID{0, 0, 0, 1}) ||
		h.AdvertisingRouter != (types.RouterID{2, 2, 2, 2}) {
		t.Fatalf("converted v6 LSA header = %+v", h)
	}
}

func TestOSPFCodecDecodeLSAckV6(t *testing.T) {
	a := ospfv3packet.LSAck{Headers: []ospfv3packet.LSAHeader{v6Hdr()}}
	p := ospfv3packet.Packet{Header: ospfv3packet.Header{Type: ospfv3packet.PacketTypeLSAck, RouterID: ospfv3types.RouterID{1, 1, 1, 1}}, LSAck: &a}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)

	got, err := v6Codec{}.DecodeLSAck(buf)
	if err != nil {
		t.Fatalf("DecodeLSAck: %v", err)
	}
	if len(got.Headers) != 1 {
		t.Fatalf("headers = %d, want 1", len(got.Headers))
	}
	assertNeutralHdr(t, got.Headers[0])
}
