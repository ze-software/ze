package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// Goal: prove the End-of-RIB marker is not sent before the initial routing
// update for the family is complete, and is sent once it is.
// Method: sendInitialRoutes runs for an IPv4 unicast peer while one plugin that
// produces this peer's initial update has not acknowledged peer-up; the
// trigger clock signals when sendInitialRoutes is waiting on it. The wire is
// read while the update is incomplete, then after the acknowledgement.
//
// VALIDATES: RFC 4724 Sections 4 and 4.2, the marker follows the completed
// initial update.
// PREVENTS: an End-of-RIB written ahead of the routes a plugin still owes.
//
// RFC requirement: RFC4724-4-1 negative -- while a plugin producing the initial routing update for IPv4 unicast has not acknowledged, nothing is written and EORSent is 0: the marker is not sent before the initial update completes.
// RFC requirement: RFC4724-4.2-9 negative -- the same wait: the Receiving Speaker writes no End-of-RIB for IPv4 unicast before its initial update for the family is complete, and writes exactly the IPv4 unicast marker once it is.
func TestRFC4724EndOfRIBNotSentBeforeTheInitialUpdateCompletes(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	tc := newTriggerClock()
	peer.SetClock(tc)
	peer.ResetPeerUpBarrier()
	peer.SetPeerUpBarrier(1)

	done := make(chan struct{})
	go func() {
		peer.sendInitialRoutes()
		close(done)
	}()

	select {
	case <-tc.waiting:
	case <-time.After(5 * time.Second):
		t.Fatal("sendInitialRoutes never waited for the plugin that owes the initial update")
	}
	require.Empty(t, conn.written(), "no End-of-RIB before the initial update is complete")
	require.Zero(t, peer.Stats().EORSent)

	peer.SignalPeerUpBarrier()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sendInitialRoutes did not finish once the initial update completed")
	}
	require.Equal(t, eorWire(family.IPv4Unicast), conn.written())
	require.Equal(t, uint32(1), peer.Stats().EORSent)
}
