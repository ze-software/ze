// Design: docs/architecture/api/commands.md -- request quiesce waits for pending route work.
// Related: peer_sync_owed.go -- the peer-level drain predicate and its error.
// Related: recovery_owed_worker_test.go -- the channel-gated real recovery writer.
package reactor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// settledRecoveryRail removes only the initial-sync obligations of the test
// peers, so each test can isolate what an actual recovery decision still owes.
func settledRecoveryRail(t *testing.T) *owedRail {
	t.Helper()
	rail := newOwedRail(t)
	for _, peer := range rail.r.Peers() {
		if session := peer.currentSession(); session != nil {
			session.sendingInitialRoutes.Store(0)
			session.initialSyncEOROwed.Store(false)
		}
	}
	require.True(t, peersSynced(rail.r.Peers()), "setup: an idle recovery worker owes no route decision")
	return rail
}

// canceledPeerSyncDrain asks the public drain with cancellation already set.
// waitForCondition still checks its predicate first: a false settled answer
// returns nil, while genuinely pending work reports the cancellation without
// relying on a sleep or on whether another goroutine has been scheduled.
func canceledPeerSyncDrain(t *testing.T, rail *owedRail) error {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return (&reactorAPIAdapter{r: rail.r}).DrainPeerSync(ctx)
}

// TestDrainPeerSyncWaitsForColdRecovery proves the drain does not call a peer
// settled while its owner withdrawal is held in a RIB lookup. Once released,
// the exact withdrawal reaches the socket and the same drain can finish.
// MUTATION: omit the recovery obligation from Peer.syncOwed; the canceled
// drain falsely returns nil before the held lookup has answered.
func TestDrainPeerSyncWaitsForColdRecovery(t *testing.T) {
	rail := settledRecoveryRail(t)
	prefix := []byte{24, 192, 0, 2}
	require.NoError(t, canceledPeerSyncDrain(t, rail), "an idle worker must not keep the barrier closed")
	rail.forward(t, false, prefix)
	rail.forward(t, true, prefix)
	request := rail.request(t)
	require.Len(t, request.Keys, 1)
	require.Equal(t, prefix, request.Keys[0].NLRI)
	require.Len(t, rail.owed(), 1, "setup: the held lookup has not decided its key")
	require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1, "setup: the owner withdrawal is not on the wire")

	err := canceledPeerSyncDrain(t, rail)
	require.ErrorIs(t, err, context.Canceled, "a held route decision must keep DrainPeerSync pending")
	require.Contains(t, err.Error(), rail.destination.Settings().Address.String(), "the error names the peer still owed work")

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	drained := make(chan error, 1)
	go func() {
		drained <- (&reactorAPIAdapter{r: rail.r}).DrainPeerSync(ctx)
	}()
	rail.release <- nil
	require.NoError(t, <-drained, "the released recovery decision must let the drain finish")
	require.Empty(t, rail.owed())
	updates := recoveryWrittenUpdates(t, rail.conn.written())
	require.Len(t, updates, 2)
	require.Equal(t, prefix, updates[1].WithdrawnRoutes, "drain completion follows the exact owed withdrawal")
	require.Empty(t, updates[1].NLRI)
	require.False(t, updates[1].IsEndOfRIBAnyFamily())
}

// TestDrainPeerSyncWaitsForRecoveryTakeoverFlush proves a local takeover does
// not open the barrier between removing an owed key and flushing its useful
// replacement. The production writer supplies that pending state directly.
func TestDrainPeerSyncWaitsForRecoveryTakeoverFlush(t *testing.T) {
	rail := settledRecoveryRail(t)
	prefix := []byte{24, 192, 0, 2}
	rail.forward(t, false, prefix)
	rail.forward(t, true, prefix)
	rail.request(t)
	session := rail.destination.currentSession()
	body := ownershipRailBody(family.IPv4Unicast, false, prefix, 0)
	session.writeMu.Lock()
	err := session.writeUpdatePreFiltered(ownershipWriterUpdate(t, body))
	session.writeMu.Unlock()
	require.NoError(t, err)
	require.Empty(t, rail.owed(), "setup: the originated replacement decides the owed key itself")
	table := &rail.destination.adjOut
	table.mu.Lock()
	pending := table.pending
	table.mu.Unlock()
	require.True(t, pending, "setup: the replacement has not been flushed")
	require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1)
	require.ErrorIs(t, canceledPeerSyncDrain(t, rail), context.Canceled,
		"removing the owe must not hide the replacement's outstanding flush")

	session.writeMu.Lock()
	err = session.flushWrites()
	session.writeMu.Unlock()
	require.NoError(t, err)
	require.NoError(t, canceledPeerSyncDrain(t, rail), "a flushed local takeover leaves no wire work owed")
	updates := recoveryWrittenUpdates(t, rail.conn.written())
	require.Len(t, updates, 2)
	require.Equal(t, prefix, updates[1].NLRI, "the permitted local replacement remains useful")
	require.Empty(t, updates[1].WithdrawnRoutes)
}

// TestDrainPeerSyncRecoveryResetClearsObligation proves the barrier does not
// retain a recovery record after the destination's production table reset.
func TestDrainPeerSyncRecoveryResetClearsObligation(t *testing.T) {
	rail := settledRecoveryRail(t)
	prefix := []byte{24, 192, 0, 2}
	rail.forward(t, false, prefix)
	rail.forward(t, true, prefix)
	rail.request(t)
	require.Len(t, rail.owed(), 1)

	rail.destination.clearEncodingContexts()
	require.Empty(t, rail.owed(), "the reset drops work framed for the departed session")
	require.NoError(t, canceledPeerSyncDrain(t, rail), "the old session's recovery work must not hold a reset table")
}

// TestDrainPeerSyncStoppedRecoveryStillOwesDecision proves stopping a worker
// alone cannot falsely report a live destination settled. Its owed record
// must be decided or dropped by the destination's reset before the drain opens.
func TestDrainPeerSyncStoppedRecoveryStillOwesDecision(t *testing.T) {
	rail := settledRecoveryRail(t)
	prefix := []byte{24, 192, 0, 2}
	rail.forward(t, false, prefix)
	rail.forward(t, true, prefix)
	rail.request(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	adapter := &reactorAPIAdapter{r: rail.r}
	require.NoError(t, adapter.FlushForwardPool(ctx), "the held lookup must not hold the independent forward barrier")
	require.Len(t, rail.owed(), 1, "the forward barrier must not decide the held recovery key")
	session := rail.destination.currentSession()
	session.stopRecovery()
	rail.release <- nil
	session.joinRecovery()
	// Canceling the recovery waiter does not remove its already-dispatched
	// admission. Its independent forward worker still has to discard it.
	require.NoError(t, adapter.FlushForwardPool(ctx), "the canceled admission must leave the forward queue")
	lowEventually(t, func() bool {
		if rail.destination.forwardOverflowPending() {
			return false
		}
		return !rail.destination.forwardChannelPending()
	}, "the canceled admission no longer owes any queued forward work")
	require.Len(t, rail.owed(), 1, "setup: stopping the worker does not decide its live session's key")
	require.Equal(t, peerSyncOwedRouteDecision, rail.destination.syncOwed(),
		"only the outstanding route decision may keep this control pending")
	require.ErrorIs(t, canceledPeerSyncDrain(t, rail), context.Canceled,
		"a stopped worker is not proof its outstanding wire work completed")

	rail.destination.clearEncodingContexts()
	require.Empty(t, rail.owed(), "resetting the destination drops its obsolete recovery obligation")
	require.NoError(t, adapter.DrainPeerSync(ctx), "the reset destination must finish its remaining asynchronous work")
	require.NoError(t, canceledPeerSyncDrain(t, rail), "resetting the destination completes the obsolete obligation")
}
