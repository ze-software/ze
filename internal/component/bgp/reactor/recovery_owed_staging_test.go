// Design: docs/architecture/update-building.md -- owed keys and the recovery worker
// Related: session_ownership.go -- adjOutWrite.owes, applyStaged
// Related: forward_build.go -- buildWithdrawalPayload
//
// VALIDATES: an owner withdrawal owed from a body the writer had to rewrite
// keeps the withdrawn NLRI it was owed for.
// PREVENTS: test/plugin/wellknown-no-export-withdraw-egress.ci, where the
// external client of a route server never received the withdrawal of a route
// it held: the owed record named a different prefix, the RIB was asked about
// that one, and the key was settled without anything reaching the wire.
package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// TestOwedWithdrawalKeepsItsNLRIWhenTheBodyIsRewritten proves the owed record
// carries the withdrawn route's own bytes when the same write also emits
// another route. Method: the source announces P to the destination. It then
// sends one UPDATE that withdraws P and announces Q, which an egress gate
// refuses, so the destination is handed the merged withdrawal of P and Q
// (buildWithdrawalPayload), Q as a synthesized withdrawal. P is owned by the
// source, so the writer owes it to recovery; Q is absent, so its synthesized
// withdrawal is written. The writer therefore rewrites the body without P and
// writes that body over the session buffer the original was copied into.
// The owed record, the RIB request and the final decision must all name P.
func TestOwedWithdrawalKeepsItsNLRIWhenTheBodyIsRewritten(t *testing.T) {
	rail := newOwedRail(t)
	p := []byte{24, 198, 51, 100}
	q := []byte{24, 10, 0, 0}

	rail.forward(t, false, p)
	require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1)

	// RFC 4271 Section 4.3: one UPDATE may withdraw P and announce Q.
	original := fwdPackUpdateBody(&message.Update{WithdrawnRoutes: p,
		PathAttributes: bytes.Clone(syncOrderAnnounceBody[4:24]), NLRI: q})
	scratch := make([]byte, len(original)+4)
	n := buildWithdrawalPayload(original, scratch, nil)
	require.NotZero(t, n, "the refused UPDATE converts to a withdrawal")
	withdrawal := scratch[:n]
	require.NoError(t, owedRailSynthesized(rail, original, withdrawal))

	updates := recoveryWrittenUpdates(t, rail.conn.written())
	require.Len(t, updates, 2, "the synthesized withdrawal of Q is written")
	require.True(t, bytes.Equal(q, updates[1].WithdrawnRoutes), "the write withdraws Q alone: P is owed")

	owed := rail.owed()
	require.Len(t, owed, 1)
	for _, record := range owed {
		require.Equal(t, p, record.nlri, "the owed record names the route it was owed for")
	}
	request := rail.request(t)
	require.Len(t, request.Keys, 1)
	require.Equal(t, p, request.Keys[0].NLRI, "the RIB is asked about P")
	rail.release <- nil

	lowEventually(t, func() bool {
		return len(recoveryWrittenUpdates(t, rail.conn.written())) == 3
	}, "the RIB's WITHDRAW decision is written")
	updates = recoveryWrittenUpdates(t, rail.conn.written())
	require.True(t, bytes.Equal(p, updates[2].WithdrawnRoutes), "the decision withdraws P")
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the decided record leaves the owed set")
}

// owedRailSynthesized writes withdrawal, the conversion of original, to the
// rail's destination as the forward pool does for a refused destination: with
// the original's provenance, so its own Withdrawn Routes stay received
// withdrawals and the converted announcement is synthesized.
func owedRailSynthesized(rail *owedRail, original, withdrawal []byte) error {
	target, source := rail.destination, rail.source
	s := target.currentSession()
	item := fwdItem{peer: target, session: s, receivedPeer: source,
		receivedGeneration: source.forwardGeneration.Load(), authority: adjOutForwarded}
	item.rawBodies = [][]byte{withdrawal}
	err := prepareFwdProvenance(&item, wireu.NewWireUpdate(original, source.recvCtxID),
		wireu.NewWireUpdate(withdrawal, source.recvCtxID), true)
	if item.provenance != nil {
		defer item.provenance.release()
	}
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.sentForward = &item
	defer func() { s.sentForward = nil }()
	if err := s.writeRawUpdateBody(withdrawal); err != nil {
		return err
	}
	return s.flushWrites()
}
