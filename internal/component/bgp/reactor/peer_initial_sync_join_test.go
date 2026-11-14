// Design: docs/architecture/behavior/peer-lifecycle.md -- the initial sync, one goroutine per session, joined when its session ends
// Related: peer_initial_sync.go -- sendInitialRoutes, the goroutine these tests hold and let go
// Related: peer_run.go -- runOnce and its session-end defer
// Related: rfc4271_automatic_start_peer_test.go -- automaticStartNeighbor, the far end each test plays by hand
// Related: peer_run_backoff_test.go -- backoffCease, the message that ends each session

package reactor

import (
	"log/slog"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/clock"
)

// syncJoinProbe is how long a test watches for something that MUST NOT happen
// while it holds an initial sync: the run loop entering its reconnect wait, or
// the peer reading as settled. Without the join, either happens within
// milliseconds of its cause, so the probe sees it on a loaded host. With the
// join, neither can happen until the test lets the sync go, so the probe can
// make a test slower and never makes it flaky.
const syncJoinProbe = 500 * time.Millisecond

// syncJoinClock is the peer clock of these tests. Peer.run's reconnect wait and
// the peer-up barrier wait (Peer.waitPeerUpBarrier) are the only callers of
// After on a peer without a reactor. The barrier wait, recognized by its
// duration peerUpBarrierTimeout, gets a channel only the test fires, so a sync
// held there stays there until the test says, on any host. Every other After
// is a reconnect wait: its duration is recorded and it ends when the test
// releases it. Now, timers and sleeps stay on the real clock, so the session's
// own timers keep working. Safe for concurrent use.
type syncJoinClock struct {
	clock.RealClock

	// waits receives the duration of every reconnect wait, fire ends one.
	waits chan time.Duration
	fire  chan time.Time

	// barrierEntered is signaled each time a sync enters the barrier wait,
	// barrierFire ends that wait as its timeout would.
	barrierEntered chan struct{}
	barrierFire    chan time.Time
}

func newSyncJoinClock() *syncJoinClock {
	return &syncJoinClock{
		waits:          make(chan time.Duration, 64),
		fire:           make(chan time.Time, 1),
		barrierEntered: make(chan struct{}, 8),
		barrierFire:    make(chan time.Time, 1),
	}
}

// After hands the barrier wait the test's barrier channel, and records every
// other wait before it hands it the reconnect channel.
func (c *syncJoinClock) After(d time.Duration) <-chan time.Time {
	if d == peerUpBarrierTimeout {
		select {
		case c.barrierEntered <- struct{}{}:
		default: // eight barrier waits is more than any test here makes; a dropped signal fails awaitBarrier
		}
		return c.barrierFire
	}
	select {
	case c.waits <- d:
	default: // 64 waits is more than any test here makes; a dropped record fails awaitWait
	}
	return c.fire
}

// awaitWait returns the duration of the next reconnect wait the run loop entered.
func (c *syncJoinClock) awaitWait(t *testing.T) time.Duration {
	t.Helper()
	select {
	case d := <-c.waits:
		return d
	case <-time.After(10 * time.Second):
		t.Fatal("the peer never entered a reconnect wait")
		return 0
	}
}

// release ends the reconnect wait the run loop is in. MUST be called once per
// wait awaitWait returned.
func (c *syncJoinClock) release() {
	c.fire <- time.Now()
}

// awaitBarrier returns once a sync has entered the peer-up barrier wait.
func (c *syncJoinClock) awaitBarrier(t *testing.T) {
	t.Helper()
	select {
	case <-c.barrierEntered:
	case <-time.After(10 * time.Second):
		t.Fatal("the initial sync never entered the peer-up barrier wait")
	}
}

// fireBarrier ends the barrier wait as its timeout would. It never blocks: a
// fire still pending is the same release.
func (c *syncJoinClock) fireBarrier() {
	select {
	case c.barrierFire <- time.Now():
	default: // a release is already pending
	}
}

// startSyncJoinNeighbor runs a dial-only Peer on a syncJoinClock against a
// listener the test accepts on, with the reconnect delay at 5 seconds to one
// minute (the waits are the clock's, so none is real).
func startSyncJoinNeighbor(t *testing.T) (*automaticStartNeighbor, *syncJoinClock) {
	t.Helper()
	return startSyncJoinNeighborWith(t, func(*PeerSettings) {})
}

// startSyncJoinNeighborWith is startSyncJoinNeighbor with configure applied to
// the peer's settings before the peer starts, for a test whose routes must
// reach the wire (an export policy, which RFC 8212 requires of an EBGP peer).
func startSyncJoinNeighborWith(t *testing.T, configure func(*PeerSettings)) (*automaticStartNeighbor, *syncJoinClock) {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // Test cleanup.
	tcpListener, ok := ln.(*net.TCPListener)
	require.True(t, ok, "Listen(\"tcp4\") returns a TCPListener")
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address must be TCP")

	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Port = uint16(addr.Port) //nolint:gosec // A listener port fits 16 bits.
	settings.Connection = ConnectionActive
	configure(settings)
	peer := NewPeer(settings)
	clk := newSyncJoinClock()
	peer.SetClock(clk)
	peer.setReconnectDelay(5*time.Second, time.Minute)
	t.Cleanup(startAndStop(t, peer))
	// Registered after startAndStop, so it runs first: a barrier wait the test
	// never let go would otherwise hold the stop.
	t.Cleanup(clk.fireBarrier)
	return &automaticStartNeighbor{peer: peer, listener: tcpListener}, clk
}

// staticHold holds the peer's staticMu, the lock the initial sync takes for its
// static phase (sendInitialRoutes). A sync started while it is held stops at
// the first statement of that phase and goes no further. Used from the test
// goroutine only.
type staticHold struct {
	peer *Peer
	held bool
}

// holdStaticPhase takes staticMu and lets it go at cleanup if the test did not.
// Registered after startSyncJoinNeighbor's cleanups, so it runs before the stop.
func holdStaticPhase(t *testing.T, peer *Peer) *staticHold {
	t.Helper()
	hold := &staticHold{peer: peer, held: true}
	peer.staticMu.Lock()
	t.Cleanup(hold.release)
	return hold
}

// release lets the held sync go into its static phase. A second call does
// nothing.
func (h *staticHold) release() {
	if !h.held {
		return
	}
	h.held = false
	h.peer.staticMu.Unlock()
}

// awaitSessionsEstablished returns once the peer's FSM history records
// sessions entries into Established. The record is appended at the end of the
// Established callback (runOnce), after the peer-up barrier was reset for that
// session and its initial sync was started, so a test that has seen it may arm
// the barrier for that sync.
func awaitSessionsEstablished(t *testing.T, peer *Peer, sessions int) {
	t.Helper()
	require.Eventually(t, func() bool {
		established := 0
		for _, transition := range peer.fSMHistory() {
			if transition.Reason == "established" {
				established++
			}
		}
		return established >= sessions
	}, 5*time.Second, time.Millisecond, "the Established callback of session %d never finished", sessions)
}

// TestRunOnceJoinsInitialSyncBeforeReturn is the wiring test of the join, from
// the FSM callback that starts a session's initial sync to the session-end
// defer of runOnce that waits for it.
//
// VALIDATES: AC-1, its first clause. A session reaches Established while the
// test holds the static phase of its initial sync, and the far end then ends it
// with a Cease. runOnce does not return, so Peer.run enters no reconnect wait,
// until the test lets the sync go. Once it does, the wait follows.
// PREVENTS: an initial sync that outlives its session. Nothing joined it, so
// runOnce returned at once and the next session could start while the old
// goroutine had still to drain the queue and write plugin routes and End-of-RIB
// on whatever session the peer held by then
// (plan/journal/late-write-lands-on-the-successor.md, 2026-08-31).
func TestRunOnceJoinsInitialSyncBeforeReturn(t *testing.T) {
	n, clk := startSyncJoinNeighbor(t)
	static := holdStaticPhase(t, n.peer)

	conn := n.accept(t)
	n.establish(t, conn, 90)
	awaitSessionsEstablished(t, n.peer, 1)
	backoffCease(t, conn)

	select {
	case d := <-clk.waits:
		t.Fatalf("runOnce returned, and the run loop entered a %s reconnect wait, while the initial sync "+
			"of the session that ended was still held in its static phase: nothing joins the sync goroutine", d)
	case <-time.After(syncJoinProbe):
	}

	static.release()
	clk.awaitWait(t)
}

// TestSuccessorPendingSyncUntilOwnMarker pins what `request quiesce` reads
// across a rapid reconnect: pendingSync, the predicate behind DrainPeerSync
// (reactor_api.go).
//
// VALIDATES: AC-2. The first session's sync is held in the peer-up barrier wait
// when the far end ends that session with a Cease, and the peer reconnects.
// While the second session's own sync is held before its static phase, the peer
// reads as pending for the whole probe; once that sync runs, it reads as
// settled.
// PREVENTS: the first session's sync, let go after the second session reached
// Established, settling the second one. Its last stores cleared the Peer-level
// queue gate and End-of-RIB-owed fact that the second establishment had set,
// so `request quiesce` returned before the second session's initial update, and
// its End-of-RIB went out on the second session's connection.
func TestSuccessorPendingSyncUntilOwnMarker(t *testing.T) {
	n, clk := startSyncJoinNeighbor(t)

	// The first sync is held in the barrier wait. The barrier is armed while the
	// static phase is held, so that sync reads the barrier only once it is armed.
	first := holdStaticPhase(t, n.peer)
	firstConn := n.accept(t)
	n.establish(t, firstConn, 90)
	awaitSessionsEstablished(t, n.peer, 1)
	n.peer.SetPeerUpBarrier(1)
	first.release()
	clk.awaitBarrier(t)

	// From here on, the second session's sync is held before its static phase.
	second := holdStaticPhase(t, n.peer)
	backoffCease(t, firstConn)

	// The first session ends. Without a join the run loop waits at once; with a
	// join whose barrier wait does not see the session end, it waits only once
	// the barrier lets the first sync go, which the test then does as the
	// timeout would.
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
	if !fired {
		// Without a join, the first session's sync is still in its barrier wait.
		// This lets it go while the second session's sync is held.
		clk.fireBarrier()
	}

	require.Never(t, func() bool { return !n.peer.pendingSync() }, syncJoinProbe, time.Millisecond,
		"pendingSync read the peer as settled while the second session's own initial sync had not "+
			"begun: the first session's sync settled the session that replaced it")

	second.release()
	require.Eventually(t, func() bool { return !n.peer.pendingSync() }, 5*time.Second, time.Millisecond,
		"the second session's own initial sync never settled the peer")
}

// awaitInitialSyncDone returns once session's initial-sync goroutine has
// signaled its end (Session.initialSyncDone).
func awaitInitialSyncDone(t *testing.T, session *Session) {
	t.Helper()
	select {
	case <-session.initialSyncDone:
	case <-time.After(5 * time.Second):
		t.Fatal("the session's initial sync goroutine never ended")
	}
}

// TestInitialSyncStartsOncePerSession pins the start latch of a session's
// initial sync (spawnInitialSync, Session.claimInitialSyncStart).
//
// VALIDATES: AC-3. The start door is called a second time for a session whose
// sync already ran, as a convergence-hold release after the FSM callback would.
// The second call starts no goroutine, so the sync does not run again; a later
// session of the same peer still runs its own.
// METHOD: the peer has no negotiated capabilities, so one run of the sync moves
// the session's gate from closed (1) to open (0) and signals its end. After the
// first run the test closes the gate again and gives the session a fresh end
// signal: a second goroutine would open that gate and close that signal.
// PREVENTS: two sync goroutines for one session, each writing a marker for the
// same initial routing update (RFC 4724 Section 4), and the second one landing
// after the routes it claims to end.
func TestInitialSyncStartsOncePerSession(t *testing.T) {
	peer := NewPeer(NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101))
	session := peer.setTestInitialSync(1, true)

	peer.spawnInitialSync(session)
	awaitInitialSyncDone(t, session)
	require.Equal(t, int32(0), session.sendingInitialRoutes.Load(), "the first start never ran the sync")

	session.initialSyncDone = make(chan struct{})
	session.sendingInitialRoutes.Store(1)
	peer.spawnInitialSync(session)
	select {
	case <-session.initialSyncDone:
		t.Fatal("a second start of the same session spawned a second initial-sync goroutine")
	case <-time.After(syncJoinProbe):
	}
	require.Equal(t, int32(1), session.sendingInitialRoutes.Load(),
		"a second start of the same session ran its initial sync again")

	// A later session of the same peer has its own latch, so its sync runs.
	successor := NewSession(peer.settings)
	successor.sendingInitialRoutes.Store(1)
	peer.spawnInitialSync(successor)
	awaitInitialSyncDone(t, successor)
	require.Equal(t, int32(0), successor.sendingInitialRoutes.Load(),
		"the latch of an earlier session stopped the next session's sync")
}

// captureRoutesLog points the bgp.routes logger at a buffer for the test and
// restores it at cleanup. Call it BEFORE startSyncJoinNeighbor, so the restore
// runs after the peer has stopped and no goroutine of the peer reads the
// logger while it is put back.
func captureRoutesLog(t *testing.T) *syncBuffer {
	t.Helper()
	sink := &syncBuffer{}
	t.Cleanup(swapRoutesLogger(slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn}))))
	return sink
}

// awaitSessionEnd returns once session's Run has returned (Session.Done).
func awaitSessionEnd(t *testing.T, session *Session) {
	t.Helper()
	select {
	case <-session.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the session never ended after the far end's Cease")
	}
}

// TestInitialSyncJoinReturnsWhenBarrierNeverOpens pins what ends the peer-up
// barrier wait of a session that ends (Peer.waitPeerUpBarrier).
//
// VALIDATES: AC-5. The barrier is armed for the first session's sync and never
// opens: no plugin acknowledges, and the barrier timeout is a channel only the
// test fires (syncJoinClock), which it never does, so the clock never reaches
// the 2 seconds. The far end ends the session with a Cease. The run loop then
// enters its reconnect wait, so runOnce joined the sync; that sync cleared its
// own session's gate and owed marker, and no barrier-timeout warning was logged.
// PREVENTS: a join that waits for the barrier timeout on every session that
// ends while a barrier plugin still owes its acknowledgement, which holds the
// peer between sessions for 2 seconds each time, or for ever where the timeout
// cannot fire.
func TestInitialSyncJoinReturnsWhenBarrierNeverOpens(t *testing.T) {
	routesLog := captureRoutesLog(t)
	n, clk := startSyncJoinNeighbor(t)
	static := holdStaticPhase(t, n.peer)

	conn := n.accept(t)
	n.establish(t, conn, 90)
	awaitSessionsEstablished(t, n.peer, 1)
	session := n.peer.currentSession()
	n.peer.SetPeerUpBarrier(1)
	static.release()
	clk.awaitBarrier(t)

	backoffCease(t, conn)
	clk.awaitWait(t)

	awaitInitialSyncDone(t, session)
	require.Equal(t, int32(0), session.sendingInitialRoutes.Load(),
		"the sync that ended with its session left that session's queue gate closed")
	require.False(t, session.initialSyncEOROwed.Load(),
		"the sync that ended with its session left that session's End-of-RIB owed")
	require.NotContains(t, routesLog.String(), "peer-up barrier timeout",
		"the barrier wait reported a timeout for a session that ended")
}

// TestBarrierTimeoutStillWarnsOnLiveSession pins the bound the barrier wait
// keeps for a session that is still up.
//
// VALIDATES: AC-5, its survivor. A barrier plugin never acknowledges and the
// session stays up (its Run never ends). The wait asks the peer clock for
// peerUpBarrierTimeout, which is 2 seconds, returns the timeout as its cause
// once that fires, and logs the warning that names the timeout.
// PREVENTS: the session-end case swallowing the timeout, so that a plugin that
// never acknowledges holds a live session's End-of-RIB for ever, or releases it
// without saying what the marker no longer proves.
func TestBarrierTimeoutStillWarnsOnLiveSession(t *testing.T) {
	routesLog := captureRoutesLog(t)
	require.Equal(t, 2*time.Second, peerUpBarrierTimeout, "the barrier timeout is 2 seconds")

	peer := NewPeer(NewPeerSettings(netip.MustParseAddr("192.0.2.3"), 65000, 65001, 0x01010101))
	clk := newSyncJoinClock()
	peer.SetClock(clk)
	peer.ResetPeerUpBarrier()
	peer.SetPeerUpBarrier(1)
	clk.fireBarrier()

	require.Equal(t, peerUpBarrierTimedOut, peer.waitPeerUpBarrier(NewSession(peer.settings)),
		"a live session's unacknowledged barrier must end on its timeout")
	clk.awaitBarrier(t)
	require.Contains(t, routesLog.String(), "peer-up barrier timeout",
		"the timeout on a live session must be logged")
	require.Contains(t, routesLog.String(), "timeout=2s")
}

// TestInitialSyncJoinReturnsDuringQueueDrain checks A-1 for the queue drain: a
// sync that drains its queue after its session ended is bounded.
//
// VALIDATES: the first session's sync is held before its static phase while
// three routes are queued for it, and the far end ends the session with a
// Cease. Once the session's Run has returned the test lets the sync go: it runs
// its static phase and its queue drain against a session whose connection is
// gone, every send fails, and the run loop enters its reconnect wait, so the
// join returned. The sync opened its own session's gate.
// PREVENTS: a join that blocks on a drain whose writes wait on a connection
// that no longer exists, which stops the peer reconnecting.
func TestInitialSyncJoinReturnsDuringQueueDrain(t *testing.T) {
	n, clk := startSyncJoinNeighbor(t)
	static := holdStaticPhase(t, n.peer)

	conn := n.accept(t)
	n.establish(t, conn, 90)
	awaitSessionsEstablished(t, n.peer, 1)
	session := n.peer.currentSession()
	for _, prefix := range []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"} {
		require.NoError(t, n.peer.QueueAnnounce(testRoute(prefix), bgptypes.NextHopExplicit, false),
			"the session's queue gate must be closed while its sync is held")
	}

	backoffCease(t, conn)
	awaitSessionEnd(t, session)
	static.release()
	clk.awaitWait(t)

	awaitInitialSyncDone(t, session)
	require.Equal(t, int32(0), session.sendingInitialRoutes.Load(),
		"the sync that drained an ended session left its queue gate closed")
}

// TestInitialSyncJoinWithQueuedTeardown checks A-2: a teardown the sync runs
// from its queue does not deadlock the join that waits for that sync.
//
// VALIDATES: a peer-level teardown is asked for while the first session's sync
// is held before its static phase, so it is queued (teardownOf). Let go, the
// sync reaches it in its queue drain: the far end reads the IPv4 End-of-RIB and
// then the Cease NOTIFICATION, the session ends, and the peer dials again at
// once (ErrTeardown), so runOnce joined the sync that tore its session down.
// PREVENTS: a deadlock between runQueuedTeardown, which ends the session from
// the sync goroutine, and the session-end defer, which waits for that goroutine.
func TestInitialSyncJoinWithQueuedTeardown(t *testing.T) {
	n, _ := startSyncJoinNeighbor(t)
	static := holdStaticPhase(t, n.peer)

	conn := n.accept(t)
	n.establish(t, conn, 90)
	awaitSessionsEstablished(t, n.peer, 1)
	session := n.peer.currentSession()
	require.NoError(t, n.peer.teardownAutomatic(message.NotifyCeaseOutOfResources, ""))
	static.release()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	var updates [][]byte
	for range 16 {
		frame, err := core4271ReadMessage(conn)
		require.NoError(t, err, "the far end never received the queued teardown's NOTIFICATION")
		if frame[18] == byte(msgtype.TypeKEEPALIVE) {
			continue
		}
		if frame[18] == byte(msgtype.TypeNOTIFICATION) {
			break
		}
		updates = append(updates, frame)
	}
	require.Equal(t, [][]byte{eorUpdate()}, updates,
		"the queued teardown must send the IPv4 End-of-RIB, and nothing else, before its NOTIFICATION")
	backoffHalfClose(t, conn)

	n.accept(t)
	awaitInitialSyncDone(t, session)
}
