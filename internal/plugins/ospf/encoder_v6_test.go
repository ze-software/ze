// VALIDATES: spec-ospf-af-unify Phase 5 -- the OSPFv3 Hello encoder (v6Encoder)
// produces wire bytes the v6 codec decodes back to the same neutral Hello, proving
// the v6 send path is symmetric with the v6 receive path (InstanceID, InterfaceID,
// Options, DR/BDR-as-RouterID, neighbors). PREVENTS: a v6 send path that encodes a
// Hello FRR/the v6 codec cannot parse.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
)

// TestAFBitInHelloAndDD pins AC-4/R-8: a multi-AF instance sets the RFC 5838 §2.4 AF-bit in
// BOTH Hello and DD Options, and a lone default instance (emitAF=false) sets it in neither
// (preserving the IPv6-base wire bytes, AC-11).
func TestAFBitInHelloAndDD(t *testing.T) {
	h := packet.Hello{InterfaceID: 1, HelloInterval: 10, DeadInterval: 40}
	dd := packet.DBDesc{InterfaceMTU: 1500, Flags: 0x07, DDSequence: 1}

	// emitAF=true -> AF-bit set in Hello and DD.
	helloWith := v6Encoder{instanceID: 64, emitAF: true}.EncodeHello(types.RouterID{1, 1, 1, 1}, types.AreaID{}, h)
	got, err := v6Codec{}.DecodeHello(helloWith)
	if err != nil || !got.AFBit {
		t.Fatalf("Hello with emitAF: AFBit=%v err=%v, want AF-bit set", got.AFBit, err)
	}
	ddWith := v6Encoder{instanceID: 64, emitAF: true}.EncodeDBDesc(types.RouterID{1, 1, 1, 1}, types.AreaID{}, dd)
	if !ddAFBit(t, ddWith) {
		t.Error("DD with emitAF: AF-bit not set in DD Options")
	}

	// emitAF=false -> AF-bit absent in both (default lone IPv6-unicast instance).
	helloNo := v6Encoder{instanceID: 0, emitAF: false}.EncodeHello(types.RouterID{1, 1, 1, 1}, types.AreaID{}, h)
	gotNo, _ := v6Codec{}.DecodeHello(helloNo)
	if gotNo.AFBit {
		t.Error("Hello without emitAF: AF-bit wrongly set")
	}
	ddNo := v6Encoder{instanceID: 0, emitAF: false}.EncodeDBDesc(types.RouterID{1, 1, 1, 1}, types.AreaID{}, dd)
	if ddAFBit(t, ddNo) {
		t.Error("DD without emitAF: AF-bit wrongly set")
	}
}

// ddAFBit decodes an OSPFv3 DD packet and reports its AF-bit.
func ddAFBit(t *testing.T, buf []byte) bool {
	t.Helper()
	p, err := ospfv3packet.DecodePacket(buf)
	if err != nil || p.DBDesc == nil {
		t.Fatalf("DecodePacket(DD): %v", err)
	}
	return p.DBDesc.Options.AF()
}

func TestOSPFv6EncodeHelloRoundTrip(t *testing.T) {
	var opts types.Options
	opts = opts.Set(types.OptionE)
	h := packet.Hello{
		InterfaceID:   42,
		Priority:      1,
		Options:       opts,
		HelloInterval: 10,
		DeadInterval:  40,
		DR:            [4]byte{10, 0, 0, 1},
		BDR:           [4]byte{10, 0, 0, 2},
		Neighbors:     []types.RouterID{{2, 2, 2, 2}},
	}

	buf := v6Encoder{instanceID: 7}.EncodeHello(types.RouterID{1, 1, 1, 1}, types.AreaID{}, h)

	// The common header round-trips with the Instance ID surfaced.
	hdr, err := v6Codec{}.DecodeHeader(buf)
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	if hdr.Type != PacketTypeHello || hdr.InstanceID != 7 || hdr.RouterID != (types.RouterID{1, 1, 1, 1}) {
		t.Fatalf("decoded header = %+v", hdr)
	}

	// The Hello body round-trips through the v6 codec.
	got, err := v6Codec{}.DecodeHello(buf)
	if err != nil {
		t.Fatalf("DecodeHello: %v", err)
	}
	if got.InterfaceID != 42 {
		t.Errorf("InterfaceID = %d, want 42", got.InterfaceID)
	}
	if got.HelloInterval != 10 || got.DeadInterval != 40 || got.Priority != 1 {
		t.Errorf("decoded v6 Hello = %+v", got)
	}
	if got.DR != ([4]byte{10, 0, 0, 1}) || got.BDR != ([4]byte{10, 0, 0, 2}) {
		t.Errorf("DR/BDR = %v/%v, want 10.0.0.1/10.0.0.2", got.DR, got.BDR)
	}
	if len(got.Neighbors) != 1 || got.Neighbors[0] != (types.RouterID{2, 2, 2, 2}) {
		t.Errorf("neighbors = %v, want [2.2.2.2]", got.Neighbors)
	}
	if !got.Options.Has(types.OptionE) {
		t.Errorf("E-bit not round-tripped through OSPFv3 Options")
	}
}

func neutralHdr() types.LSAHeader {
	return types.LSAHeader{
		Age:               10,
		Type:              types.LSType(0x2001),
		LinkStateID:       types.LinkStateID{0, 0, 0, 1},
		AdvertisingRouter: types.RouterID{2, 2, 2, 2},
		Sequence:          types.LSSequenceNumber(5),
		Checksum:          0xabcd,
		Length:            20,
	}
}

func TestOSPFv6EncodeDBDescRoundTrip(t *testing.T) {
	var opts types.Options
	opts = opts.Set(types.OptionE)
	dd := packet.DBDesc{
		Options:      opts,
		InterfaceMTU: 1500,
		Flags:        0x07,
		DDSequence:   42,
		Headers:      []types.LSAHeader{neutralHdr()},
	}
	buf := v6Encoder{instanceID: 3}.EncodeDBDesc(types.RouterID{1, 1, 1, 1}, types.AreaID{}, dd)

	hdr, err := v6Codec{}.DecodeHeader(buf)
	if err != nil || hdr.Type != PacketTypeDBDesc || hdr.InstanceID != 3 {
		t.Fatalf("header = %+v err=%v", hdr, err)
	}
	got, err := v6Codec{}.DecodeDBDesc(buf)
	if err != nil {
		t.Fatalf("DecodeDBDesc: %v", err)
	}
	if got.InterfaceMTU != 1500 || got.DDSequence != 42 || got.Flags != 0x07 {
		t.Fatalf("decoded DBDesc = %+v", got)
	}
	if !got.Options.Has(types.OptionE) {
		t.Errorf("E-bit not round-tripped")
	}
	if len(got.Headers) != 1 || got.Headers[0] != neutralHdr() {
		t.Fatalf("LSA header round-trip = %+v, want %+v", got.Headers, neutralHdr())
	}
}

func TestOSPFv6EncodeLSReqRoundTrip(t *testing.T) {
	r := packet.LSReq{Requests: []types.LSRequestEntry{{
		Type:              types.LSType(0x2001),
		LinkStateID:       types.LinkStateID{0, 0, 0, 5},
		AdvertisingRouter: types.RouterID{3, 3, 3, 3},
	}}}
	buf := v6Encoder{instanceID: 0}.EncodeLSReq(types.RouterID{1, 1, 1, 1}, types.AreaID{}, r)

	got, err := v6Codec{}.DecodeLSReq(buf)
	if err != nil {
		t.Fatalf("DecodeLSReq: %v", err)
	}
	if len(got.Requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(got.Requests))
	}
	e := got.Requests[0]
	if e.Type != types.LSType(0x2001) || e.LinkStateID != (types.LinkStateID{0, 0, 0, 5}) || e.AdvertisingRouter != (types.RouterID{3, 3, 3, 3}) {
		t.Fatalf("LSReq entry round-trip = %+v", e)
	}
}
