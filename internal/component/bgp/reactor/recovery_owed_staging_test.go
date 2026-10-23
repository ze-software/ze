// Design: docs/architecture/update-building.md -- owed keys and the recovery worker
// Related: session_ownership.go -- adjOutWrite.owes, applyStaged
//
// VALIDATES: an owner withdrawal owed from a body the writer had to rewrite
// keeps the withdrawn NLRI it was owed for.
// PREVENTS: a client never receiving the withdrawal of a route it holds when
// the same UPDATE also announces another route: the owed record named the
// announced prefix read back from the rewritten buffer, the RIB was asked
// about that one, and the key was settled without anything reaching the wire.
package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestOwedWithdrawalKeepsItsNLRIWhenTheBodyIsRewritten proves the owed record
// carries the withdrawn route's own bytes when the same write also emits
// another route. Method: the source announces P to the destination. It then
// sends one UPDATE that withdraws P and announces R. P is owned by the source,
// so the writer owes its withdrawal to recovery; R is the first route of its
// key, so its announcement is written. The writer therefore rewrites the body
// without P and writes that body over the session buffer the original was
// copied into, where P's withdrawn bytes were. The owed record, the RIB
// request and the final decision must all name P.
//
// The written sibling is an announcement: a synthesized withdrawal sibling
// would have to name a route the source advertised to this destination, and a
// withdrawal of such a route is owed to recovery as well (owesWithdrawal), so
// no withdrawal sibling of P is written while P is owed.
func TestOwedWithdrawalKeepsItsNLRIWhenTheBodyIsRewritten(t *testing.T) {
	rail := newOwedRail(t)
	p := []byte{24, 198, 51, 100}
	r := []byte{24, 10, 0, 0}

	rail.forward(t, false, p)
	require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1)

	// RFC 4271 Section 4.3: one UPDATE may withdraw P and announce R.
	original := fwdPackUpdateBody(&message.Update{WithdrawnRoutes: p,
		PathAttributes: bytes.Clone(syncOrderAnnounceBody[4:24]), NLRI: r})
	require.NoError(t, ownershipWriterForward(t, rail.destination, rail.source, original, false))

	updates := recoveryWrittenUpdates(t, rail.conn.written())
	require.Len(t, updates, 2, "the announcement of R is written")
	require.Empty(t, updates[1].WithdrawnRoutes, "the write withdraws nothing: P is owed")
	require.True(t, bytes.Equal(r, updates[1].NLRI), "the write announces R")

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
