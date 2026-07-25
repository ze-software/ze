// VALIDATES: spec-ospfv3-2-wire AC-5 -- each Link State Request entry round-trips
// the 16-bit LS Type (in a 32-bit slot with 2 leading reserved octets), the Link
// State ID, and the Advertising Router.
// PREVENTS: reading the LS Type from the wrong half of the request slot.

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3LSReqRoundTrip(t *testing.T) {
	want := LSReq{Requests: []LSRequestEntry{
		{Type: types.LSTypeRouter, LinkStateID: mustLSID(t, "0.0.0.0"), AdvertisingRouter: mustRouterID(t, "10.0.0.1")},
		{Type: types.LSTypeIntraAreaPrefix, LinkStateID: mustLSID(t, "0.0.0.5"), AdvertisingRouter: mustRouterID(t, "10.0.0.2")},
		// An unknown function code must still round-trip: a peer may request an LSA
		// type this router does not implement.
		{Type: types.LSType(0xa05a), LinkStateID: mustLSID(t, "1.2.3.4"), AdvertisingRouter: mustRouterID(t, "10.0.0.3")},
	}}
	p := Packet{Header: sampleHeader(t, PacketTypeLSReq), LSReq: &want}
	wire := encodePacket(t, p)

	// The two leading octets of each entry must be reserved (zero).
	off := CommonHeaderLen
	if wire[off] != 0 || wire[off+1] != 0 {
		t.Fatalf("request entry reserved octets not zero: %#x %#x", wire[off], wire[off+1])
	}

	got, err := DecodePacket(wire)
	if err != nil {
		t.Fatalf("DecodePacket lsreq: %v", err)
	}
	r := got.LSReq
	if r == nil || len(r.Requests) != len(want.Requests) {
		t.Fatalf("decoded requests = %+v, want %d entries", r, len(want.Requests))
	}
	for i := range want.Requests {
		if r.Requests[i] != want.Requests[i] {
			t.Fatalf("request[%d] = %+v, want %+v", i, r.Requests[i], want.Requests[i])
		}
	}
}
