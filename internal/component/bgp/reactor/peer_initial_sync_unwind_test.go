// Design: docs/architecture/behavior/peer-lifecycle.md -- initial-sync panic recovery.
package reactor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
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

// wireThroughSocketBoundary connects session to conn as connectionEstablished
// does: bufWriter writes through an observedBGPWriter, the socket boundary
// that marks a write a panic interrupted (Session.discardInterruptedWrite).
func wireThroughSocketBoundary(session *Session, conn net.Conn) {
	session.conn = conn
	session.wireWriter = &observedBGPWriter{writer: conn, session: session}
	session.bufWriter = bufio.NewWriterSize(session.wireWriter, 4096)
}

// ceaseUnspecificWire is the NOTIFICATION a recovered sync panic sends.
// RFC 4271 Section 4.1: the type octet follows the 16-octet marker and the
// 2-octet length; RFC 4271 Section 4.5: the error code, then the subcode.
func ceaseUnspecificWire() []byte {
	return []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x15, byte(msgtype.TypeNOTIFICATION),
		byte(message.NotifyCease), message.NotifyCeaseUnspecific,
	}
}

// assertWireEndsWithCease asserts the wire holds exactly taken, what the
// socket took before the injected fault, then the Cease / Unspecific
// NOTIFICATION: nothing of the message whose socket write panicked.
func assertWireEndsWithCease(t *testing.T, wire, taken []byte) {
	t.Helper()
	want := append(slices.Clone(taken), ceaseUnspecificWire()...)
	if !bytes.Equal(wire, want) {
		t.Errorf("wire after a recovered socket-write panic = %x, want what the socket took before it (%x) then only the Cease / Unspecific NOTIFICATION",
			wire, taken)
	}
}

// assertOneUpdateFrame asserts frame is one whole UPDATE. RFC 4271 Section
// 4.1: a 16-octet marker, the 2-octet length of the whole message, then the
// type octet.
func assertOneUpdateFrame(t *testing.T, frame []byte) {
	t.Helper()
	if len(frame) < message.HeaderLen {
		t.Fatalf("socket took %x before the fault, want one whole UPDATE", frame)
	}
	if length := int(binary.BigEndian.Uint16(frame[16:18])); length != len(frame) {
		t.Fatalf("socket took %d octets before the fault, its header says %d: want one whole UPDATE", len(frame), length)
	}
	if msgtype.MessageType(frame[18]) != msgtype.TypeUPDATE {
		t.Fatalf("socket took a type %d message before the fault, want an UPDATE", frame[18])
	}
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
			wireThroughSocketBoundary(session, conn)

			peer.runInitialSyncNow()

			// The injected panic, then the recovery's NOTIFICATION, which ends
			// the session (endSessionAfterSyncPanic); nothing more of the sync.
			if conn.writes != 2 {
				t.Fatalf("socket writes = %d, want the injected panic then the NOTIFICATION", conn.writes)
			}
			// The fault was the first socket write, so the socket took nothing
			// before it: the interrupted UPDATE or End-of-RIB stays off the wire.
			assertWireEndsWithCease(t, conn.written(), nil)
			if peer.testInitialSync().sendingInitialRoutes.Load() != 0 {
				t.Error("panic recovery left the initial-sync queue gate closed")
			}
			if peer.testInitialSync().initialSyncEOROwed.Load() {
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
		if err := c.peer.QueueAnnounce(c.route, bgptypes.NextHopExplicit, false); err != nil {
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
		wireThroughSocketBoundary(peer.session, conn)
		peer.runInitialSyncNow()
		// The injected panic, then the recovery's NOTIFICATION.
		if conn.writes != 2 {
			t.Fatalf("static socket writes = %d, want the injected panic then the NOTIFICATION", conn.writes)
		}
		// The fault was the first socket write: the static UPDATE stays off the wire.
		assertWireEndsWithCease(t, conn.written(), nil)
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
				if err := peer.QueueAnnounce(route, bgptypes.NextHopExplicit, false); err != nil {
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
			peer.runInitialSyncNow()
			if codec.calls != 1 {
				t.Fatalf("NLRI codec calls = %d, want one injected panic", codec.calls)
			}
			assertInitialSyncPeerLocksReleased(t, peer)
		})
	}
}

// TestInitialSyncPanicEndsTheSession pins AC-7: a sync that recovers from a
// panic with an operation still queued ends its session rather than leaving an
// Established peer whose queue nothing drains. Method: a queued route whose NLRI
// codec panics in the first drain, through the real sendInitialRoutes. The
// session is ended as an automatic stop: the peer leaves Established, the wire
// carries exactly one Cease / Unspecific NOTIFICATION and no End-of-RIB, and
// the queue still holds the route for the next session's sync.
func TestInitialSyncPanicEndsTheSession(t *testing.T) {
	family.RegisterTestFamilies()
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	peer, conn := newInitialSyncPeer(t, true, fam)
	wire, err := nlri.NewWireNLRI(fam, mustHex(t, "050118c00002"), false)
	if err != nil {
		t.Fatal(err)
	}
	codec := &initialSyncPanicNLRI{NLRI: wire}
	route := rib.NewRouteWithASPath(codec, netip.Addr{}, nil, nil)
	if err := peer.QueueAnnounce(route, bgptypes.NextHopExplicit, false); err != nil {
		t.Fatal(err)
	}

	peer.runInitialSyncNow()

	if codec.calls != 1 {
		t.Fatalf("NLRI codec calls = %d, want one injected panic", codec.calls)
	}
	if state := peer.State(); state == PeerStateEstablished {
		t.Errorf("peer state = %v after a recovered sync panic, want the session ended", state)
	}
	notification := ceaseUnspecificWire()
	if got := conn.written(); !bytes.Equal(got, notification) {
		t.Errorf("wire after a recovered sync panic = %x, want only the Cease / Unspecific NOTIFICATION %x (no End-of-RIB)",
			got, notification)
	}
	peer.mu.Lock()
	queued := slices.Clone(peer.opQueue)
	peer.mu.Unlock()
	if len(queued) != 1 {
		t.Fatalf("opQueue holds %d operations after the recovered panic, want the 1 the next session owes", len(queued))
	}
	if queued[0].Type != PeerOpAnnounce {
		t.Errorf("queued operation type = %v, want the announce the next session owes", queued[0].Type)
	}
	if queued[0].Route != route {
		t.Error("queued operation is not the route queued before the panic")
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
	if peer.testInitialSync().sendingInitialRoutes.Load() != 0 {
		t.Error("panic recovery left the initial-sync queue gate closed")
	}
	if peer.testInitialSync().initialSyncEOROwed.Load() {
		t.Error("panic recovery left an End-of-RIB marker owed")
	}
	peer.session.closeConn()
}

// initialSyncQueuedWritePanicConn checks the unlocked send boundary before
// faulting the selected write. Earlier plugin writes admit the final-drain route.
// taken is what the socket had taken when the fault fired.
type initialSyncQueuedWritePanicConn struct {
	initialSyncQueueConn
	t      *testing.T
	writes int
	failAt int
	taken  []byte
}

func (c *initialSyncQueuedWritePanicConn) Write(b []byte) (int, error) {
	c.writes++
	if c.peer.mu.TryLock() {
		c.peer.mu.Unlock()
	} else {
		c.t.Error("initial-sync socket write retained peer.mu")
	}
	if c.writes == c.failAt {
		c.taken = c.written()
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
				if err := peer.QueueAnnounce(route, bgptypes.NextHopExplicit, false); err != nil {
					t.Fatal(err)
				}
			} else {
				peer.settings.PluginRoutes = []PluginRoute{{
					Family: fam.String(), NLRI: mustHex(t, "050118c00002"),
				}}
				conn.route = route
				conn.failAt = 2
			}
			wireThroughSocketBoundary(peer.session, conn)
			peer.runInitialSyncNow()
			// The injected panic, then the recovery's NOTIFICATION, which is
			// also written with p.mu released.
			if conn.writes != conn.failAt+1 {
				t.Fatalf("socket writes = %d, want injected panic at %d then the NOTIFICATION", conn.writes, conn.failAt)
			}
			// queued-initial faults the first socket write, so nothing came
			// before it; queued-final faults the second, after the plugin UPDATE.
			if phase == "queued-initial" {
				if len(conn.taken) != 0 {
					t.Fatalf("socket took %x before the first write's fault, want nothing", conn.taken)
				}
			} else {
				assertOneUpdateFrame(t, conn.taken)
			}
			assertWireEndsWithCease(t, conn.written(), conn.taken)
			assertInitialSyncPeerLocksReleased(t, peer)
		})
	}
}
