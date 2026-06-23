// VALIDATES: spec-ospfv3-2-wire AC-6, AC-18 -- the Link State Update body
// round-trips the LSA count and the LSA list, and an over-long count is rejected
// without panic.
// PREVENTS: a crafted count pre-allocating or over-reading past the body.

package packet

import (
	"testing"
)

func TestOSPFv3LSUpdateRoundTrip(t *testing.T) {
	want := LSUpdate{LSAs: []LSA{sampleRouterLSA(t), sampleRouterLSA(t)}}
	p := Packet{Header: sampleHeader(t, PacketTypeLSUpdate), LSUpdate: &want}
	wire := encodePacket(t, p)

	// The 4-octet count must equal the number of LSAs.
	if got := readUint32(wire, CommonHeaderLen); got != 2 {
		t.Fatalf("encoded LSA count = %d, want 2", got)
	}

	got, err := DecodePacket(wire)
	if err != nil {
		t.Fatalf("DecodePacket lsupdate: %v", err)
	}
	u := got.LSUpdate
	if u == nil || len(u.LSAs) != 2 {
		t.Fatalf("decoded LSUpdate = %+v, want 2 LSAs", u)
	}
	for i := range u.LSAs {
		if u.LSAs[i].Header.Key() != want.LSAs[i].Header.Key() {
			t.Fatalf("LSA[%d] key = %+v, want %+v", i, u.LSAs[i].Header.Key(), want.LSAs[i].Header.Key())
		}
	}
}

func TestOSPFv3LSUpdateRejectsOverLongCount(t *testing.T) {
	want := LSUpdate{LSAs: []LSA{sampleRouterLSA(t)}}
	p := Packet{Header: sampleHeader(t, PacketTypeLSUpdate), LSUpdate: &want}
	wire := encodePacket(t, p)
	// Claim 1000 LSAs while only one is present.
	writeUint32(wire, CommonHeaderLen, 1000)
	if _, err := DecodePacket(wire); err == nil {
		t.Fatalf("DecodePacket accepted an over-long LSA count")
	}
}

func TestOSPFv3LSUpdateRejectsCountMismatch(t *testing.T) {
	want := LSUpdate{LSAs: []LSA{sampleRouterLSA(t), sampleRouterLSA(t)}}
	p := Packet{Header: sampleHeader(t, PacketTypeLSUpdate), LSUpdate: &want}
	wire := encodePacket(t, p)
	// Two LSAs are present but the count claims one: the iterator consumes both,
	// so the decoded count will not match.
	writeUint32(wire, CommonHeaderLen, 1)
	if _, err := DecodePacket(wire); err == nil {
		t.Fatalf("DecodePacket accepted a count that does not match the LSA list")
	}
}
