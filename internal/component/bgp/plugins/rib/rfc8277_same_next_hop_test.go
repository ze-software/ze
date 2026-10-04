// Design: docs/architecture/plugin/rib-storage-design.md -- ADD-PATH label bindings.
// Related: rfc8277_test.go -- labeled ingest fixtures and different-next-hop coverage.

package rib

import (
	"net/netip"
	"slices"
	"testing"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC8277AddPathSameNextHopKeepsIndependentLabels sends two labeled paths
// through ingest with one peer, prefix, family and next hop, changing only the
// path identifier and label. It then relabels and withdraws the second path.
//
// RFC 8277 Section 2.5: "If I1 is not the same as I2, U2 MUST be interpreted as
// meaning that L2 is now bound to P at N1, but U2 MUST NOT be interpreted as
// meaning that L1 is no longer bound to P at N1."
//
// VALIDATES: both labels remain bound at the same next hop under different
// identifiers; reusing an identifier replaces only that path's label, and a
// withdrawal removes only the path it names.
// PREVENTS: an implementation preserving labels only when next hops differ,
// sharing one label binding per next hop, or stacking a replacement label beside
// the obsolete label.
// MUTATION: binding every update to the first path in pathSet.setLabels loses
// the first path's label when the second identifier arrives; retaining the old
// handle there instead leaves label 200 after the same-id update binds 300.
//
// RFC requirement: RFC8277-2.5-3 positive -- on one ADD-PATH session with the same prefix and next hop, path 9 binding label 200 preserves path 7's label 100; subsequent relabel and withdrawal of path 9 leave path 7 intact.
// RFC requirement: RFC8277-2.5-2 positive -- re-advertising path 9 at the same next hop replaces label 200 with label 300 alone, retaining exactly two routes and path 7's label 100.
// RFC requirement: RFC8277-2.5-2 negative -- a new identifier at the same next hop leaves the earlier route and label installed; two paths remain.
func TestRFC8277AddPathSameNextHopKeepsIndependentLabels(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.15")
	ctxID, err := bgpctx.Registry.Register(
		bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{labeledFamily: true}))
	if err != nil {
		t.Fatalf("register ADD-PATH context: %v", err)
	}

	prefix := netip.MustParsePrefix("10.0.0.0/8")
	nextHop := [4]byte{10, 0, 0, 1}
	path7 := []byte{0, 0, 0, 7, 8, 10}
	path9 := []byte{0, 0, 0, 9, 8, 10}

	// RFC 8277 Section 2.5: the next hop stays N1 for every announcement.
	feedReceivedIBGP(r, peer, ctxID, labeledUpdateBody(nextHop, 10,
		labeledNLRI(7, true, prefix, []uint32{100})))
	if got := r.bgpPeers[peer].FamilyLen(labeledFamily); got != 1 {
		t.Fatalf("paths after U1 = %d, want 1", got)
	}
	if got := labelsFor(r, peer, path7); !slices.Equal(got, []uint32{100}) {
		t.Fatalf("path 7 labels after U1 = %v, want [100]", got)
	}

	feedReceivedIBGP(r, peer, ctxID, labeledUpdateBody(nextHop, 10,
		labeledNLRI(9, true, prefix, []uint32{200})))
	if got := r.bgpPeers[peer].FamilyLen(labeledFamily); got != 2 {
		t.Fatalf("paths after a different identifier = %d, want 2", got)
	}
	if got := labelsFor(r, peer, path7); !slices.Equal(got, []uint32{100}) {
		t.Fatalf("path 7 labels after path 9 arrives at the same next hop = %v, want [100]", got)
	}
	if got := labelsFor(r, peer, path9); !slices.Equal(got, []uint32{200}) {
		t.Fatalf("path 9 labels = %v, want [200]", got)
	}

	// RFC 8277 Section 2.5: "UPDATE U1 is implicitly withdrawn."
	feedReceivedIBGP(r, peer, ctxID, labeledUpdateBody(nextHop, 10,
		labeledNLRI(9, true, prefix, []uint32{300})))
	if got := r.bgpPeers[peer].FamilyLen(labeledFamily); got != 2 {
		t.Fatalf("paths after reusing path 9 = %d, want 2", got)
	}
	if got := labelsFor(r, peer, path7); !slices.Equal(got, []uint32{100}) {
		t.Fatalf("path 7 labels after path 9 is relabeled = %v, want [100]", got)
	}
	if got := labelsFor(r, peer, path9); !slices.Equal(got, []uint32{300}) {
		t.Fatalf("path 9 labels after replacement = %v, want [300] alone", got)
	}

	// RFC 8277 Section 2.4: the path identifier, not Compatibility, names the path.
	feedReceivedIBGP(r, peer, ctxID, labeledWithdrawBody(
		labeledWithdrawNLRI(9, true, prefix, [3]byte{0x80, 0, 0})))
	if got := r.bgpPeers[peer].FamilyLen(labeledFamily); got != 1 {
		t.Fatalf("paths after withdrawing path 9 = %d, want 1", got)
	}
	if _, found := r.bgpPeers[peer].Lookup(labeledFamily, path9); found {
		t.Fatal("withdrawn path 9 is still stored")
	}
	if got := labelsFor(r, peer, path9); len(got) != 0 {
		t.Fatalf("withdrawn path 9 labels = %v, want none", got)
	}
	if _, found := r.bgpPeers[peer].Lookup(labeledFamily, path7); !found {
		t.Fatal("withdrawing path 9 removed path 7")
	}
	if got := labelsFor(r, peer, path7); !slices.Equal(got, []uint32{100}) {
		t.Fatalf("path 7 labels after withdrawing path 9 = %v, want [100]", got)
	}
}
