// Design: docs/architecture/core-design.md -- section 18a, a restart names its session.
// Related: peer_initial_sync.go -- the two opQueue drains and the teardown they run.
package reactor

import (
	"bufio"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// closingDrainTeardownConn drives the closing drain from the socket. Its first
// write is the plugin route, sent after the main drain: it queues route, which
// only the closing drain (drainAndCloseQueueGate) can then send. Its next write
// is that send, made with p.mu released, and it asks for a restart of the
// session the way a reload's binding restart does.
type closingDrainTeardownConn struct {
	recordingConn
	peer   *Peer
	route  *rib.Route
	writes int
}

func (c *closingDrainTeardownConn) Write(b []byte) (int, error) {
	c.writes++
	switch c.writes {
	case 1:
		if err := c.peer.QueueAnnounce(c.route, false, false); err != nil {
			return 0, err
		}
	case 2:
		c.peer.restartForBinding(c.peer.session, []string{"monitor"}, "test restart")
	}
	return c.recordingConn.Write(b)
}

// TestClosingDrainRunsQueuedTeardown checks that a teardown queued while the
// closing drain sends is carried out: End-of-RIB, then the Cease NOTIFICATION
// on the session it names. The closing drain once logged it as unexpected and
// dropped it, so a reload's restart there left the process with a down and no
// up (review B3). Method: the real sendInitialRoutes over a recording socket.
//
// VALIDATES: a teardown queued during the closing drain sends End-of-RIB, then
// the NOTIFICATION, and opens the queue gate.
// PREVENTS: a reload's restart lost in the closing drain.
func TestClosingDrainRunsQueuedTeardown(t *testing.T) {
	family.RegisterTestFamilies()
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	peer, _ := newInitialSyncPeer(t, true, fam)
	peer.settings.PluginRoutes = []PluginRoute{{Family: fam.String(), NLRI: mustHex(t, "050118c00002")}}
	wire, err := nlri.NewWireNLRI(fam, mustHex(t, "050118c00003"), false)
	if err != nil {
		t.Fatal(err)
	}
	conn := &closingDrainTeardownConn{peer: peer, route: rib.NewRouteWithASPath(wire, netip.Addr{}, nil, nil)}
	peer.session.conn = conn
	peer.session.bufWriter = bufio.NewWriterSize(conn, 4096)

	peer.sendInitialRoutes()

	if conn.writes < 2 {
		t.Fatalf("socket writes = %d, the closing drain never sent the queued route", conn.writes)
	}
	var types []uint8
	for _, frame := range bgpFrames(t, conn.written()) {
		types = append(types, frame[18])
	}
	if len(types) == 0 || types[len(types)-1] != uint8(msgtype.TypeNOTIFICATION) {
		t.Fatalf("message types on the wire = %v, want a Cease NOTIFICATION last", types)
	}
	if peer.sendingInitialRoutes.Load() != 0 {
		t.Error("the teardown left the initial-sync queue gate closed")
	}
}

// sessionSwapConn hands the peer to next on its first write, the first
// End-of-RIB the main drain's teardown path sends: the initial sync then
// outlives the session its queued teardown named.
type sessionSwapConn struct {
	recordingConn
	peer *Peer
	next *Session
	done bool
}

func (c *sessionSwapConn) Write(b []byte) (int, error) {
	if !c.done {
		c.done = true
		c.peer.mu.Lock()
		c.peer.session = c.next
		c.peer.mu.Unlock()
	}
	return c.recordingConn.Write(b)
}

// TestQueuedTeardownSkipsSessionThatReplacedItsTarget checks that the main
// drain's teardown ends only the session it names. The target is checked when
// the teardown is dequeued, but the End-of-RIB sends come between that and the
// teardown, and the peer can move to another session in between (review I1).
// Method: a teardown for the first session is queued, the socket hands the peer
// to a second session during the first End-of-RIB, and the second session's
// wire must carry no NOTIFICATION.
//
// VALIDATES: the queued teardown re-checks its target before it tears down.
// PREVENTS: a restart decided for one session ending the session after it.
func TestQueuedTeardownSkipsSessionThatReplacedItsTarget(t *testing.T) {
	peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
	first := peer.session
	other, nextConn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	next := other.session
	conn := &sessionSwapConn{peer: peer, next: next}
	first.conn = conn
	first.bufWriter = bufio.NewWriterSize(conn, 4096)
	peer.restartForBinding(first, []string{"monitor"}, "test restart")

	peer.sendInitialRoutes()

	if !conn.done {
		t.Fatal("the teardown path sent no End-of-RIB on the first session")
	}
	for _, frame := range bgpFrames(t, nextConn.written()) {
		if frame[18] == uint8(msgtype.TypeNOTIFICATION) {
			t.Fatal("a teardown named for the first session ended the second")
		}
	}
}

// TestQueuedTeardownSendsNothingToSessionThatReplacedItsTarget checks that a
// teardown whose session has gone writes nothing to the session that holds the
// peer now. Its End-of-RIB would tell that session's peer the initial update is
// complete before any of it was sent, so a graceful-restart peer could flush
// its stale routes early (RFC 4724 Section 2) (review I2). Method: the peer is
// handed to a second session before runQueuedTeardown runs for the first, and
// the second session's wire must stay empty.
//
// VALIDATES: runQueuedTeardown checks its target before the End-of-RIB.
// PREVENTS: an early End-of-RIB on the session after the one being torn down.
func TestQueuedTeardownSendsNothingToSessionThatReplacedItsTarget(t *testing.T) {
	peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
	first := peer.session
	other, nextConn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	peer.mu.Lock()
	peer.session = other.session
	peer.mu.Unlock()

	peer.runQueuedTeardown(peer.settings.Address.String(), peerOp{Type: PeerOpTeardown, TeardownSession: first})

	if frames := bgpFrames(t, nextConn.written()); len(frames) != 0 {
		t.Fatalf("the second session's wire carries %d frames, want none", len(frames))
	}
}
