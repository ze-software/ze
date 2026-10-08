// Design: docs/architecture/wire/nlri.md -- labeled unicast withdrawal framing
// RFC: rfc/short/rfc8277.md -- RFC8277-2.4-1, the Compatibility field on receipt
package rib

import (
	"net/netip"
	"slices"
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
)

// labeledWithdrawNLRI builds one RFC 8277 Section 2.4 withdrawal NLRI for an
// IPv4 prefix: [Length = 24 + prefixBits][Compatibility(3)][prefix octets],
// behind a path identifier when addPath is set.
func labeledWithdrawNLRI(pathID uint32, addPath bool, prefix netip.Prefix, compatibility [3]byte) []byte {
	var out []byte
	if addPath {
		out = append(out, byte(pathID>>24), byte(pathID>>16), byte(pathID>>8), byte(pathID))
	}
	prefixBits := prefix.Bits()
	out = append(out, byte(24+prefixBits))
	out = append(out, compatibility[:]...)
	return append(out, prefix.Addr().AsSlice()[:(prefixBits+7)/8]...)
}

// labeledWithdrawCompatibilities are the Compatibility values the tests send.
// The S bit of the third octet is clear in the RECOMMENDED value and in zero,
// which a reader that walks a label stack to its bottom runs past.
var labeledWithdrawCompatibilities = []struct {
	name  string
	value [3]byte
}{
	{name: "recommended 0x800000", value: [3]byte{0x80, 0x00, 0x00}},
	{name: "zero", value: [3]byte{0x00, 0x00, 0x00}},
	{name: "label 999 bottom of stack", value: [3]byte{0x00, 0x3e, 0x71}},
	{name: "all ones", value: [3]byte{0xff, 0xff, 0xff}},
	{name: "S-set label 100", value: [3]byte{0x00, 0x06, 0x41}},
	{name: "arbitrary S-clear", value: [3]byte{0x12, 0x34, 0x50}},
}

// TestRFC8277LabeledWithdrawIgnoresCompatibility announces one labeled unicast
// route without ADD-PATH and withdraws it through the real ingest path, once
// for each Compatibility value.
//
// VALIDATES: whatever the Compatibility field holds, the withdrawal removes the
// route and its label binding, and the best-change publishes the withdrawal.
// PREVENTS: the withdrawal framed as an announcement, whose label stack walk
// read a Compatibility field with its S bit clear as one more label and ran
// past the NLRI, so the route stayed installed.
//
// RFC requirement: RFC8277-2.4-1 positive -- a labeled unicast withdrawal removes the route whatever its Compatibility field holds, 0x800000 and zero included, and the Loc-RIB publishes the withdrawal.
// RFC requirement: RFC8277-2.4-1 negative -- valid nonrecommended Compatibility values cannot retain IPv4/IPv6 labeled paths or remove a different prefix.
func TestRFC8277LabeledWithdrawIgnoresCompatibility(t *testing.T) {
	compatibilityRouteState(t, family.SAFIMPLSLabel, false)
	pfx := netip.MustParsePrefix("10.0.0.0/8")
	for _, tc := range labeledWithdrawCompatibilities {
		t.Run(tc.name, func(t *testing.T) {
			bus := newTestEventBus()
			r := newTestRIBManagerWithBus(bus)
			peer := netip.MustParseAddr("192.0.2.21")
			r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
			ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

			feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
				labeledNLRI(0, false, pfx, []uint32{100})))
			if got := r.bgpPeers[peer].Len(); got != 1 {
				t.Fatalf("routes after the announcement = %d, want 1", got)
			}

			feedReceived(r, peer, ctxID, labeledWithdrawBody(labeledWithdrawNLRI(0, false, pfx, tc.value)))
			if got := r.bgpPeers[peer].Len(); got != 0 {
				t.Fatalf("routes after the withdrawal = %d, want 0: the Compatibility field names no route", got)
			}
			if got := labelsFor(r, peer, []byte{8, 10}); got != nil {
				t.Fatalf("labels after the withdrawal = %v, want none", got)
			}
			changes := vpnBestChanges(bus, labeledFamily)
			if len(changes) != 2 {
				t.Fatalf("published %d best-changes, want 2 (add, withdraw)", len(changes))
			}
			if changes[1].Action != routeaction.Withdraw {
				t.Fatalf("second change action = %v, want withdraw", changes[1].Action)
			}
		})
	}
}

// TestRFC8277LabeledWithdrawAddPathIgnoresCompatibility announces two paths of
// one labeled prefix under ADD-PATH and withdraws one of them through the real
// ingest path, once for each Compatibility value.
//
// VALIDATES: the withdrawal removes exactly the path its identifier names,
// whatever the Compatibility field holds; the other path keeps its route and
// its own label.
// PREVENTS: the same label stack misreading as the test above, and a fix that
// strips the Compatibility field but loses the path identifier.
//
// RFC requirement: RFC8277-2.4-1 positive -- under ADD-PATH a labeled unicast withdrawal removes the path its identifier names whatever its Compatibility field holds, and the other path keeps its route and label.
// RFC requirement: RFC8277-2.4-1 negative -- Compatibility does not select labels or conflate IPv4/IPv6 ADD-PATH zero and 17; exact survivors and replacement labels remain visible.
func TestRFC8277LabeledWithdrawAddPathIgnoresCompatibility(t *testing.T) {
	compatibilityRouteState(t, family.SAFIMPLSLabel, true)
	pfx := netip.MustParsePrefix("10.0.0.0/8")
	for _, tc := range labeledWithdrawCompatibilities {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManager(t)
			peer := netip.MustParseAddr("192.0.2.22")
			ctxID, _ := bgpctx.Registry.Register(
				bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{labeledFamily: true}))

			feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
				labeledNLRI(7, true, pfx, []uint32{100})))
			feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 20,
				labeledNLRI(9, true, pfx, []uint32{200})))
			if got := r.bgpPeers[peer].Len(); got != 2 {
				t.Fatalf("routes after the announcements = %d, want 2", got)
			}

			feedReceived(r, peer, ctxID, labeledWithdrawBody(labeledWithdrawNLRI(7, true, pfx, tc.value)))
			if got := r.bgpPeers[peer].Len(); got != 1 {
				t.Fatalf("routes after withdrawing path 7 = %d, want 1", got)
			}
			if _, found := r.bgpPeers[peer].Lookup(labeledFamily, []byte{0, 0, 0, 7, 8, 10}); found {
				t.Fatalf("path 7 is still installed after its withdrawal")
			}
			if got := labelsFor(r, peer, []byte{0, 0, 0, 9, 8, 10}); !slices.Equal(got, []uint32{200}) {
				t.Fatalf("path 9 labels = %v, want [200]: the withdrawal named path 7 only", got)
			}
		})
	}
}
