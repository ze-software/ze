// Design: docs/architecture/plugin/rib-storage-design.md -- cold original cut and retained successor isolation.
package reactor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The retained original already has membership. Its successor's uncompleted
// publication cannot delay it or replace its original physical wire facts.
func TestBoundColdRetainedOriginalDoesNotBorrowSuccessorPublication(t *testing.T) {
	f := newBoundRelayWireFixture(t, false)
	work, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	lease, err := sourceReceipts.acquire(f.route.SourceReceipt, f.target.owner)
	require.NoError(t, err)
	require.NoError(t, lease.retain())
	t.Cleanup(func() {
		require.NoError(t, sourceReceipts.releaseOwned(context.Background(), f.route.SourceReceipt, f.target.owner))
	})
	oldGeneration := f.source.forwardGeneration.Load()
	f.session.closeSourceReceipt()
	next := sourceSessionFixture(t, f.source)
	next.setRecvCtxID(f.session.recvCtxID)
	require.NoError(t, next.captureSourceReceipt(f.source, &sourceReceipts))
	t.Cleanup(next.closeSourceReceipt)
	successor := &receivePublication{}
	successor.accepted.Store(999)
	successor.failed.Store(true)
	f.source.receivePublication.Store(successor)
	require.NotEqual(t, oldGeneration, f.source.forwardGeneration.Load())
	// A failed successor is a behavioral discriminator: borrowing its cut would
	// return a publication error instead of sending this authorized old actor.
	request, err := newBoundForwardRequest(t.Context(), f.sender, f.target.owner)
	require.NoError(t, err)
	original, err := f.target.api.coldBoundRelaySource(t.Context(), f.source.settings.Address, &f.route, f.sender, request)
	require.NoError(t, err)
	require.Equal(t, f.route.SourceReceipt, original.receipt)
	require.Equal(t, f.route.SourceConnectionID, original.info.filterInfo.ConnectionID)
	require.NoError(t, f.relay(t, f.route))
	require.Len(t, updateBodies(t, f.target.conn.written()), 1)
	for _, entry := range f.target.peer.adjOut.routes {
		require.Equal(t, f.route.SourceConnectionID, entry.owner.connectionID)
	}
	fresh := f.route
	fresh.SourceReceipt = 0
	fresh.SourceConnectionID++
	require.Error(t, f.relay(t, fresh), "unknown successor facts cannot borrow the retained old lease")
	other, wire := boundRelaySecondTarget(t, f, "192.0.2.187", 2179)
	target := rpc.ForwardTarget{Destination: other.settings.PeerKey().String(), DestinationOwner: other.sourceOwner, DestinationSession: other.session.initialReplay.Load()}
	sender, err := f.target.owner.Sender().WithDestination(target.DestinationOwner, target.DestinationSession)
	require.NoError(t, err)
	require.NoError(t, f.target.api.RelayStoredRouteBound(t.Context(), other.settings.Address, []rpc.StoredRoute{f.route}, sender, f.target.owner))
	require.Len(t, updateBodies(t, wire.written()), 1, "independent useful target survives beside new physical source")
	successor.failed.Store(false)
	successor.completed.Store(999)
	useful := f.route
	useful.SourceReceipt = next.sourceReceiptID()
	useful.SourceConnectionID = next.connectionID
	useful.MsgID++
	useful.NLRIHex = "18c61208"
	require.NoError(t, f.target.api.RelayStoredRouteBound(work, f.target.peer.settings.Address, []rpc.StoredRoute{useful}, f.sender, f.target.owner), "new physical source has its own useful original receipt")
	require.Len(t, updateBodies(t, f.target.conn.written()), 2)
}
