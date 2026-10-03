// Design: docs/architecture/wire/nlri.md -- labeled unicast in the Adj-RIB-In
// RFC: rfc/short/rfc8277.md -- implicit withdrawal and best-path comparability
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

var labeledFamily = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}

// labeledNLRI builds one RFC 8277 Section 2.2/2.3 NLRI for an IPv4 prefix:
// [Length = 24*len(labels) + prefixBits][label entries][prefix octets].
// A path ID is prepended when pathID is non-zero-flagged by addPath.
func labeledNLRI(pathID uint32, addPath bool, prefix netip.Prefix, labels []uint32) []byte {
	var out []byte
	if addPath {
		out = append(out, byte(pathID>>24), byte(pathID>>16), byte(pathID>>8), byte(pathID))
	}
	prefixBits := prefix.Bits()
	prefixBytes := (prefixBits + 7) / 8
	out = append(out, byte(len(labels)*24+prefixBits))
	for i, l := range labels {
		last := byte(0)
		if i == len(labels)-1 {
			last = 1
		}
		out = append(out, byte(l>>12), byte(l>>4), byte(l<<4)&0xF0|last)
	}
	out = append(out, prefix.Addr().AsSlice()[:prefixBytes]...)
	return out
}

// labeledUpdateBody builds an UPDATE body announcing one labeled unicast NLRI
// via MP_REACH_NLRI (AFI 1 / SAFI 4) with ORIGIN, an empty AS_PATH and a MED.
func labeledUpdateBody(nextHop [4]byte, med uint32, nlriBytes []byte) []byte {
	mpValue := []byte{0x00, 0x01, 0x04, 0x04, nextHop[0], nextHop[1], nextHop[2], nextHop[3], 0x00}
	mpValue = append(mpValue, nlriBytes...)

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x80, 0x04, 0x04, byte(med >> 24), byte(med >> 16), byte(med >> 8), byte(med), // MED
	}
	attrs = append(attrs, 0x80, 0x0e, byte(len(mpValue))) //nolint:gosec // test NLRI is short
	attrs = append(attrs, mpValue...)

	body := []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))} //nolint:gosec // test attrs are short
	return append(body, attrs...)
}

// labeledWithdrawBody builds an UPDATE body withdrawing one labeled unicast
// NLRI via MP_UNREACH_NLRI (AFI 1 / SAFI 4).
func labeledWithdrawBody(nlriBytes []byte) []byte {
	mpValue := []byte{0x00, 0x01, 0x04}
	mpValue = append(mpValue, nlriBytes...)

	attrs := []byte{0x80, 0x0f, byte(len(mpValue))} //nolint:gosec // test NLRI is short
	attrs = append(attrs, mpValue...)

	body := []byte{0x00, 0x00, byte(len(attrs) >> 8), byte(len(attrs))} //nolint:gosec // test attrs are short
	return append(body, attrs...)
}

// labelsFor reads back the MPLS label stack a peer's Adj-RIB-In holds for a
// labeled unicast prefix. cidrBytes is the label-stripped NLRI key.
func labelsFor(r *RIBManager, peer netip.Addr, cidrBytes []byte) []uint32 {
	peerRIB := r.bgpPeers[peer]
	if peerRIB == nil {
		return nil
	}
	return pool.ResolveLabels(peerRIB.LookupLabels(labeledFamily, cidrBytes))
}

// TestLabeledImplicitWithdrawalNoAddPath pins RFC 8277 Section 2.5 for a
// session without ADD-PATH: a second UPDATE binding a different label to a
// prefix already advertised by that peer implicitly withdraws the first
// binding. The Adj-RIB-In keys labeled routes on the label-stripped prefix and
// carries the stack as side-data, so the second UPDATE replaces both.
//
// VALIDATES: insertLabeled + FamilyRIB.Insert replace the (prefix) entry and
// its label side-data, leaving exactly one binding for the prefix.
// PREVENTS: a relabel accumulating two bindings for one prefix, so the FIB
// keeps pushing a label the advertising router no longer honors.
//
// RFC requirement: RFC8277-2.5-1 positive -- without ADD-PATH, a second UPDATE for the same prefix implicitly withdraws the first binding; only the new label survives.
// RFC requirement: RFC8277-2.5-1 negative -- an UPDATE for a DIFFERENT prefix must not be read as an implicit withdrawal: the first binding is untouched.
func TestLabeledImplicitWithdrawalNoAddPath(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.11")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	pfx := netip.MustParsePrefix("10.0.0.0/8")
	cidr := []byte{8, 10}

	feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
		labeledNLRI(0, false, pfx, []uint32{100})))
	require.Equal(t, 1, r.bgpPeers[peer].Len(), "U1 installs the labeled route")
	require.Equal(t, []uint32{100}, labelsFor(r, peer, cidr))

	// U2: same prefix, same peer, new label. Section 2.5 makes this an
	// implicit withdrawal of U1, not a second binding.
	feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
		labeledNLRI(0, false, pfx, []uint32{200})))
	assert.Equal(t, 1, r.bgpPeers[peer].Len(),
		"U2 implicitly withdraws U1: one binding for the prefix, not two")
	assert.Equal(t, []uint32{200}, labelsFor(r, peer, cidr),
		"the new label replaces the old one at the same next hop")

	// Negative: an UPDATE for a different prefix carries no implicit
	// withdrawal of the first binding.
	other := netip.MustParsePrefix("10.1.0.0/16")
	feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
		labeledNLRI(0, false, other, []uint32{300})))
	assert.Equal(t, 2, r.bgpPeers[peer].Len(),
		"a different prefix must not implicitly withdraw the earlier binding")
	assert.Equal(t, []uint32{200}, labelsFor(r, peer, cidr),
		"the earlier prefix keeps its label")
	assert.Equal(t, []uint32{300}, labelsFor(r, peer, []byte{16, 10, 1}))
}

// TestLabeledImplicitWithdrawalAddPath pins RFC 8277 Section 2.5 for a session
// where ADD-PATH (RFC 7911) is in use: the implicit withdrawal is keyed on the
// Path Identifier. Re-advertising the SAME path ID replaces the binding; a
// DIFFERENT path ID adds a second binding and withdraws nothing.
//
// VALIDATES: the labeled Adj-RIB-In keys on (path-id, prefix) under ADD-PATH
// so per-path label bindings are independent.
// PREVENTS: a second labeled path silently destroying the first (losing an
// alternate MPLS path), or a relabel of one path leaking a stale binding.
//
// RFC requirement: RFC8277-2.5-2 positive -- with ADD-PATH and the same Path Identifier, U2 implicitly withdraws U1: the path keeps one route entry and carries U2's label.
// RFC requirement: RFC8277-2.5-2 negative -- a different Path Identifier does not trigger the implicit withdrawal: the earlier path's route entry is still present.
func TestLabeledImplicitWithdrawalAddPath(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.12")
	ctxID, _ := bgpctx.Registry.Register(
		bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{labeledFamily: true}))

	pfx := netip.MustParsePrefix("10.0.0.0/8")
	cidr7 := []byte{0, 0, 0, 7, 8, 10}
	cidr9 := []byte{0, 0, 0, 9, 8, 10}

	feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
		labeledNLRI(7, true, pfx, []uint32{100})))
	require.Equal(t, 1, r.bgpPeers[peer].Len())
	require.Equal(t, []uint32{100}, labelsFor(r, peer, cidr7))

	// Negative: a different path ID is a NEW route entry; path 7's entry stays.
	feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
		labeledNLRI(9, true, pfx, []uint32{200})))
	assert.Equal(t, 2, r.bgpPeers[peer].Len(),
		"different Path Identifiers are separate routes for the same prefix")
	_, found7 := r.bgpPeers[peer].Lookup(labeledFamily, cidr7)
	assert.True(t, found7, "path 7 must not be withdrawn by an UPDATE for path 9")

	// Positive: the same path ID is an implicit withdrawal of the earlier
	// UPDATE for that path -- no third entry appears and the label is replaced.
	feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
		labeledNLRI(9, true, pfx, []uint32{300})))
	assert.Equal(t, 2, r.bgpPeers[peer].Len(),
		"re-advertising path 9 replaces it rather than adding a third route")
	assert.Equal(t, []uint32{300}, labelsFor(r, peer, cidr9),
		"the same Path Identifier makes U2 an implicit withdrawal of U1")

	// An explicit MP_UNREACH for path 9 removes exactly that path's entry.
	feedReceived(r, peer, ctxID, labeledWithdrawBody(labeledNLRI(9, true, pfx, []uint32{300})))
	assert.Equal(t, 1, r.bgpPeers[peer].Len())
	_, stillFound7 := r.bgpPeers[peer].Lookup(labeledFamily, cidr7)
	assert.True(t, stillFound7, "withdrawing path 9 leaves path 7 installed")
}

// TestRFC8277AddPathLabelsBoundPerPath pins RFC 8277 Section 2.5 for the label
// bindings of an ADD-PATH session. "If I1 is not the same as I2, U2 MUST be
// interpreted as meaning that L2 is now bound to P at N1, but U2 MUST NOT be
// interpreted as meaning that L1 is no longer bound to P at N1." And "If I1 is
// the same as I2, UPDATE U2 MUST be interpreted as meaning that L2 is now bound
// to P at N1 and that L1 is no longer bound to P at N1."
//
// VALIDATES: AC-4: path 7 binds label 100 and path 9 label 200; path 9 is
// re-advertised with label 300; path 7 still reads 100 and path 9 reads 300.
// Withdrawing path 9 leaves path 7's label in place and path 9 with none.
// PREVENTS: label side-data keyed on the prefix alone, where the second path's
// UPDATE overwrote the first path's binding and withdrawing either path
// deleted the shared binding. That was the RFC8277-2.5-3 gap this test
// replaces the pin of.
//
// RFC requirement: RFC8277-2.5-3 positive -- on one ADD-PATH session, an UPDATE binding label 200 to 10.0.0.0/8 under path 9 leaves label 100 bound under path 7, and so do a relabel and a withdrawal of path 9.
// RFC requirement: RFC8277-2.5-2 positive -- on one ADD-PATH session, re-advertising 10.0.0.0/8 under path 9 with label 300 leaves path 9 bound to 300 alone, never to its earlier label 200.
func TestRFC8277AddPathLabelsBoundPerPath(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.13")
	ctxID, _ := bgpctx.Registry.Register(
		bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{labeledFamily: true}))

	pfx := netip.MustParsePrefix("10.0.0.0/8")
	cidr7 := []byte{0, 0, 0, 7, 8, 10}
	cidr9 := []byte{0, 0, 0, 9, 8, 10}

	feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
		labeledNLRI(7, true, pfx, []uint32{100})))
	require.Equal(t, []uint32{100}, labelsFor(r, peer, cidr7))

	feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 2}, 20,
		labeledNLRI(9, true, pfx, []uint32{200})))
	assert.Equal(t, []uint32{100}, labelsFor(r, peer, cidr7),
		"a different Path Identifier must not unbind path 7's label")
	assert.Equal(t, []uint32{200}, labelsFor(r, peer, cidr9), "path 9 binds its own label")

	feedReceived(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 2}, 20,
		labeledNLRI(9, true, pfx, []uint32{300})))
	assert.Equal(t, []uint32{300}, labelsFor(r, peer, cidr9),
		"the same Path Identifier binds the new label and unbinds the old one")
	assert.Equal(t, []uint32{100}, labelsFor(r, peer, cidr7),
		"relabelling path 9 must not touch path 7's label")

	feedReceived(r, peer, ctxID, labeledWithdrawBody(labeledNLRI(9, true, pfx, []uint32{300})))
	assert.Equal(t, []uint32{100}, labelsFor(r, peer, cidr7),
		"withdrawing path 9 leaves path 7's label bound")
	assert.Nil(t, labelsFor(r, peer, cidr9), "the withdrawn path holds no label")
}

// TestLabeledRoutesWithDifferentLabelsAreComparable pins RFC 8277 Section 3.1:
// two routes for the same prefix that differ only in the label they carry are
// COMPARABLE, so ordinary best-path selection runs over both. ze strips the
// label stack into side-data before keying the RIB, so both peers' routes land
// on the same prefix key and both appear as candidates; the label itself is
// never a selection input.
//
// VALIDATES: gatherCandidates returns both peers' routes for one labeled
// prefix, and SelectBest picks by MED regardless of the label values.
// PREVENTS: labels splitting one prefix into two independent best paths (which
// would install both and break MPLS forwarding), or a label value tie-breaking
// best-path selection.
//
// RFC requirement: RFC8277-3.1-1 positive -- two routes for the same prefix with different labels are gathered as candidates for a single best-path selection.
// RFC requirement: RFC8277-3.1-1 negative -- the label value is not a selection input: swapping the two labels leaves the same peer winning, and no second best path appears.
func TestLabeledRoutesWithDifferentLabelsAreComparable(t *testing.T) {
	r := newTestRIBManager(t)
	peerA := netip.MustParseAddr("192.0.2.21")
	peerB := netip.MustParseAddr("192.0.2.22")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	pfx := netip.MustParsePrefix("10.0.0.0/8")
	cidr := []byte{8, 10}

	// Peer A: label 100, MED 10 (the better MED). Peer B: label 200, MED 20.
	feedReceived(r, peerA, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
		labeledNLRI(0, false, pfx, []uint32{100})))
	feedReceived(r, peerB, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 2}, 20,
		labeledNLRI(0, false, pfx, []uint32{200})))

	candidates := r.gatherCandidates(labeledFamily, cidr)
	require.Len(t, candidates, 2,
		"routes with different labels for the same prefix are compared against each other")

	best := SelectBest(candidates)
	require.NotNil(t, best)
	assert.Equal(t, peerA.String(), best.PeerAddr, "the lower MED wins")

	// Negative: swap the labels only. Nothing about the selection may move,
	// which is what "comparable" means -- the label is not a tie-break.
	feedReceived(r, peerA, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 10,
		labeledNLRI(0, false, pfx, []uint32{200})))
	feedReceived(r, peerB, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 2}, 20,
		labeledNLRI(0, false, pfx, []uint32{100})))

	swapped := r.gatherCandidates(labeledFamily, cidr)
	require.Len(t, swapped, 2, "swapping labels must not make the routes incomparable")
	bestSwapped := SelectBest(swapped)
	require.NotNil(t, bestSwapped)
	assert.Equal(t, peerA.String(), bestSwapped.PeerAddr,
		"the label value must not change which route wins")
	assert.Equal(t, []uint32{200}, labelsFor(r, peerA, cidr),
		"the winner's own label follows the winning route")
}
