// Design: docs/architecture/api/process-protocol.md -- transient live source write joins.
package reactor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLiveSourceWriteCannotBorrowLaterRetention holds a live-only write while
// the same actual owner's explicit acquisition upgrades its shared lease. After
// DOWN, only a separately admitted retained write can use retained authority.
func TestLiveSourceWriteCannotBorrowLaterRetention(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var store sourceReceiptStore
	live, err := store.issue(facts)
	require.NoError(t, err)
	owner, done := sourceReceiptOwner(t)
	defer close(done)
	boundSourceGrant(peer, "rib")
	write, err := store.beginLive(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	require.True(t, write.live)
	require.True(t, write.lease.transient)
	require.NoError(t, write.current())
	lease, err := store.acquire(live.ID(), owner)
	require.NoError(t, err)
	require.Same(t, write.lease, lease)
	require.False(t, lease.transient)
	require.NoError(t, lease.retain())
	peer.state.Store(int32(PeerStateStopped))
	live.close()
	require.ErrorIs(t, write.current(), errForwardNoSource, "later lease retention must not upgrade this held write")
	retained, err := lease.begin(peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	require.NoError(t, retained.current(), "a separately admitted retained write remains useful")
	retained.complete()
	write.complete()
	require.NoError(t, lease.revoke(t.Context()))
	require.Empty(t, store.entries)
}

// TestLiveSourceWriteGrantCleanupJoinsHeldGuard revokes through the existing
// grant transaction while the live guard is outstanding. A canceled join cannot
// acknowledge it; completing the actual writer allows the same apply to finish.
func TestLiveSourceWriteGrantCleanupJoinsHeldGuard(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var store sourceReceiptStore
	live, err := store.issue(facts)
	require.NoError(t, err)
	owner, done := sourceReceiptOwner(t)
	defer close(done)
	boundSourceGrant(peer, "rib")
	write, err := store.beginLive(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	prepared, err := store.prepareGrant(peer, owner)
	require.NoError(t, err)
	canceled, stop := context.WithCancel(t.Context())
	stop()
	applied, err := prepared.apply(canceled)
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, applied)
	require.Error(t, write.current())
	require.Error(t, write.lease.lifetime.Err())
	write.complete()
	require.NoError(t, applied.finalize(t.Context()))
	require.Empty(t, live.record.leases, "completed transient writes leave no source lease history")
	live.close()
	require.Empty(t, store.entries)
}

// TestLiveSourceWriteNeedsOpenOriginalGeneration keeps a useful current write
// beside a closed original receipt. Missing delivery never gains retention from
// an in-flight transient lease, and final completion removes its final row.
func TestLiveSourceWriteNeedsOpenOriginalGeneration(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var store sourceReceiptStore
	live, err := store.issue(facts)
	require.NoError(t, err)
	owner, done := sourceReceiptOwner(t)
	defer close(done)
	boundSourceGrant(peer, "rib")
	write, err := store.beginLive(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	require.NoError(t, write.current())
	live.close()
	_, err = store.acquire(live.ID(), owner)
	require.Error(t, err)
	_, err = store.beginLive(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.Error(t, err)
	write.complete()
	require.Empty(t, store.entries)
}
