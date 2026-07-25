// VALIDATES: spec-ospfv3-2-wire AC-6 -- the Link State Acknowledgment body
// round-trips a list of 20-octet LSA headers.
// PREVENTS: an LSAck mis-sized against the 20-octet OSPFv3 LSA header width.

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3LSAckRoundTrip(t *testing.T) {
	want := LSAck{Headers: []LSAHeader{
		sampleLSAHeader(t, types.LSTypeRouter, "0.0.0.1"),
		sampleLSAHeader(t, types.LSTypeIntraAreaPrefix, "0.0.0.9"),
	}}
	for i := range want.Headers {
		want.Headers[i].Length = LSAHeaderLen
		want.Headers[i].Checksum = 0x2222
	}
	p := Packet{Header: sampleHeader(t, PacketTypeLSAck), LSAck: &want}
	wire := encodePacket(t, p)

	wantLen := CommonHeaderLen + len(want.Headers)*LSAHeaderLen
	if len(wire) != wantLen {
		t.Fatalf("encoded length = %d, want %d", len(wire), wantLen)
	}

	got, err := DecodePacket(wire)
	if err != nil {
		t.Fatalf("DecodePacket lsack: %v", err)
	}
	a := got.LSAck
	if a == nil || len(a.Headers) != len(want.Headers) {
		t.Fatalf("decoded LSAck = %+v, want %d headers", a, len(want.Headers))
	}
	for i := range want.Headers {
		if a.Headers[i] != want.Headers[i] {
			t.Fatalf("header[%d] = %+v, want %+v", i, a.Headers[i], want.Headers[i])
		}
	}
}
