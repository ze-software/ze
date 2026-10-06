// Design: docs/architecture/behavior/peer-lifecycle.md -- initial-sync panic recovery.
package reactor

import (
	"bufio"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// initialSyncPanicConn faults at the socket write boundary, after the real
// initial-sync encoder has built its UPDATE and acquired its write hold.
type initialSyncPanicConn struct {
	recordingConn
	writes int
}

func (c *initialSyncPanicConn) Write(b []byte) (int, error) {
	c.writes++
	if c.writes == 1 {
		panic("BUG: injected initial-sync socket write failure")
	}
	return c.recordingConn.Write(b)
}

// TestInitialSyncPanicReleasesWriteHolds injects a panic at each held phase's
// first socket write. Synchronous recovery and TryLock make both failures
// bounded: cleanup never waits on the very mutex whose release is under test.
func TestInitialSyncPanicReleasesWriteHolds(t *testing.T) {
	family.RegisterTestFamilies()
	for _, phase := range []string{"plugin", "eor"} {
		t.Run(phase, func(t *testing.T) {
			fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
			peer, _ := newInitialSyncPeer(t, true, fam)
			if phase == "plugin" {
				peer.settings.PluginRoutes = []PluginRoute{{
					Family: fam.String(), NLRI: mustHex(t, "050118c00002"),
				}}
				peer.settings.ManualEOR = true
			}
			conn := &initialSyncPanicConn{}
			session := peer.session
			session.conn = conn
			session.bufWriter = bufio.NewWriterSize(conn, 4096)

			peer.sendInitialRoutes()

			if conn.writes != 1 {
				t.Fatalf("socket writes = %d, want one injected panic", conn.writes)
			}
			if peer.sendingInitialRoutes.Load() != 0 {
				t.Error("panic recovery left the initial-sync queue gate closed")
			}
			if peer.initialSyncEOROwed.Load() {
				t.Error("panic recovery left an End-of-RIB marker owed")
			}
			if !session.writeMu.TryLock() {
				t.Fatal("initial-sync panic retained session.writeMu; teardown cannot progress")
			}
			session.writeMu.Unlock()
			if !peer.mu.TryLock() {
				t.Fatal("initial-sync panic retained peer.mu")
			}
			peer.mu.Unlock()
			if !peer.staticMu.TryLock() {
				t.Fatal("initial-sync panic retained peer.staticMu")
			}
			peer.staticMu.Unlock()
			session.closeConn()
		})
	}
}

// initialSyncPanicNLRI injects a codec failure where buildRIBRouteUpdate
// measures its NLRI, while the queue drainer still owns peer.mu.
type initialSyncPanicNLRI struct {
	nlri.NLRI
	calls int
}

func (n *initialSyncPanicNLRI) Len() int {
	n.calls++
	panic("BUG: injected initial-sync NLRI codec failure")
}

// initialSyncQueueConn admits a route during the plugin write hold, after the
// first queue drain but before drainAndCloseQueueGate. The next drain therefore
// reaches the same codec fault through the real sendInitialRoutes lifecycle.
type initialSyncQueueConn struct {
	recordingConn
	peer  *Peer
	route *rib.Route
}

func (c *initialSyncQueueConn) Write(b []byte) (int, error) {
	if c.route != nil {
		if err := c.peer.QueueAnnounce(c.route, false, false); err != nil {
			return 0, err
		}
		c.route = nil
	}
	return c.recordingConn.Write(b)
}

// TestInitialSyncPanicReleasesPeerLocks faults a static socket write or a
// queued NLRI codec in each drain. TryLock asserts unwind without letting a
// broken cleanup hang the red test.
func TestInitialSyncPanicReleasesPeerLocks(t *testing.T) {
	t.Run("static", func(t *testing.T) {
		peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
		peer.settings.StaticRoutes = []StaticRoute{{
			Prefix:  netip.MustParsePrefix("192.0.2.0/24"),
			NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
		}}
		conn := &initialSyncPanicConn{}
		peer.session.conn = conn
		peer.session.bufWriter = bufio.NewWriterSize(conn, 4096)
		peer.sendInitialRoutes()
		if conn.writes != 1 {
			t.Fatalf("static socket writes = %d, want one injected panic", conn.writes)
		}
		assertInitialSyncPeerLocksReleased(t, peer)
	})
	for _, phase := range []string{"queued-initial", "queued-final"} {
		t.Run(phase, func(t *testing.T) {
			family.RegisterTestFamilies()
			fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
			peer, _ := newInitialSyncPeer(t, true, fam)
			wire, err := nlri.NewWireNLRI(fam, mustHex(t, "050118c00002"), false)
			if err != nil {
				t.Fatal(err)
			}
			codec := &initialSyncPanicNLRI{NLRI: wire}
			route := rib.NewRouteWithASPath(codec, netip.Addr{}, nil, nil)
			if phase == "queued-initial" {
				if err := peer.QueueAnnounce(route, false, false); err != nil {
					t.Fatal(err)
				}
			} else {
				peer.settings.PluginRoutes = []PluginRoute{{
					Family: fam.String(), NLRI: mustHex(t, "050118c00002"),
				}}
				conn := &initialSyncQueueConn{peer: peer, route: route}
				peer.session.conn = conn
				peer.session.bufWriter = bufio.NewWriterSize(conn, 4096)
			}
			peer.sendInitialRoutes()
			if codec.calls != 1 {
				t.Fatalf("NLRI codec calls = %d, want one injected panic", codec.calls)
			}
			assertInitialSyncPeerLocksReleased(t, peer)
		})
	}
}

func assertInitialSyncPeerLocksReleased(t *testing.T, peer *Peer) {
	t.Helper()
	if !peer.staticMu.TryLock() {
		t.Fatal("initial-sync panic retained peer.staticMu")
	}
	peer.staticMu.Unlock()
	if !peer.mu.TryLock() {
		t.Fatal("initial-sync panic retained peer.mu")
	}
	peer.mu.Unlock()
	if !peer.session.writeMu.TryLock() {
		t.Fatal("initial-sync panic retained session.writeMu")
	}
	peer.session.writeMu.Unlock()
	if peer.sendingInitialRoutes.Load() != 0 {
		t.Error("panic recovery left the initial-sync queue gate closed")
	}
	if peer.initialSyncEOROwed.Load() {
		t.Error("panic recovery left an End-of-RIB marker owed")
	}
	peer.session.closeConn()
}

// initialSyncQueuedWritePanicConn checks the unlocked send boundary before
// faulting the selected write. Earlier plugin writes admit the final-drain route.
type initialSyncQueuedWritePanicConn struct {
	initialSyncQueueConn
	t      *testing.T
	writes int
	failAt int
}

func (c *initialSyncQueuedWritePanicConn) Write(b []byte) (int, error) {
	c.writes++
	if c.peer.mu.TryLock() {
		c.peer.mu.Unlock()
	} else {
		c.t.Error("initial-sync socket write retained peer.mu")
	}
	if c.writes == c.failAt {
		panic("BUG: injected unlocked queued write failure")
	}
	return c.initialSyncQueueConn.Write(b)
}

// TestInitialSyncQueuedWritePanicRunsUnlocked controls the opposite ownership
// state: a queued socket write panics after p.mu is released. Neither drainer
// may hold p.mu across the write or attempt to release it twice on unwind.
func TestInitialSyncQueuedWritePanicRunsUnlocked(t *testing.T) {
	family.RegisterTestFamilies()
	for _, phase := range []string{"queued-initial", "queued-final"} {
		t.Run(phase, func(t *testing.T) {
			fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
			peer, _ := newInitialSyncPeer(t, true, fam)
			wire, err := nlri.NewWireNLRI(fam, mustHex(t, "050118c00002"), false)
			if err != nil {
				t.Fatal(err)
			}
			route := rib.NewRouteWithASPath(wire, netip.Addr{}, nil, nil)
			conn := &initialSyncQueuedWritePanicConn{
				peer:   peer,
				t:      t,
				failAt: 1,
			}
			if phase == "queued-initial" {
				if err := peer.QueueAnnounce(route, false, false); err != nil {
					t.Fatal(err)
				}
			} else {
				peer.settings.PluginRoutes = []PluginRoute{{
					Family: fam.String(), NLRI: mustHex(t, "050118c00002"),
				}}
				conn.route = route
				conn.failAt = 2
			}
			peer.session.conn = conn
			peer.session.bufWriter = bufio.NewWriterSize(conn, 4096)
			peer.sendInitialRoutes()
			if conn.writes != conn.failAt {
				t.Fatalf("socket writes = %d, want injected panic at %d", conn.writes, conn.failAt)
			}
			assertInitialSyncPeerLocksReleased(t, peer)
		})
	}
}
