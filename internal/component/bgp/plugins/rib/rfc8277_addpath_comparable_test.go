// Design: docs/architecture/plugin/rib-storage-design.md -- best-path candidates
// RFC: rfc/short/rfc8277.md -- RFC8277-3.1-1, comparability on one ADD-PATH session
// Related: rfc8277_test.go -- TestLabeledRoutesWithDifferentLabelsAreComparable, the two-session case

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC8277AddPathRoutesOnOneSessionAreComparable covers the second case of RFC 8277 Section 3.1: two UPDATEs "received on the same
// session, add-paths is used on that session, and the NLRIs of the two UPDATEs
// have different path identifiers. These two routes MUST be considered to be
// comparable, even if they specify different labels. Thus, the BGP best-path
// selection procedures (see Section 9.1 of [RFC4271]) are applied to select one
// of them as the better path."
//
// VALIDATES: one iBGP ADD-PATH peer sends 10.0.0.0/8 as path 7 (label 100,
// MED 20) and path 9 (label 200, MED 10); the candidates gathered for the
// prefix under either path's key hold both routes, and selection picks path 9's
// route on its MED. The session is iBGP and the AS_PATH empty, so both routes
// name the local AS as their neighbor AS and the MED step compares them
// (RFC 4271 Section 9.1.2.2 (c)). MED opposes the lowest-path-id tie-break,
// which would pick path 7, so only a selection that compares the two routes'
// MED passes.
// PREVENTS: best-path selection keyed on (path identifier, prefix), which never
// compares two paths of one session and elects one best route per identifier.
//
// Red before spec-bgp-addpath-best-path-per-prefix: gatherCandidatesLocked
// looked each peer up with the path-id-keyed NLRI, so each key yielded one
// candidate and path 7's key elected the MED 20 route.
//
// RFC requirement: RFC8277-3.1-1 positive -- two labeled paths of one prefix on one ADD-PATH session, under different path identifiers and labels, are both candidates of the one selection for the prefix, and the MED 10 path, path 9, is selected over path 7's MED 20 whichever path's key starts the lookup.
func TestRFC8277AddPathRoutesOnOneSessionAreComparable(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.31")
	ctxID, _ := bgpctx.Registry.Register(
		bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{labeledFamily: true}))

	pfx := netip.MustParsePrefix("10.0.0.0/8")
	feedReceivedIBGP(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 1}, 20,
		labeledNLRI(7, true, pfx, []uint32{100})))
	feedReceivedIBGP(r, peer, ctxID, labeledUpdateBody([4]byte{10, 0, 0, 2}, 10,
		labeledNLRI(9, true, pfx, []uint32{200})))
	require.Equal(t, 2, r.bgpPeers[peer].Len(), "precondition: two paths stored")

	for _, key := range [][]byte{{0, 0, 0, 7, 8, 10}, {0, 0, 0, 9, 8, 10}} {
		candidates := r.gatherFramedCandidates(labeledFamily, key, true)
		assert.Len(t, candidates, 2, "both paths of the session are candidates for 10.0.0.0/8 (key %x)", key)
		best := SelectBest(candidates)
		require.NotNil(t, best, "one of the two paths is selected (key %x)", key)
		assert.Equal(t, uint32(10), best.MED, "the MED 10 path wins (key %x)", key)
		assert.Equal(t, uint32(9), best.PathID, "path 9 wins on MED against the path-id tie-break (key %x)", key)
	}
}

// feedReceivedIBGP is feedReceived for an iBGP session: the peer and this
// speaker are both in AS 65000, so a route with an empty AS_PATH names the local
// AS as its neighbor AS and the MED step compares it (RFC 4271 Section
// 9.1.2.2 (c)).
func feedReceivedIBGP(r *RIBManager, peer netip.Addr, ctxID bgpctx.ContextID, body []byte) {
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, _ := wu.Attrs()
	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: peer.String(),
		PeerAS:      65000,
		LocalAS:     65000,
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   body,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	})
}
