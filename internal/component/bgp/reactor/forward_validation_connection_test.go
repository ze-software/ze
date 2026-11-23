// Design: docs/architecture/bgp/structural-forwarding.md -- original receive validation queries.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// The real partitioner asks validation about the held original receipt even
// after the Peer publishes another Session. Unknown receipts never acquire
// that successor's identity. Useful original prefix bytes are the oracle.
func TestForwardValidationUsesOriginalReceiptConnection(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	facts.source.ctxID = ctxID
	live, err := sourceReceipts.issue(facts)
	require.NoError(t, err)
	t.Cleanup(live.close)
	replacement := peer.newSession()
	t.Cleanup(replacement.timers.StopAll)
	peer.mu.Lock()
	peer.session = replacement
	peer.mu.Unlock()
	require.NotEqual(t, facts.connectionID, replacement.connectionID)
	var seen []ribevents.ValidationRoute
	ribevents.RegisterValidationLookup(func(route ribevents.ValidationRoute, _ uint64) bool {
		seen = append(seen, route)
		return route.ConnectionID == facts.connectionID
	}, func(ribevents.ValidationRoute) bool { return false })
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	body := []byte{0, 0, 0, 0, 24, 10, 77, 0}
	update := &ReceivedUpdate{SourcePeerIP: facts.source.addr, SourceOwner: peer.sourceOwner, SourceReceipt: live.ID(), WireUpdate: wireu.NewWireUpdate(body, ctxID)}
	selected, _, _, err := forwardValidationWire(update)
	require.NoError(t, err)
	defer update.returnFwdHandles()
	require.NotNil(t, selected)
	prefix, err := selected.NLRI()
	require.NoError(t, err)
	require.Equal(t, []byte{24, 10, 77, 0}, prefix)
	require.NotEmpty(t, seen)
	for _, query := range seen {
		require.Equal(t, facts.connectionID, query.ConnectionID)
	}
	update.SourceReceipt = 0
	selected, _, _, err = forwardValidationWire(update)
	require.NoError(t, err)
	require.Nil(t, selected, "unrecorded provenance must not borrow the current Session")
}
