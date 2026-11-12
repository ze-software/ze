// Design: docs/architecture/behavior/peer-lifecycle.md -- Reconnect backoff: when the delay and the prefix count restart
// Related: rfc4271_automatic_start_peer_test.go -- automaticStartNeighbor, the far end each test plays by hand
// Related: peer_run.go -- Peer.run, the loop these tests drive

package reactor

import (
	"net"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/clock"
)

// backoffClock is the peer clock for the backoff tests. Its Now moves only when
// the test moves it, so the time a session spends in Established is exactly the
// duration the test chose, to the nanosecond. Its After records the duration
// the run loop asked to wait and blocks until the test releases it, so a test
// reads the wait the loop chose instead of measuring one.
//
// Timers, tickers and sleeps stay on the real clock: the session's own timers
// (HoldTimer, KEEPALIVE, write polls) must keep working. Now is set to the real
// time again at each release, so deadlines a session computes from Now are
// never in the past when the next cycle starts. Safe for concurrent use.
type backoffClock struct {
	clock.RealClock

	mu  sync.Mutex
	now time.Time

	// waits receives the duration of every After call. Only Peer.run calls
	// After on a peer without a reactor: the peer-up barrier wait returns
	// before its After when no barrier plugin is expected.
	waits chan time.Duration
	fire  chan time.Time
}

func newBackoffClock() *backoffClock {
	return &backoffClock{
		now:   time.Now().Round(0),
		waits: make(chan time.Duration, 64),
		fire:  make(chan time.Time, 1),
	}
}

// Now returns the test-controlled time.
func (c *backoffClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// After records d and returns the channel release fires.
func (c *backoffClock) After(d time.Duration) <-chan time.Time {
	select {
	case c.waits <- d:
	default: // 64 waits is more than any test here makes; a dropped record fails awaitWait
	}
	return c.fire
}

// advance moves Now forward by d.
func (c *backoffClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// awaitWait returns the duration of the next wait the run loop entered.
func (c *backoffClock) awaitWait(t *testing.T) time.Duration {
	t.Helper()
	select {
	case d := <-c.waits:
		return d
	case <-time.After(10 * time.Second):
		t.Fatal("the peer never entered a reconnect wait")
		return 0
	}
}

// requireNoWait fails when the run loop entered a wait the test did not expect.
func (c *backoffClock) requireNoWait(t *testing.T, why string) {
	t.Helper()
	select {
	case d := <-c.waits:
		t.Fatalf("%s: the run loop waited %s", why, d)
	default: // no wait recorded
	}
}

// release ends the wait the run loop is in. Now catches up with the real time
// first, so the next session computes its deadlines from a time that is not
// in the past.
func (c *backoffClock) release() {
	c.mu.Lock()
	if wall := time.Now().Round(0); wall.After(c.now) {
		c.now = wall
	}
	now := c.now
	c.mu.Unlock()
	c.fire <- now
}

// backoffReconnectMin is the reconnect delay minimum of every backoff test. The
// assertions that a wait restarts at the minimum name it.
const backoffReconnectMin = 5 * time.Second

// startBackoffNeighbor runs a dial-only Peer on a backoffClock against a
// listener the test accepts on, with backoffReconnectMin as its reconnect delay
// minimum. configure, when not nil, edits the settings before the Peer is built.
func startBackoffNeighbor(t *testing.T, reconnectMax time.Duration, configure func(*PeerSettings)) (*automaticStartNeighbor, *backoffClock) {
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
	if configure != nil {
		configure(settings)
	}
	peer := NewPeer(settings)
	clk := newBackoffClock()
	peer.SetClock(clk)
	peer.setReconnectDelay(backoffReconnectMin, reconnectMax)
	t.Cleanup(startAndStop(t, peer))
	return &automaticStartNeighbor{peer: peer, listener: tcpListener}, clk
}

// backoffEstablish brings the session on conn to Established and returns once
// the Peer stamped its entry time. The clock moves one second first, so the
// stamp is this session's and not an earlier one's.
func backoffEstablish(t *testing.T, n *automaticStartNeighbor, clk *backoffClock, conn net.Conn) {
	t.Helper()
	clk.advance(time.Second)
	stamp := clk.Now()
	n.establish(t, conn, 90)
	require.Eventually(t, func() bool {
		return n.peer.EstablishedAt().UnixNano() == stamp.UnixNano()
	}, 5*time.Second, time.Millisecond, "the peer never stamped this session's entry to Established")
}

// requireBackoffBase asserts that wait is fsm.Jitter of base: between three
// quarters of base and base. Two bases one doubling apart give disjoint ranges.
func requireBackoffBase(t *testing.T, wait, base time.Duration, why string) {
	t.Helper()
	require.GreaterOrEqual(t, wait, base-base/4, why)
	require.LessOrEqual(t, wait, base, why)
}

// backoffPrefixTimer gives ipv4/unicast a maximum of one prefix, torn down on
// overflow, reconnecting on `reconnect timer` with a 30 second idle-timeout.
func backoffPrefixTimer(settings *PeerSettings) {
	settings.PrefixMaximum = map[string]uint32{"ipv4/unicast": 1}
	settings.PrefixWarning = map[string]uint32{"ipv4/unicast": 1}
	settings.PrefixTeardown = map[string]bool{"ipv4/unicast": true}
	settings.PrefixIdleTimeout = map[string]uint16{"ipv4/unicast": 30}
	settings.PrefixReconnect = map[string]PrefixReconnectMode{"ipv4/unicast": PrefixReconnectTimer}
}

// backoffPrefixIdle is the idle-timeout backoffPrefixTimer configures.
const backoffPrefixIdle = 30 * time.Second

// backoffPrefixOverflow sends an UPDATE announcing two IPv4 /24 prefixes, one
// over the maximum of backoffPrefixTimer, which tears the session down.
func backoffPrefixOverflow(t *testing.T, conn net.Conn) {
	t.Helper()
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9, // AS_PATH: AS_SEQUENCE 65001
		0x40, 0x03, 0x04, 192, 0, 2, 1, // NEXT_HOP 192.0.2.1
	}
	nlri := []byte{24, 10, 0, 0, 24, 10, 0, 1}
	_, err := conn.Write(automaticStartMessage(2, makeUpdateBody(nil, attrs, nlri)...))
	require.NoError(t, err)
	backoffHalfClose(t, conn)
}

// backoffHalfClose sends FIN after the message that ends a session. Ze's
// closeConn drains the socket until EOF or a deadline it computes from the
// peer clock, which runs ahead of the wall clock once a test advanced it, so
// without the FIN the drain outlasts awaitWait. A full Close could send RST
// over the unread messages Ze wrote and make Ze lose the one the test sent.
func backoffHalfClose(t *testing.T, conn net.Conn) {
	t.Helper()
	tcp, ok := conn.(*net.TCPConn)
	require.True(t, ok, "an accepted connection is TCP")
	require.NoError(t, tcp.CloseWrite())
}

// backoffCease ends an Established session with a received Cease NOTIFICATION.
func backoffCease(t *testing.T, conn net.Conn) {
	t.Helper()
	automaticStartCease(t, conn)
	backoffHalfClose(t, conn)
}

// backoffBadMarker fails a session in OpenSent with a header error.
func backoffBadMarker(t *testing.T, conn net.Conn) {
	t.Helper()
	automaticStartBadMarker(t, conn)
	backoffHalfClose(t, conn)
}

// TestRunBackoffRestartsAfterStableSession is the wiring test of the stable
// session rule, through Peer.run and the peer's clock.
//
// VALIDATES: AC-8. A failed handshake doubles the delay to 10 seconds. The next
// session reaches Established, stays there for reconnectMax (one minute) and
// ends on a received Cease: the wait that follows has reconnectMin (5 seconds)
// as its base.
// PREVENTS: the delay returning to its minimum only when a cycle returns no
// error, which only a stop does, so a peer that ran for a week and then lost
// its session reconnects on whatever delay its history left.
func TestRunBackoffRestartsAfterStableSession(t *testing.T) {
	n, clk := startBackoffNeighbor(t, time.Minute, nil)

	backoffBadMarker(t, n.accept(t))
	requireBackoffBase(t, clk.awaitWait(t), backoffReconnectMin, "the first wait has the minimum as its base")
	clk.release()

	conn := n.accept(t)
	backoffEstablish(t, n, clk, conn)
	clk.advance(time.Minute)
	backoffCease(t, conn)
	requireBackoffBase(t, clk.awaitWait(t), backoffReconnectMin,
		"a session that stayed in Established for reconnectMax restarts the delay at reconnectMin")
}

// TestRunBackoffDoublesAfterShortEstablishedSession is the damping half.
//
// VALIDATES: AC-9. After a failed handshake (base 5 seconds), a session that
// reaches Established and stays there half of reconnectMax before a Cease is
// followed by a wait of base 10 seconds, and the next failed handshake by a
// wait of base 20 seconds.
// PREVENTS: reaching Established resetting the delay by itself, which would let
// a peer that flaps every few seconds redial at the minimum for ever.
func TestRunBackoffDoublesAfterShortEstablishedSession(t *testing.T) {
	n, clk := startBackoffNeighbor(t, time.Minute, nil)

	backoffBadMarker(t, n.accept(t))
	requireBackoffBase(t, clk.awaitWait(t), backoffReconnectMin, "the first wait has the minimum as its base")
	clk.release()

	conn := n.accept(t)
	backoffEstablish(t, n, clk, conn)
	clk.advance(30 * time.Second)
	backoffCease(t, conn)
	requireBackoffBase(t, clk.awaitWait(t), 10*time.Second,
		"a session shorter than reconnectMax waits on the current delay")
	clk.release()

	backoffBadMarker(t, n.accept(t))
	requireBackoffBase(t, clk.awaitWait(t), 20*time.Second,
		"the delay doubled after the short session")
}

// TestRunBackoffKeepsDelayAfterLongCycleThatNeverEstablished proves that the
// stable time is read only for a cycle that reached Established.
//
// VALIDATES: AC-9b. A first session reaches Established and ends at once on a
// Cease (base 5 seconds), which leaves its entry time in EstablishedAt. The
// second cycle sits in OpenSent for twice reconnectMax and fails its
// handshake: the wait has base 10 seconds, the doubled delay.
// PREVENTS: a stable-time check that subtracts EstablishedAt without asking
// whether this cycle reached Established, which would read the earlier
// session's stamp and restart the delay after a cycle that never came up.
func TestRunBackoffKeepsDelayAfterLongCycleThatNeverEstablished(t *testing.T) {
	n, clk := startBackoffNeighbor(t, time.Minute, nil)

	first := n.accept(t)
	backoffEstablish(t, n, clk, first)
	backoffCease(t, first)
	requireBackoffBase(t, clk.awaitWait(t), backoffReconnectMin, "the first wait has the minimum as its base")
	clk.release()

	second := n.accept(t)
	clk.advance(2 * time.Minute)
	require.False(t, n.peer.EstablishedAt().IsZero(), "the first session's entry time is still held")
	backoffBadMarker(t, second)
	requireBackoffBase(t, clk.awaitWait(t), 10*time.Second,
		"a cycle that never reached Established keeps the delay, however long it lasted")
}

// TestRunPrefixTeardownCountKeptAfterLongCycleThatNeverEstablished is the
// prefix-count half of AC-9b.
//
// VALIDATES: AC-9b. A short session torn down by its prefix limit waits the
// idle-timeout (count 1). The next cycle sits in OpenSent for twice
// reconnectMax and fails its handshake. The session after it, torn down by its
// prefix limit again, waits twice the idle-timeout (count 2).
// PREVENTS: the prefix count restarting on a long cycle that never reached
// Established, which would let a peer that floods its prefix limit come back
// on the base idle-timeout after every slow failed attempt.
func TestRunPrefixTeardownCountKeptAfterLongCycleThatNeverEstablished(t *testing.T) {
	n, clk := startBackoffNeighbor(t, time.Minute, backoffPrefixTimer)

	first := n.accept(t)
	backoffEstablish(t, n, clk, first)
	backoffPrefixOverflow(t, first)
	require.Equal(t, backoffPrefixIdle, clk.awaitWait(t), "the first prefix teardown waits the idle-timeout")
	clk.release()

	second := n.accept(t)
	clk.advance(2 * time.Minute)
	backoffBadMarker(t, second)
	requireBackoffBase(t, clk.awaitWait(t), backoffReconnectMin, "a failed handshake takes the normal backoff")
	clk.release()

	third := n.accept(t)
	backoffEstablish(t, n, clk, third)
	backoffPrefixOverflow(t, third)
	require.Equal(t, 2*backoffPrefixIdle, clk.awaitWait(t),
		"a cycle that never reached Established keeps the prefix teardown count")
}

// TestRunBackoffKeepsCapOnPeerThatNeverComesUp proves that a peer that is
// simply down stays at the cap, where a rule measured error to error would
// restart it.
//
// VALIDATES: AC-9b with reconnectMin 5 seconds and reconnectMax 20 seconds. A
// peer whose every handshake fails, each cycle lasting reconnectMax on the
// clock, waits on bases 5, 10 and 20 seconds, then ten more times on base 20.
// PREVENTS: BIRD's literal measure (previous error to this error), under which
// two errors one maximum apart restart the delay, so a peer that never came up
// would cycle back to the minimum at random under the jitter. It also refuses a
// check that reads the zero EstablishedAt of a peer that never established as
// a session older than reconnectMax.
func TestRunBackoffKeepsCapOnPeerThatNeverComesUp(t *testing.T) {
	n, clk := startBackoffNeighbor(t, 20*time.Second, nil)

	bases := []time.Duration{backoffReconnectMin, 10 * time.Second}
	for range 11 {
		bases = append(bases, 20*time.Second)
	}
	for attempt, base := range bases {
		conn := n.accept(t)
		clk.advance(20 * time.Second)
		backoffBadMarker(t, conn)
		requireBackoffBase(t, clk.awaitWait(t), base, "attempt "+strconv.Itoa(attempt+1)+" of a peer that never came up")
		clk.release()
	}
}

// TestRunPrefixTeardownCountRestartsAfterStableSession is the prefix-count half
// of the stable session rule.
//
// VALIDATES: AC-10. Under `reconnect timer` with a 30 second idle-timeout, a
// short session torn down by its prefix limit waits 30 seconds (count 1). The
// next session stays in Established for reconnectMax before its prefix
// teardown: it waits 30 seconds again, count 1, not 60.
// PREVENTS: prefixTeardownCount restarting only when a cycle returns no error,
// so one burst of route leaks long ago keeps doubling every later wait up to
// the one hour cap.
func TestRunPrefixTeardownCountRestartsAfterStableSession(t *testing.T) {
	n, clk := startBackoffNeighbor(t, time.Minute, backoffPrefixTimer)

	first := n.accept(t)
	backoffEstablish(t, n, clk, first)
	backoffPrefixOverflow(t, first)
	require.Equal(t, backoffPrefixIdle, clk.awaitWait(t), "the first prefix teardown waits the idle-timeout")
	clk.release()

	second := n.accept(t)
	backoffEstablish(t, n, clk, second)
	clk.advance(time.Minute)
	backoffPrefixOverflow(t, second)
	require.Equal(t, backoffPrefixIdle, clk.awaitWait(t),
		"a prefix teardown of a session that stayed reconnectMax in Established waits the base idle-timeout")
}

// TestRunPrefixTeardownCountGrowsOnShortSessions is the damping half of AC-10.
//
// VALIDATES: three short sessions, each torn down by its prefix limit, wait 30,
// 60 and 120 seconds.
// PREVENTS: the stable-session check restarting the prefix count after a
// session shorter than reconnectMax, which would let a peer that leaks routes
// on every session reconnect on the base idle-timeout for ever.
func TestRunPrefixTeardownCountGrowsOnShortSessions(t *testing.T) {
	n, clk := startBackoffNeighbor(t, time.Minute, backoffPrefixTimer)

	for _, want := range []time.Duration{backoffPrefixIdle, 2 * backoffPrefixIdle, 4 * backoffPrefixIdle} {
		conn := n.accept(t)
		backoffEstablish(t, n, clk, conn)
		backoffPrefixOverflow(t, conn)
		require.Equal(t, want, clk.awaitWait(t), "each short prefix teardown doubles the wait")
		clk.release()
	}
}

// TestRunTeardownStillResetsDelay keeps the ErrTeardown branch as it is for a
// short session.
//
// VALIDATES: AC-8's permitted survivor. After a failed handshake (delay now 10
// seconds) and a prefix teardown (count 1), a short session torn down locally
// (ErrTeardown) redials with no wait; the next failed handshake waits on base
// 5 seconds; the next prefix teardown waits twice the idle-timeout.
// PREVENTS: the stable-session check moving the teardown reset, making a local
// teardown wait, or a short session's teardown clearing the prefix count.
func TestRunTeardownStillResetsDelay(t *testing.T) {
	n, clk := startBackoffNeighbor(t, time.Minute, backoffPrefixTimer)

	backoffBadMarker(t, n.accept(t))
	requireBackoffBase(t, clk.awaitWait(t), backoffReconnectMin, "the first wait has the minimum as its base")
	clk.release()

	leak := n.accept(t)
	backoffEstablish(t, n, clk, leak)
	backoffPrefixOverflow(t, leak)
	require.Equal(t, backoffPrefixIdle, clk.awaitWait(t), "the first prefix teardown waits the idle-timeout")
	clk.release()

	short := n.accept(t)
	backoffEstablish(t, n, clk, short)
	session := n.peer.currentSession()
	require.NotNil(t, session)
	automaticStartInitialEOR(t, n.peer, short)
	automaticStartTeardown(t, n.peer, session, short)

	failed := n.accept(t)
	clk.requireNoWait(t, "a local teardown redials at once")
	backoffBadMarker(t, failed)
	requireBackoffBase(t, clk.awaitWait(t), backoffReconnectMin, "a local teardown resets the delay")
	clk.release()

	again := n.accept(t)
	backoffEstablish(t, n, clk, again)
	backoffPrefixOverflow(t, again)
	require.Equal(t, 2*backoffPrefixIdle, clk.awaitWait(t),
		"a local teardown of a short session keeps the prefix teardown count")
}

// TestRunTeardownAfterStableSessionClearsPrefixCount runs the stable-session
// check before the ErrTeardown branch.
//
// VALIDATES: AC-8 for ErrTeardown. After a prefix teardown (count 1), a session
// that stays in Established for reconnectMax and is torn down locally redials
// with no wait, and the next prefix teardown waits the base idle-timeout.
// PREVENTS: the check running only for errors other than ErrTeardown, so an
// operator's teardown of a stable session leaves the prefix count of a leak
// long gone.
func TestRunTeardownAfterStableSessionClearsPrefixCount(t *testing.T) {
	n, clk := startBackoffNeighbor(t, time.Minute, backoffPrefixTimer)

	leak := n.accept(t)
	backoffEstablish(t, n, clk, leak)
	backoffPrefixOverflow(t, leak)
	require.Equal(t, backoffPrefixIdle, clk.awaitWait(t), "the first prefix teardown waits the idle-timeout")
	clk.release()

	stable := n.accept(t)
	backoffEstablish(t, n, clk, stable)
	session := n.peer.currentSession()
	require.NotNil(t, session)
	automaticStartInitialEOR(t, n.peer, stable)
	clk.advance(time.Minute)
	automaticStartTeardown(t, n.peer, session, stable)

	again := n.accept(t)
	clk.requireNoWait(t, "a local teardown redials at once")
	backoffEstablish(t, n, clk, again)
	backoffPrefixOverflow(t, again)
	require.Equal(t, backoffPrefixIdle, clk.awaitWait(t),
		"a local teardown of a stable session clears the prefix teardown count")
}

// TestRunBackoffStableTimeBoundary pins the boundary of the stable time.
//
// VALIDATES: the boundary row of the TDD plan. After a failed handshake (delay
// 10 seconds), a session that stays in Established reconnectMax minus one
// nanosecond waits on base 10 seconds, and one that stays exactly reconnectMax
// waits on base 5 seconds.
// PREVENTS: an off-by-one comparison (greater than instead of at least).
func TestRunBackoffStableTimeBoundary(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stable time.Duration
		base   time.Duration
	}{
		{name: "one nanosecond short keeps the delay", stable: time.Minute - time.Nanosecond, base: 10 * time.Second},
		{name: "exactly reconnectMax restarts it", stable: time.Minute, base: backoffReconnectMin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, clk := startBackoffNeighbor(t, time.Minute, nil)

			backoffBadMarker(t, n.accept(t))
			requireBackoffBase(t, clk.awaitWait(t), backoffReconnectMin, "the first wait has the minimum as its base")
			clk.release()

			conn := n.accept(t)
			backoffEstablish(t, n, clk, conn)
			clk.advance(tc.stable)
			backoffCease(t, conn)
			requireBackoffBase(t, clk.awaitWait(t), tc.base, "the wait after a session that stayed "+tc.stable.String())
		})
	}
}
