// VALIDATES: RFC 8665 Section 3.2 on the reception path: the SRGB of a remote router is
// rebuilt from the SID/Label Range TLVs of its received RI Opaque LSA in the order they were
// advertised, so a SID index maps to the label the advertised order gives.
// PREVENTS: a receiver that sorts, reverses or otherwise reorders the received ranges before
// mapping an index, which the SRGB unit tests cannot see because they build the SRGB by hand.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC8665-3.2-6 positive -- two routers advertise the same two SID/Label
// Range TLVs (20000 size 5, 16000 size 5) in opposite orders in their received RI Opaque LSA;
// after the reception path (engine.srRemoteCapabilities, srDecodeRemoteCapabilities) each
// router's index 0, 4, 5 and 9 map to the literal labels its own advertised order gives:
// 20000, 20004, 16000, 16004 for the first and 16000, 16004, 20000, 20004 for the second.
//
// Goal: prove the receiver keeps the advertised range order end to end, not only inside
// sr.SRGB. Method: install both RI LSAs in the LSDB as received, read the engine's remote
// capability map, and check each index against a literal label.
func TestRFC8665ReceivedRangesMappedInAdvertisedOrder(t *testing.T) {
	eng, _ := newRedistEngine(t, extOrigCfg)
	forward := types.RouterID{2, 2, 2, 2}
	reversed := types.RouterID{3, 3, 3, 3}
	low := sr.LabelRange{Base: 16000, Size: 5}
	high := sr.LabelRange{Base: 20000, Size: 5}
	srOriginateTestOpaque(t, eng, forward, packet.RIOpaqueType, packet.EncodeRITLVs([]packet.RITLV{
		{Type: sr.V4TypeSRAlgorithm, Value: sr.EncodeAlgorithmValue([]uint8{0})},
		{Type: sr.V4TypeSRGB, Value: sr.EncodeRangeValue(high)},
		{Type: sr.V4TypeSRGB, Value: sr.EncodeRangeValue(low)},
	}))
	srOriginateTestOpaque(t, eng, reversed, packet.RIOpaqueType, packet.EncodeRITLVs([]packet.RITLV{
		{Type: sr.V4TypeSRAlgorithm, Value: sr.EncodeAlgorithmValue([]uint8{0})},
		{Type: sr.V4TypeSRGB, Value: sr.EncodeRangeValue(low)},
		{Type: sr.V4TypeSRGB, Value: sr.EncodeRangeValue(high)},
	}))

	caps, _ := eng.srRemoteCapabilities()
	cases := []struct {
		router types.RouterID
		index  uint32
		label  uint32
	}{
		{forward, 0, 20000}, {forward, 4, 20004}, {forward, 5, 16000}, {forward, 9, 16004},
		{reversed, 0, 16000}, {reversed, 4, 16004}, {reversed, 5, 20000}, {reversed, 9, 20004},
	}
	for _, c := range cases {
		srgb, ok := caps[c.router]
		if !ok {
			t.Fatalf("router %v: no SRGB decoded from its RI Opaque LSA", c.router)
		}
		label, ok := srgb.Label(c.index)
		if !ok {
			t.Fatalf("router %v: index %d maps to no label, want %d", c.router, c.index, c.label)
		}
		if label != c.label {
			t.Fatalf("router %v: index %d = label %d, want %d (advertised order)", c.router, c.index, label, c.label)
		}
	}
}
