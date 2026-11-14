package reactor

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// recordingConn is a non-blocking net.Conn whose Write appends to an in-memory
// buffer, so a test can directly observe whether (and what) a Session flushed to
// the wire. Unlike net.Pipe it needs no concurrent reader, keeping the test
// fully synchronous and deterministic.
type recordingConn struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *recordingConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(b)
}

func (c *recordingConn) written() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.buf.Bytes()...)
}

// reset forgets what was written so far, so a fixture's setup output is not
// read as the output of the step under test.
func (c *recordingConn) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf.Reset()
}

func (c *recordingConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (c *recordingConn) Close() error                     { return nil }
func (c *recordingConn) LocalAddr() net.Addr              { return testConnAddr{} }
func (c *recordingConn) RemoteAddr() net.Addr             { return testConnAddr{} }
func (c *recordingConn) SetDeadline(time.Time) error      { return nil }
func (c *recordingConn) SetReadDeadline(time.Time) error  { return nil }
func (c *recordingConn) SetWriteDeadline(time.Time) error { return nil }

type testConnAddr struct{}

func (testConnAddr) Network() string { return "test" }
func (testConnAddr) String() string  { return "10.0.0.2:179" }

// newEORGuardPeer builds an Established peer wired to an Established Session that
// is backed by a recordingConn. The AnnounceEOR ordering-guard tests then assert
// directly on the bytes flushed to the wire (an End-of-RIB or nothing), rather
// than inferring intent from an error.
func newEORGuardPeer(t *testing.T) (*reactorAPIAdapter, *Peer, *recordingConn) {
	t.Helper()
	peer, conn := newEstablishedRecordingPeer(t, "10.0.0.2", family.IPv4Unicast)
	adapter := &reactorAPIAdapter{r: &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[netip.AddrPort]*Peer{peer.settings.PeerKey(): peer},
	}}
	return adapter, peer, conn
}

// newEstablishedRecordingPeer builds an Established peer at address that
// negotiated exactly families, its session writing into a recording conn.
func newEstablishedRecordingPeer(t *testing.T, address string, families ...family.Family) (*Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr(address),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	negotiated := make(map[family.Family]bool, len(families))
	for _, fam := range families {
		negotiated[fam] = true
	}
	peer.negotiated.Store(&NegotiatedCapabilities{families: negotiated})

	// Drive a Session to Established, then back it with the recording conn.
	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())

	conn := &recordingConn{}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	return peer, conn
}

// eorWire returns the on-wire encoding of the End-of-RIB for a family.
func eorWire(fam family.Family) []byte {
	var buf [64]byte
	n := message.BuildEOR(fam).WriteTo(buf[:], 0, nil)
	return buf[:n]
}

// TestAnnounceEOR_SkipsPeerInInitialRouteSync pins the EoR-ordering guard
// (commit 99c943404) on a reactor without a forward pool, which only a test
// builds: there an external AnnounceEOR (plugin / route-server) must NOT write
// an End-of-RIB to a peer that is still draining its initial-route opQueue,
// because no queue can hold the marker behind the still-queued route NLRI. With
// a pool the request waits in the forward queue instead
// (TestExplicitEndOfRIBDuringSyncFollowsQueuedRoutes).
//
// VALIDATES: while shouldQueue()==true, AnnounceEOR succeeds but flushes nothing
// to the wire (the peer is skipped, its EoR deferred to sendInitialRoutes).
// PREVENTS: regression of the guard -- without it the EoR is flushed here,
// landing on the wire ahead of the queued routes.
func TestAnnounceEOR_SkipsPeerInInitialRouteSync(t *testing.T) {
	adapter, peer, conn := newEORGuardPeer(t)

	// Simulate "initial route sync in progress" -> shouldQueue() == true.
	peer.setTestInitialSync(1, false)
	require.True(t, peer.shouldQueue(), "precondition: peer must be in initial-sync state")

	err := adapter.AnnounceEOR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender())
	require.NoError(t, err)
	require.Empty(t, conn.written(),
		"no End-of-RIB may reach the wire while the peer drains its initial-route queue")
}

// TestAnnounceEOR_SendsWhenNotInInitialSync confirms the skip is specific to the
// initial-sync window, not unconditional. A normally-established peer (no
// opQueue, not syncing) is sent the EoR, which lands on the wire.
//
// VALIDATES: while shouldQueue()==false, AnnounceEOR flushes the exact End-of-RIB.
// PREVENTS: an over-broad guard that silently drops every external EoR.
func TestAnnounceEOR_SendsWhenNotInInitialSync(t *testing.T) {
	adapter, peer, conn := newEORGuardPeer(t)

	require.False(t, peer.shouldQueue(), "precondition: peer must NOT be in initial-sync state")

	err := adapter.AnnounceEOR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender())
	require.NoError(t, err)
	require.Equal(t, eorWire(family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}), conn.written(),
		"a not-syncing peer must receive the End-of-RIB on the wire")
}

// TestAnnounceEORDefersToCurrentSessionMarker pins the second half of the
// AnnounceEOR ordering guard on a reactor without a forward pool, which only a
// test builds: the End-of-RIB the current session's initial sync still owes
// (Session.initialSyncEOROwed, read by initialSyncMarkerOwed). With a pool a
// requested marker is queued instead, and only a replay's stands down
// (TestReplayEndOfRIBStandsDownWhileSyncOwesItsMarker).
//
// VALIDATES: AC-7. The session's queueing gate is open, so shouldQueue alone
// would let the marker through, but its own marker is still owed: AnnounceEOR
// succeeds and writes nothing. Once that marker is no longer owed, the same call
// writes the End-of-RIB, the family not being claimed yet.
// PREVENTS: a route server's marker overtaking the one the initial sync owes,
// in the window after the sync opened its gate and before its End-of-RIB loop
// ran; and a marker withheld for good once the sync owes nothing.
func TestAnnounceEORDefersToCurrentSessionMarker(t *testing.T) {
	adapter, peer, conn := newEORGuardPeer(t)
	session := peer.setTestInitialSync(0, true)
	require.False(t, peer.shouldQueue(), "precondition: the session's queueing gate must be open")

	ipv4 := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	err := adapter.AnnounceEOR(selector.All(), uint16(ipv4.AFI), uint8(ipv4.SAFI), plugin.OperatorSender())
	require.NoError(t, err)
	require.Empty(t, conn.written(),
		"no End-of-RIB may reach the wire while the current session's own marker is owed")

	session.initialSyncEOROwed.Store(false)
	err = adapter.AnnounceEOR(selector.All(), uint16(ipv4.AFI), uint8(ipv4.SAFI), plugin.OperatorSender())
	require.NoError(t, err)
	require.Equal(t, eorWire(ipv4), conn.written(),
		"once the current session owes no marker, AnnounceEOR must write the End-of-RIB")
}

// newEORSyncRail is the destination of the End-of-RIB tests below, on a
// reactor with a forward pool running the production batch handler
// (newSyncOrderRail): the state enterEstablished leaves before the initial sync
// runs, its queueing gate closed and its own marker owed. manualEOR sets
// `behavior { manual-eor true }`.
func newEORSyncRail(t *testing.T, manualEOR bool) (*reactorAPIAdapter, *Peer, *recordingConn) {
	t.Helper()
	r, _, dst, conn, _ := newSyncOrderRail(t)
	dst.settings.ManualEOR = manualEOR
	dst.resetAPISync(nil)
	dst.testInitialSync().initialSyncEOROwed.Store(true)
	return &reactorAPIAdapter{r: r}, dst, conn
}

// eorFamilyClaimed reports whether the IPv4 unicast End-of-RIB is claimed for
// the peer's session (Peer.claimInitialSyncEOR), without taking the claim.
func eorFamilyClaimed(peer *Peer) bool {
	peer.mu.RLock()
	defer peer.mu.RUnlock()
	return peer.initialSyncEOR[family.IPv4Unicast]
}

// TestExplicitEndOfRIBDuringSyncFollowsQueuedRoutes proves that an End-of-RIB
// asked for while the peer's initial sync still runs is written, after the
// route operation queued for that peer.
//
// VALIDATES: AC-5. With the queueing gate closed, AnnounceEOR answers no error
// and the marker waits in the peer's forward queue, with nothing written. Once
// the sync drains the queued announce and opens the gate, the worker writes the
// marker behind it, and it is the session's one marker: the sync's own phase
// finds the family claimed.
// PREVENTS: a requested marker dropped while answered as sent
// (plan/journal/guard-addition-drops-what-it-refuses.md, 2026-11-14).
//
// Method: the destination's connection is the record of the wire.
func TestExplicitEndOfRIBDuringSyncFollowsQueuedRoutes(t *testing.T) {
	adapter, dst, conn := newEORSyncRail(t, false)
	require.NoError(t, dst.QueueAnnounce(testRoute(syncOrderPrefix), bgptypes.NextHopExplicit, false))

	require.NoError(t, adapter.AnnounceEOR(selector.Addr(dst.Settings().Address),
		uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender()))
	require.True(t, dst.forwardOverflowPending(), "the requested marker must wait in the peer's forward queue")
	require.Empty(t, conn.written(), "nothing may reach the wire while the queueing gate is closed")

	dst.runInitialSyncNow()

	var seen []wireUpdate
	require.Eventually(t, func() bool {
		seen = parseWireUpdates(t, conn.written())
		return len(seen) >= 2
	}, 5*time.Second, 5*time.Millisecond, "the queued announce and the requested marker must reach the wire")
	require.Never(t, func() bool {
		return len(parseWireUpdates(t, conn.written())) > 2
	}, 200*time.Millisecond, 5*time.Millisecond, "the session carries one marker for the family")
	require.Equal(t, []string{"announce", "end-of-rib"}, wireKinds(seen), "wire order")
	require.Equal(t, uint32(1), dst.counters.eorSent.Load(), "the marker is metered once, when written")
}

// TestReplayEndOfRIBStandsDownWhileSyncOwesItsMarker proves that a replay's
// End-of-RIB asked for while the session's initial sync still owes its own
// marker writes nothing and leaves the family to the sync.
//
// VALIDATES: AC-5's permitted survivor. AnnounceReplayEOR answers no error,
// queues nothing and takes no claim; the sync then writes the family's one
// automatic marker.
// PREVENTS: a replay's marker standing in for the sync's, which a replay that
// finished before Ze's own table could put on the wire ahead of that table's
// end.
//
// Method: the sync writes its marker on the calling goroutine
// (runInitialSyncNow), so the wire holds it once the sync returns.
func TestReplayEndOfRIBStandsDownWhileSyncOwesItsMarker(t *testing.T) {
	adapter, dst, conn := newEORSyncRail(t, false)

	require.NoError(t, adapter.AnnounceReplayEOR(selector.Addr(dst.Settings().Address),
		uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender()),
		"a replay's marker that stands down is answered as handled")
	require.False(t, dst.forwardOverflowPending(), "a replay's marker must not be queued while the sync owes its own")
	require.False(t, eorFamilyClaimed(dst), "a replay's marker that stood down must leave the family to the sync")

	dst.runInitialSyncNow()

	require.Equal(t, eorWire(family.IPv4Unicast), conn.written(), "the sync's own marker is the one marker")
	require.Never(t, func() bool {
		return len(parseWireUpdates(t, conn.written())) > 1
	}, 200*time.Millisecond, 5*time.Millisecond, "no second marker may follow")
	require.Equal(t, uint32(1), dst.counters.eorSent.Load(), "one marker per family per session")
}

// TestManualEORPeerReceivesOnlyTheRequestedMarker proves that a peer with
// `behavior { manual-eor true }` receives the End-of-RIB a process asks for
// during the initial sync, after the queued route, and no automatic marker.
//
// VALIDATES: AC-6. A replay's marker before the sync, the sync itself and a
// replay's marker after it write nothing; the requested marker is written once.
// PREVENTS: the requested marker dropped during the sync while the sync
// withholds its own, which left the peer with no marker at all and the caller
// told it was sent.
//
// Method: the destination's connection is the record of the wire.
func TestManualEORPeerReceivesOnlyTheRequestedMarker(t *testing.T) {
	adapter, dst, conn := newEORSyncRail(t, true)
	require.NoError(t, dst.QueueAnnounce(testRoute(syncOrderPrefix), bgptypes.NextHopExplicit, false))
	sel := selector.Addr(dst.Settings().Address)
	ipv4, unicast := uint16(family.AFIIPv4), uint8(family.SAFIUnicast)

	require.NoError(t, adapter.AnnounceReplayEOR(sel, ipv4, unicast, plugin.OperatorSender()))
	require.NoError(t, adapter.AnnounceEOR(sel, ipv4, unicast, plugin.OperatorSender()))
	dst.runInitialSyncNow()

	var seen []wireUpdate
	require.Eventually(t, func() bool {
		seen = parseWireUpdates(t, conn.written())
		return len(seen) >= 2
	}, 5*time.Second, 5*time.Millisecond, "the queued announce and the requested marker must reach the wire")

	require.NoError(t, adapter.AnnounceReplayEOR(sel, ipv4, unicast, plugin.OperatorSender()))
	require.Never(t, func() bool {
		return len(parseWireUpdates(t, conn.written())) > 2
	}, 200*time.Millisecond, 5*time.Millisecond, "no automatic marker may reach a manual-eor peer")
	require.Equal(t, []string{"announce", "end-of-rib"}, wireKinds(seen), "wire order")
	require.Equal(t, uint32(1), dst.counters.eorSent.Load(), "exactly the requested marker")
}

// TestManualEORPeerWithholdsAReplayEndOfRIBAfterTheSync proves that a replay's
// End-of-RIB stands down on a `manual-eor` peer once the initial sync has
// ended, while the family is still unclaimed.
//
// VALIDATES: D-2. AnnounceReplayEOR answers no error, queues nothing, takes no
// claim and writes nothing.
// PREVENTS: a replay that finishes after the sync claiming the family and
// sending a marker the operator asked Ze not to send.
func TestManualEORPeerWithholdsAReplayEndOfRIBAfterTheSync(t *testing.T) {
	adapter, dst, conn := newEORSyncRail(t, true)
	dst.setTestInitialSync(0, false)

	require.NoError(t, adapter.AnnounceReplayEOR(selector.Addr(dst.Settings().Address),
		uint16(family.AFIIPv4), uint8(family.SAFIUnicast), plugin.OperatorSender()),
		"a replay's marker that stands down is answered as handled")
	require.False(t, eorFamilyClaimed(dst), "a withheld replay marker must take no claim")
	require.Never(t, func() bool {
		return len(conn.written()) > 0
	}, 200*time.Millisecond, 5*time.Millisecond, "a manual-eor peer must receive no replay marker")
	require.Zero(t, dst.counters.eorSent.Load(), "nothing was written")
}

// TestFailedEndOfRIBWriteHandsBackOnlyAnOwnedClaim proves that a queued
// End-of-RIB whose write failed hands back the family's claim only when it took
// that claim.
//
// VALIDATES: settleEndOfRIB reads fwdEndOfRIB.ownsClaim. A requested marker
// queued while another producer holds the claim leaves it held; one that took
// the claim releases it.
// PREVENTS: a failed requested marker releasing the claim the initial sync or
// a replay holds, which let a second automatic marker out for the family.
func TestFailedEndOfRIBWriteHandsBackOnlyAnOwnedClaim(t *testing.T) {
	peer, _ := newEstablishedRecordingPeer(t, "10.0.0.2", family.IPv4Unicast)
	session := peer.session
	fam := family.IPv4Unicast
	require.True(t, peer.claimInitialSyncEOR(fam), "precondition: another producer holds the claim")

	settleEndOfRIB(peer, session, []fwdItem{{endOfRIB: &fwdEndOfRIB{session: session, family: fam}}}, false)
	require.True(t, eorFamilyClaimed(peer), "a marker that did not take the claim must not hand it back")

	settleEndOfRIB(peer, session, []fwdItem{{endOfRIB: &fwdEndOfRIB{session: session, family: fam, ownsClaim: true}}}, false)
	require.False(t, eorFamilyClaimed(peer), "a marker that took the claim hands it back when its write failed")
}
