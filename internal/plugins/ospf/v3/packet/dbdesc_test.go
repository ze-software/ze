// VALIDATES: spec-ospfv3-2-wire AC-4 -- the Database Description body round-trips
// the 24-bit Options, Interface MTU, the I/M/MS flags, and the DD sequence in the
// 12-octet fixed layout, and the trailing LSA headers iterate.
// PREVENTS: the OSPFv2 8-octet DD prefix or an 8-bit Options leaking into OSPFv3.

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3DBDescRoundTrip(t *testing.T) {
	want := DBDesc{
		Options:      mustOptions(t, uint32(types.OptV6|types.OptE)),
		InterfaceMTU: 1500,
		Flags:        DDFlagInit | DDFlagMore | DDFlagMaster,
		DDSequence:   0xdeadbeef,
		Headers: []LSAHeader{
			sampleLSAHeader(t, types.LSTypeRouter, "0.0.0.1"),
			sampleLSAHeader(t, types.LSTypeNetwork, "0.0.0.2"),
		},
	}
	// LSA header round-trip needs the Length and Checksum to be encodable; set them.
	for i := range want.Headers {
		want.Headers[i].Length = LSAHeaderLen
		want.Headers[i].Checksum = 0x1111
	}
	p := Packet{Header: sampleHeader(t, PacketTypeDBDesc), DBDesc: &want}
	wire := encodePacket(t, p)

	got, err := DecodePacket(wire)
	if err != nil {
		t.Fatalf("DecodePacket dbdesc: %v", err)
	}
	d := got.DBDesc
	if d == nil {
		t.Fatalf("decoded packet has no DBDesc body")
	}
	if d.Options != want.Options || d.InterfaceMTU != want.InterfaceMTU || d.Flags != want.Flags || d.DDSequence != want.DDSequence {
		t.Fatalf("DBDesc scalars: got %+v want %+v", d, want)
	}
	if len(d.Headers) != len(want.Headers) {
		t.Fatalf("header count = %d, want %d", len(d.Headers), len(want.Headers))
	}
	for i := range want.Headers {
		if d.Headers[i] != want.Headers[i] || d.Headers[i].Key() != want.Headers[i].Key() {
			t.Fatalf("header[%d] = %+v, want %+v", i, d.Headers[i], want.Headers[i])
		}
	}
	// 12-octet fixed prefix, not the OSPFv2 8 octets.
	wantLen := CommonHeaderLen + dbDescFixedLen + len(want.Headers)*LSAHeaderLen
	if len(wire) != wantLen {
		t.Fatalf("encoded length = %d, want %d", len(wire), wantLen)
	}
	if dbDescFixedLen != 12 {
		t.Fatalf("dbDescFixedLen = %d, want 12", dbDescFixedLen)
	}
}
