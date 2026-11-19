// Design: docs/architecture/api/process-protocol.md -- joined bound forwarding.
package reactor

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A runnable overflow predecessor completes before the real bound writer.
// While it is held, at most one additional bound carrier is admitted.
func TestBoundForwardOrdersAfterRunnableOverflow(t *testing.T) {
	f := newBoundRelayWireFixture(t, false)
	f.target.api.r.fwdPool.Stop()
	key := fwdKey{peerAddr: f.target.peer.settings.PeerKey()}
	held, release := make(chan struct{}), make(chan struct{})
	prior := make(chan []byte, 1)
	var released sync.Once
	unblock := func() { released.Do(func() { close(release) }) }
	defer unblock()
	const holdMarker, predecessorMarker = 1, 2
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		for i := range items {
			switch items[i].sourceMessageID {
			case holdMarker:
				close(held)
				<-release
			case predecessorMarker:
				prior <- f.target.conn.written()
			}
		}
	}, fwdPoolConfig{chanSize: 1, idleTimeout: time.Minute})
	f.target.api.r.fwdPool = pool
	t.Cleanup(pool.Stop)

	require.True(t, pool.TryDispatch(key, &fwdItem{sourceMessageID: holdMarker}))
	select {
	case <-held:
	case <-time.After(5 * time.Second):
		t.Fatal("forward worker never reached the predecessor hold")
	}

	require.True(t, pool.dispatchOverflow(key, &fwdItem{sourceMessageID: predecessorMarker}))
	pool.mu.RLock()
	worker := pool.workers[key]
	pool.mu.RUnlock()
	require.NotNil(t, worker)
	work, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	first := runBoundForwardQueueRelay(work, f, f.route)
	require.Eventually(t, func() bool {
		return worker.overflowPending.Load() == 2
	}, time.Second, time.Millisecond, "one predecessor and one bound carrier remain owed")
	second := runBoundForwardQueueRelay(work, f, f.route)
	select {
	case err := <-second:
		require.ErrorIs(t, err, errBoundForwardUnsettled)
	case <-work.Done():
		t.Fatal("the second carrier exceeded the worker's overflow reservation")
	}
	require.Empty(t, f.target.conn.written())
	unblock()
	select {
	case err := <-first:
		require.NoError(t, err)
	case <-work.Done():
		t.Fatal("the admitted carrier did not join its writer")
	}
	select {
	case before := <-prior:
		require.Empty(t, before, "the predecessor completed before the bound write")
	default:
		t.Fatal("the admitted carrier overtook its runnable predecessor")
	}
	require.Len(t, updateBodies(t, f.target.conn.written()), 1)
}

// Cancellation joins the captured writer while Ze's own initial sync gate
// still holds. A fresh replay writes normally after that same gate opens.
func TestBoundForwardCanceledBehindInitialSyncJoins(t *testing.T) {
	f := newBoundRelayWireFixture(t, false)
	peer := f.target.peer
	session := peer.currentSession()
	f.target.api.r.fwdPool.Stop()
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		for i := range items {
			if items[i].peer != nil {
				t.Error("the bound carrier escaped to the ordinary forwarding handler")
			}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Minute})
	t.Cleanup(pool.Stop)
	f.target.api.r.fwdPool = pool
	pool.registerOutgoingPool(fwdKey{peerAddr: peer.settings.PeerKey()}, 4096)
	session.sendingInitialRoutes.Store(1)
	open := func() {
		session.sendingInitialRoutes.Store(0)
		peer.wakeForwardOverflow()
	}
	defer open()
	f.route.InitialUpdate = true

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := runBoundForwardQueueRelay(ctx, f, f.route)
	require.Eventually(t, peer.forwardOverflowPending, time.Second, time.Millisecond)
	require.Empty(t, f.target.conn.written())
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("canceled carrier did not join while initial sync was held")
	}
	require.True(t, peer.initialSyncInProgress())
	require.Empty(t, f.target.conn.written())

	require.Eventually(t, func() bool {
		return !peer.forwardOverflowPending()
	}, time.Second, time.Millisecond, "cancellation cleared the old overflow credit")
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	replay := runBoundForwardQueueRelay(work, f, f.route)
	require.Eventually(t, peer.forwardOverflowPending, time.Second, time.Millisecond,
		"another replay can reserve while the hard initial gate still holds")
	require.Empty(t, f.target.conn.written())
	open()
	select {
	case err := <-replay:
		require.NoError(t, err)
	case <-work.Done():
		t.Fatal("the admitted retry did not join after the hard initial gate opened")
	}
	require.Len(t, updateBodies(t, f.target.conn.written()), 1)
}

// Live work already admitted in the channel remains ordered through an API
// replay hold and an overlapping policy-replay hold. Two later bound live
// carriers park independently while initial replay writes, then all four live
// prefixes follow the replay in their original arrival order.
func TestBoundForwardDefersLiveUntilInitialReplayFinishes(t *testing.T) {
	f := newBoundRelayWireFixture(t, false)
	peer := f.target.peer
	session := peer.currentSession()
	f.target.api.r.fwdPool.Stop()
	key := fwdKey{peerAddr: peer.settings.PeerKey()}
	held, release := make(chan struct{}), make(chan struct{})
	policyRaised := make(chan struct{})
	var released sync.Once
	unblock := func() { released.Do(func() { close(release) }) }
	defer unblock()
	const holdMarker, policyMarker = 1, 2
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		for i := range items {
			switch items[i].sourceMessageID {
			case holdMarker:
				close(held)
				<-release
			case policyMarker:
				// The first direct carrier has reached its final writer
				// under the API fence. Raise a separate hold before
				// the next direct carrier reaches the same writer.
				peer.beginPolicyReplay(session)
				close(policyRaised)
			}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Minute})
	f.target.api.r.fwdPool = pool
	pool.registerOutgoingPool(key, 4096)
	t.Cleanup(pool.Stop)
	require.True(t, pool.TryDispatch(key, &fwdItem{sourceMessageID: holdMarker}))
	select {
	case <-held:
	case <-time.After(5 * time.Second):
		t.Fatal("forward worker never reached the predecessor hold")
	}

	work, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	first := runBoundForwardQueueRelay(work, f, f.route)
	require.Eventually(t, func() bool {
		return peer.fwdChannelPending.Load() == 1
	}, time.Second, time.Millisecond, "first live carrier is already admitted to the bounded channel")
	require.True(t, pool.TryDispatch(key, &fwdItem{sourceMessageID: policyMarker}))
	secondRoute := f.route
	secondRoute.NLRIHex = "18c61208"
	secondRoute.MsgID++
	second := runBoundForwardQueueRelay(work, f, secondRoute)
	require.Eventually(t, func() bool {
		return peer.fwdChannelPending.Load() == 2
	}, time.Second, time.Millisecond, "both live carriers are already admitted to the bounded channel")
	peer.initialUpdateOwed.Store(true)

	thirdRoute := f.route
	thirdRoute.NLRIHex = "18c6120a"
	thirdRoute.MsgID += 2
	third := runBoundForwardQueueRelay(work, f, thirdRoute)
	require.Eventually(t, peer.forwardOverflowPending, time.Second, time.Millisecond,
		"third live carrier is parked")
	fourthRoute := f.route
	fourthRoute.NLRIHex = "18c6120b"
	fourthRoute.MsgID += 3
	fourth := runBoundForwardQueueRelay(work, f, fourthRoute)
	require.Eventually(t, func() bool {
		return peer.fwdOverflowPending.Load().Load() == 2
	}, time.Second, time.Millisecond, "both post-fence live carriers are parked")
	replay := f.route
	replay.NLRIHex = "18c61209"
	replay.MsgID += 4
	replay.InitialUpdate = true
	initial := runBoundForwardQueueRelay(work, f, replay)
	require.Eventually(t, func() bool {
		return peer.fwdOverflowPending.Load().Load() == 3
	}, time.Second, time.Millisecond, "initial replay reserves beside both parked live carriers")

	unblock()
	select {
	case err := <-initial:
		require.NoError(t, err)
	case <-work.Done():
		t.Fatal("initial replay did not join while the live holds stayed raised")
	}
	select {
	case <-policyRaised:
	default:
		t.Fatal("policy replay marker never ran between the direct carriers")
	}
	require.True(t, peer.forwardOrderHold(false))
	requireBoundForwardQueuePrefixes(t, f, 9)
	liveResults := []<-chan error{first, second, third, fourth}
	for _, result := range liveResults {
		select {
		case err := <-result:
			t.Fatalf("live carrier joined before the holds opened: %v", err)
		default:
		}
	}

	peer.initialUpdateOwed.Store(false)
	peer.endPolicyReplay()
	for _, result := range liveResults {
		select {
		case err := <-result:
			require.NoError(t, err)
		case <-work.Done():
			t.Fatal("live carrier did not join after the holds opened")
		}
	}
	requireBoundForwardQueuePrefixes(t, f, 9, 7, 8, 10, 11)
}

func requireBoundForwardQueuePrefixes(t *testing.T, f *boundRelayWireFixture, prefixes ...byte) {
	t.Helper()
	bodies := updateBodies(t, f.target.conn.written())
	require.Len(t, bodies, len(prefixes))
	for i, prefix := range prefixes {
		address := netip.AddrFrom4([4]byte{198, 18, prefix, 0})
		require.Equal(t, []netip.Prefix{netip.PrefixFrom(address, 24)}, updatePrefixes(t, bodies[i], false))
	}
}

func runBoundForwardQueueRelay(ctx context.Context, f *boundRelayWireFixture, route rpc.StoredRoute) <-chan error {
	result := make(chan error, 1)
	go func() {
		result <- f.target.api.RelayStoredRouteBound(ctx, f.target.peer.settings.Address, []rpc.StoredRoute{route}, f.sender, f.target.owner)
	}()
	return result
}
