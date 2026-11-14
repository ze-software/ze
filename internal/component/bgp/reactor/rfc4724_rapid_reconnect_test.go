// Design: docs/architecture/behavior/peer-lifecycle.md -- the initial sync, one goroutine per session, joined when its session ends
// Related: peer_initial_sync_join_test.go -- startSyncJoinNeighbor, holdStaticPhase and the clock these tests hold the sync with
// Related: peer_initial_sync.go -- sendInitialRoutes, the sync whose End-of-RIB closes the initial update

package reactor

import (
	"bytes"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// rapidReconnectPrefix is the route queued for the second session, and
// rapidReconnectNLRI its NLRI as an UPDATE carries it: the prefix length, then
// the three octets of 192.0.2.0/24.
const rapidReconnectPrefix = "192.0.2.0/24"

var rapidReconnectNLRI = []byte{24, 192, 0, 2}

// readInitialUpdate reads conn until the IPv4 End-of-RIB and returns every
// UPDATE that came before it, KEEPALIVEs skipped. It then watches conn for
// syncJoinProbe and fails on any further UPDATE: a second End-of-RIB, or a
// write that lands after the marker that claims to end the initial update.
func readInitialUpdate(t *testing.T, conn net.Conn) [][]byte {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	var routes [][]byte
	for range 16 {
		frame, err := core4271ReadMessage(conn)
		require.NoError(t, err, "the second session's connection carried no End-of-RIB")
		if frame[18] == byte(msgtype.TypeKEEPALIVE) {
			continue
		}
		require.Equal(t, byte(msgtype.TypeUPDATE), frame[18], "only UPDATEs and KEEPALIVEs make an initial update")
		if bytes.Equal(frame, eorUpdate()) {
			requireNoUpdateAfter(t, conn)
			return routes
		}
		routes = append(routes, frame)
	}
	t.Fatal("the second session's connection carried 16 messages and no End-of-RIB")
	return nil
}

// requireNoUpdateAfter fails when conn carries an UPDATE within syncJoinProbe.
// The read deadline ending the watch is the pass.
func requireNoUpdateAfter(t *testing.T, conn net.Conn) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(syncJoinProbe)))
	for {
		frame, err := core4271ReadMessage(conn)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return
		}
		require.NoError(t, err, "the second session's connection failed after its End-of-RIB")
		require.Equal(t, byte(msgtype.TypeKEEPALIVE), frame[18],
			"an UPDATE reached the second session's connection after its End-of-RIB")
	}
}

// TestPredecessorSyncWritesNothingOnSuccessor is the RFC 4724 Section 4 proof
// of a rapid reconnect: a session's initial update, and the End-of-RIB that
// closes it, belong to that session alone.
//
// VALIDATES: AC-1. The first session's sync is held in the peer-up barrier wait
// (armed, never acknowledged) when the far end ends that session with a Cease.
// The peer reconnects, and the second session reaches Established with its own
// sync held before its static phase. A route is queued for the second session
// (QueueAnnounce, accepted only while its queue gate is closed). If the first
// sync is still waiting, the test lets it go as the barrier timeout would and
// waits for it to end. Then it lets the second sync go. The second session's
// connection carries exactly one UPDATE before its End-of-RIB, the queued
// route, then exactly one IPv4 unicast End-of-RIB, the one family negotiated,
// and no UPDATE after it.
// PREVENTS: the first session's sync outliving its session and running its
// queue drain after the second session established, which takes the route
// queued for the second session off the queue and writes it on a connection
// that is gone. The second session's End-of-RIB then claims an initial update
// that never carried that route.
// MUTATION: the record rests on a pair (rfc/discrimination/): the session-end
// join removed from runOnce (`session.endInitialSync()` in peer_run.go) AND the
// barrier wait made blind to the session's end (`case <-session.Done():` in
// Peer.waitPeerUpBarrier). Each alone leaves this test green, because either
// one ends the first sync before the second session's queue exists.
//
// RFC requirement: RFC4724-4-1 positive -- a running EBGP Peer whose export policy accepts, whose first session ends by a received Cease while its initial sync waits in an unacknowledged peer-up barrier, and whose second session reaches Established with one route queued for it, writes on the second session's connection exactly one UPDATE carrying that route, then exactly one IPv4 unicast End-of-RIB, and no UPDATE within half a second after it.
func TestPredecessorSyncWritesNothingOnSuccessor(t *testing.T) {
	// RFC 8212 Section 3: an EBGP peer with no export policy is sent no routes
	// (Session.writeUpdateBody withdraws what the final step rejects), so the
	// peer accepts at the end of its export chain.
	n, clk := startSyncJoinNeighborWith(t, func(settings *PeerSettings) {
		settings.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	})

	firstHold := holdStaticPhase(t, n.peer)
	firstConn := n.accept(t)
	n.establish(t, firstConn, 90)
	awaitSessionsEstablished(t, n.peer, 1)
	first := n.peer.currentSession()
	n.peer.SetPeerUpBarrier(1)
	firstHold.release()
	clk.awaitBarrier(t)

	secondHold := holdStaticPhase(t, n.peer)
	backoffCease(t, firstConn)

	// A join whose barrier wait does not see the session end leaves the run
	// loop out of its reconnect wait until the barrier lets the first sync go,
	// which the test does as the timeout would.
	fired := false
	select {
	case <-clk.waits:
	case <-time.After(syncJoinProbe):
		clk.fireBarrier()
		fired = true
		clk.awaitWait(t)
	}
	clk.release()

	secondConn := n.accept(t)
	n.establish(t, secondConn, 90)
	awaitSessionsEstablished(t, n.peer, 2)
	require.NoError(t, n.peer.QueueAnnounce(testRoute(rapidReconnectPrefix), bgptypes.NextHopExplicit, false),
		"the second session's queue gate must be closed while its sync is held")

	if !fired {
		// A first sync that ends on its own has ended by now. One that is still
		// in its barrier wait is let go as the timeout would.
		select {
		case <-first.initialSyncDone:
		case <-time.After(syncJoinProbe):
		}
		clk.fireBarrier()
	}
	awaitInitialSyncDone(t, first)
	secondHold.release()

	routes := readInitialUpdate(t, secondConn)
	require.Len(t, routes, 1,
		"the second session's initial update must carry exactly the route queued for it before its End-of-RIB")
	require.True(t, bytes.Contains(routes[0], rapidReconnectNLRI),
		"the UPDATE before the second session's End-of-RIB does not carry the route queued for it")
}
