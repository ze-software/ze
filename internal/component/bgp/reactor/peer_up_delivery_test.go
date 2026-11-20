// Design: docs/architecture/plugin/plugin-system.md -- peer-up delivery and the initial sync that follows it.
// Related: relay_bound_queue_test.go -- the bound relay held behind the initial-sync gate.
package reactor

import (
	"context"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// peerUpReplayConsumer is a state consumer whose UP handler runs on the FSM
// callback goroutine and returns only when its own work has, as the plugin
// server's delivery of a peer-up event does (deliverPeerState).
type peerUpReplayConsumer struct {
	up func()
}

func (c peerUpReplayConsumer) OnPeerEstablished(*Peer)        { c.up() }
func (peerUpReplayConsumer) OnPeerClosed(*Peer, sessionClose) {}

// TestEstablishedPeerUpReplayJoinsBeforeEndOfRIB proves that a plugin which
// replays a stored route from inside its peer-up handler, and joins the write
// as bgp-adj-rib-in does, gets that route on the wire, and that neither the BFD
// client nor the End-of-RIB order depends on the handler.
//
// Method: the session is left as enterEstablished leaves it, gate closed and
// marker owed, and the gate is never opened by hand. A state consumer's UP
// handler issues one bound relay with InitialUpdate set, under a deadline, and
// records what it saw when the relay returned. The production Established
// sequence (Peer.startEstablishedSession, which the FSM callback calls) then
// runs on the test goroutine.
//
// VALIDATES: the relay returns nil well inside its deadline, the route is the
// only UPDATE on the wire when the handler returns, the End-of-RIB follows it,
// and the BFD start seam was reached before the handler ran.
// PREVENTS: the Established callback waiting, for the plugin's whole deadline,
// on a gate only the initial sync it has not started can open: the replay then
// ends canceled and the peer receives no stored route and no BFD session.
func TestEstablishedPeerUpReplayJoinsBeforeEndOfRIB(t *testing.T) {
	f := newBoundRelayWireFixture(t, false)
	peer := f.target.peer
	session := peer.currentSession()
	r := f.target.api.r

	// The fixture's pool refuses every call to its handler. A release of the
	// parked carrier wakes the worker, so this one refuses routed work only.
	r.fwdPool.Stop()
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		for i := range items {
			if items[i].peer != nil {
				t.Error("the bound carrier escaped to the ordinary forwarding handler")
			}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Minute})
	t.Cleanup(pool.Stop)
	r.fwdPool = pool
	pool.registerOutgoingPool(fwdKey{peerAddr: peer.settings.PeerKey()}, 4096)

	var bfdStarted atomic.Bool
	peer.bfd.afterClaim = func() { bfdStarted.Store(true) }

	// The production publication: the gate closes, the marker is owed and the
	// session takes the initial-replay receipt its peer-up event carries.
	peer.enterEstablished(session)
	require.Equal(t, int32(1), session.sendingInitialRoutes.Load(), "enterEstablished leaves the gate closed")
	require.True(t, session.initialSyncEOROwed.Load(), "enterEstablished leaves the End-of-RIB owed")
	sender, err := f.target.owner.Sender().WithDestination(peer.sourceOwner, session.initialReplay.Load())
	require.NoError(t, err)
	route := f.route
	route.InitialUpdate = true

	const deadline = 4 * time.Second
	handled := false
	var (
		relayErr       error
		relayElapsed   time.Duration
		bfdBeforeRelay bool
		wireAtReturn   []byte
	)
	r.addPeerObserver(peerUpReplayConsumer{up: func() {
		handled = true
		bfdBeforeRelay = bfdStarted.Load()
		ctx, cancel := context.WithTimeout(t.Context(), deadline)
		defer cancel()
		began := time.Now()
		relayErr = f.target.api.RelayStoredRouteBound(ctx, peer.settings.Address, []rpc.StoredRoute{route}, sender, f.target.owner)
		relayElapsed = time.Since(began)
		wireAtReturn = f.target.conn.written()
	}})
	t.Cleanup(func() {
		if session.initialSyncStart.Load() != syncLatchStarted {
			return
		}
		select {
		case <-session.initialSyncDone:
		case <-time.After(10 * time.Second):
			t.Error("the session's initial sync never returned")
		}
	})

	peer.startEstablishedSession(session)

	require.True(t, handled, "the Established sequence must deliver the peer-up to the state consumer")
	require.True(t, bfdBeforeRelay, "the BFD client must start before the plugin notification, not after it")
	require.NoError(t, relayErr, "a replay joined from the peer-up handler must reach the wire")
	require.Less(t, relayElapsed, deadline/2, "the replay must not wait out the handler's deadline")

	// RFC 4724 Section 2: the End-of-RIB marker indicates "the completion of the
	// initial routing update", so the replay joined in the handler precedes it.
	atReturn := updateBodies(t, wireAtReturn)
	require.Len(t, atReturn, 1, "the replayed route is the only UPDATE written when the handler returns")
	wantPrefix := []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}
	require.Equal(t, wantPrefix, updatePrefixes(t, atReturn[0], false))

	select {
	case <-session.initialSyncDone:
	case <-time.After(10 * time.Second):
		t.Fatal("the initial sync did not finish once the peer-up notification returned")
	}
	bodies := updateBodies(t, f.target.conn.written())
	require.Len(t, bodies, 2, "the route, then one End-of-RIB for the negotiated family")
	require.Equal(t, wantPrefix, updatePrefixes(t, bodies[0], false))
	require.Equal(t, eorWire(family.IPv4Unicast)[19:], bodies[1], "the End-of-RIB follows the replayed route")
	require.False(t, peer.initialSyncInProgress(), "the sync opened its own gate")
	require.False(t, session.initialSyncEOROwed.Load(), "the sync sent its marker")
}
