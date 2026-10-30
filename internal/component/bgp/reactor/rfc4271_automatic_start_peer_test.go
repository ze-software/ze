// Design: docs/architecture/behavior/fsm.md — the start event of each reconnect cycle
// Related: docs/architecture/behavior/peer-lifecycle.md — the peer run loop that restarts a session
// RFC: rfc/short/rfc4271.md — Section 8.2.2, Idle state

package reactor

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// automaticStartNeighbor is the far end of a dial-only Peer: the test accepts
// each connection the Peer makes and plays the BGP speaker by hand.
type automaticStartNeighbor struct {
	peer     *Peer
	listener *net.TCPListener
}

// startAutomaticStartNeighbor runs a dial-only Peer against a listener. The
// reconnect delay is long enough (half a second, jittered) for the test to read
// the ConnectRetryCounter between the end of one cycle and the start of the
// next, and short enough that the next cycle starts inside the test.
func startAutomaticStartNeighbor(t *testing.T) *automaticStartNeighbor {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // test cleanup
	tcpListener, isTCP := ln.(*net.TCPListener)
	require.True(t, isTCP, "Listen(\"tcp\") returns a TCPListener")
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address must be TCP")

	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Port = uint16(addr.Port) //nolint:gosec // a listener port fits 16 bits
	settings.Connection = ConnectionActive
	peer := NewPeer(settings)
	peer.setReconnectDelay(500*time.Millisecond, 500*time.Millisecond)
	t.Cleanup(startAndStop(t, peer))
	return &automaticStartNeighbor{peer: peer, listener: tcpListener}
}

// startAutomaticStartBothNeighbor is startAutomaticStartNeighbor for a Peer
// that accepts as well as dials (ConnectionBoth), with the reconnect delay the
// caller chooses, so a test can hand it an inbound connection mid-backoff.
func startAutomaticStartBothNeighbor(t *testing.T, delay time.Duration) *automaticStartNeighbor {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // test cleanup
	tcpListener, isTCP := ln.(*net.TCPListener)
	require.True(t, isTCP, "Listen(\"tcp\") returns a TCPListener")
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address must be TCP")

	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Port = uint16(addr.Port) //nolint:gosec // a listener port fits 16 bits
	settings.Connection = ConnectionBoth
	peer := NewPeer(settings)
	peer.setReconnectDelay(delay, delay)
	t.Cleanup(startAndStop(t, peer))
	return &automaticStartNeighbor{peer: peer, listener: tcpListener}
}

// accept takes the Peer's next connection and reads its OPEN, so the Peer's
// session is in OpenSent when it returns. The connection is closed at cleanup.
func (n *automaticStartNeighbor) accept(t *testing.T) net.Conn {
	t.Helper()
	require.NoError(t, n.listener.SetDeadline(time.Now().Add(10*time.Second)))
	conn, err := n.listener.Accept()
	require.NoError(t, err, "the peer never dialed")
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

	buf := make([]byte, 4096)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	got, err := conn.Read(buf)
	require.NoError(t, err)
	require.GreaterOrEqual(t, got, message.HeaderLen)
	require.Equal(t, byte(1), buf[18], "the peer's first message is its OPEN")
	return conn
}

// establish answers the Peer's OPEN with an OPEN carrying holdTime and a
// KEEPALIVE, and waits until the Peer's session reports Established.
func (n *automaticStartNeighbor) establish(t *testing.T, conn net.Conn, holdTime uint16) {
	t.Helper()
	open := &message.Open{
		Version: 4, MyAS: 65001, HoldTime: holdTime, BGPIdentifier: 0x02020202,
		OptionalParams: []byte{2, 6, 65, 4, 0, 0, 0xFD, 0xE9}, // ASN4 65001
	}
	wire := message.PackTo(open, nil)
	wire = append(wire, automaticStartMessage(4)...) // KEEPALIVE
	_, err := conn.Write(wire)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		session := n.peer.currentSession()
		return session != nil && session.State() == fsm.StateEstablished
	}, 5*time.Second, time.Millisecond, "the peer's session never reached Established")
}

// automaticStartMessage builds a BGP message of the given type whose body is
// body: the all-ones marker, the length, the type.
func automaticStartMessage(msgType byte, body ...byte) []byte {
	msg := make([]byte, message.HeaderLen, message.HeaderLen+len(body))
	for i := range message.MarkerLen {
		msg[i] = 0xFF
	}
	binary.BigEndian.PutUint16(msg[16:18], uint16(message.HeaderLen+len(body))) //nolint:gosec // a test body is a few octets
	msg[18] = msgType
	return append(msg, body...)
}

// TestRFC4271AutomaticStartEvent3ZeroesCounter is the wiring test for RFC 4271
// Event 3 (AutomaticStart): a session that reached Established ends, and the
// reconnect loop starts the next cycle with Event 3, whose Idle action list
// "sets ConnectRetryCounter to zero".
//
// VALIDATES: for an Established session ended by a received Cease NOTIFICATION
// (Event 25) and for one ended by HoldTimer expiry (Event 10), the Peer's
// ConnectRetryCounter first reads 1 (the Established state's increment), and
// reads 0 once the Peer has dialed its next connection.
// PREVENTS: the reconnect loop firing Event 6 after a session that ran, which
// keeps the counter, so a peer that held a session for a day and then lost it
// would restart carrying the failures from before that session. Both exits are
// error returns from Session.Run, so a predicate tied to runOnce returning nil
// (reached only when the peer stops) would leave the counter at 1.
//
// RFC requirement: RFC4271-8.2.2-20 positive -- a running Peer whose Established session ends by a received Cease NOTIFICATION, or by HoldTimer expiry with a negotiated Hold Time of 3 seconds, holds ConnectRetryCounter 1 after the session ends and 0 once it has dialed its next connection.
func TestRFC4271AutomaticStartEvent3ZeroesCounter(t *testing.T) {
	tests := []struct {
		name     string
		holdTime uint16
		end      func(t *testing.T, conn net.Conn)
	}{
		{
			name:     "Cease NOTIFICATION",
			holdTime: 90,
			end: func(t *testing.T, conn net.Conn) {
				t.Helper()
				_, err := conn.Write(automaticStartMessage(3, byte(message.NotifyCease), 2))
				require.NoError(t, err)
			},
		},
		{
			name:     "HoldTimer expiry",
			holdTime: 3,
			end:      func(*testing.T, net.Conn) {}, // send nothing: Ze's HoldTimer expires
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := startAutomaticStartNeighbor(t)
			first := n.accept(t)
			n.establish(t, first, tt.holdTime)
			require.Equal(t, uint32(0), n.peer.ConnectRetryCounter(), "reaching Established moves no counter")

			tt.end(t, first)
			require.Eventually(t, func() bool {
				return n.peer.ConnectRetryCounter() == 1
			}, 10*time.Second, time.Millisecond, "the end of an Established session increments the counter")

			n.accept(t)
			require.Equal(t, uint32(0), n.peer.ConnectRetryCounter(),
				"the cycle after an Established session starts with Event 3, which zeroes the counter")
		})
	}
}

// TestRFC4271FailedCycleRestartKeepsCounter is the negative half: a cycle that
// never reached Established is followed by Event 6, which keeps the counter.
//
// VALIDATES: a Peer whose first session fails in OpenSent on a header with an
// all-zero marker (Event 21, which increments the counter) still reads
// ConnectRetryCounter 1 once it has dialed its next connection.
// PREVENTS: Event 3 on every automatic restart, which would zero the counter
// before each attempt and leave it unable to count oscillation.
//
// RFC requirement: RFC4271-8.2.2-20 negative -- a running Peer whose session fails in OpenSent on an all-zero-marker header, never having reached Established, holds ConnectRetryCounter 1 after the failure and still 1 once it has dialed its next connection.
func TestRFC4271FailedCycleRestartKeepsCounter(t *testing.T) {
	n := startAutomaticStartNeighbor(t)
	first := n.accept(t)

	badHeader := automaticStartMessage(4)
	for i := range message.MarkerLen {
		badHeader[i] = 0
	}
	_, err := first.Write(badHeader)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return n.peer.ConnectRetryCounter() == 1
	}, 10*time.Second, time.Millisecond, "a header error in OpenSent increments the counter")

	n.accept(t)
	require.Equal(t, uint32(1), n.peer.ConnectRetryCounter(),
		"the cycle after a failed attempt starts with Event 6, which keeps the counter")
}

// automaticStartCease writes a Cease NOTIFICATION (Administrative Shutdown) to
// conn, which ends an Established session with Event 25 at the Peer.
func automaticStartCease(t *testing.T, conn net.Conn) {
	t.Helper()
	_, err := conn.Write(automaticStartMessage(3, byte(message.NotifyCease), 2))
	require.NoError(t, err)
}

// automaticStartBadMarker writes a KEEPALIVE whose marker is all zeros to conn,
// which fails a session in OpenSent with a header error (Event 21) at the Peer.
func automaticStartBadMarker(t *testing.T, conn net.Conn) {
	t.Helper()
	badHeader := automaticStartMessage(4)
	for i := range message.MarkerLen {
		badHeader[i] = 0
	}
	_, err := conn.Write(badHeader)
	require.NoError(t, err)
}

// TestRFC4271FailedCycleAfterEstablishedKeepsCounter covers the cycle AFTER the
// Event 3 cycle: the flag that chose Event 3 belongs to one cycle, so a failed
// attempt that follows a session that ran is a plain retry again.
//
// VALIDATES: a Peer whose first session reaches Established and ends on a Cease
// NOTIFICATION, whose second session fails in OpenSent on an all-zero marker,
// reads ConnectRetryCounter 0 once it has dialed the second connection, 1 after
// the second session fails, and still 1 once it has dialed the third.
// PREVENTS: the "this cycle reached Established" answer outliving the cycle it
// describes. Read without clearing, it stays true for ever after the first
// session, so every later restart fires Event 3 and zeroes the counter, and a
// peer that once held a session could never count a failure past one.
//
// RFC requirement: RFC4271-8.2.2-20 negative -- a running Peer whose first session reaches Established and ends on a received Cease NOTIFICATION, and whose second session fails in OpenSent on an all-zero-marker header, holds ConnectRetryCounter 1 after the second session fails and still 1 once it has dialed its third connection.
func TestRFC4271FailedCycleAfterEstablishedKeepsCounter(t *testing.T) {
	n := startAutomaticStartNeighbor(t)
	first := n.accept(t)
	n.establish(t, first, 90)
	automaticStartCease(t, first)

	second := n.accept(t)
	require.Equal(t, uint32(0), n.peer.ConnectRetryCounter(),
		"the cycle after an Established session starts with Event 3, which zeroes the counter")

	automaticStartBadMarker(t, second)
	require.Eventually(t, func() bool {
		return n.peer.ConnectRetryCounter() == 1
	}, 10*time.Second, time.Millisecond, "a header error in OpenSent increments the counter")

	n.accept(t)
	require.Equal(t, uint32(1), n.peer.ConnectRetryCounter(),
		"the cycle after a failed attempt starts with Event 6, even when an earlier cycle reached Established")
}

// TestRFC4271AutomaticStopTeardownRestartZeroesCounter covers the run loop's
// ErrTeardown exit, which restarts with no backoff at all.
//
// VALIDATES: a Peer whose Established session the local system tears down
// (teardownAutomatic, RFC 4271 Event 8, which increments the counter) reads
// ConnectRetryCounter 0 once it has dialed its next connection.
// PREVENTS: the immediate redial after a teardown taking a path that skips the
// start-event choice, which would fire Event 6 and carry the Event 8 increment
// into a cycle that follows a session that ran.
func TestRFC4271AutomaticStopTeardownRestartZeroesCounter(t *testing.T) {
	n := startAutomaticStartNeighbor(t)
	first := n.accept(t)
	n.establish(t, first, 90)

	require.Eventually(t, func() bool {
		return n.peer.teardownAutomatic(message.NotifyCeaseOutOfResources, "") == nil
	}, 5*time.Second, 10*time.Millisecond, "the teardown was never accepted")

	n.accept(t)
	require.Equal(t, uint32(0), n.peer.ConnectRetryCounter(),
		"the cycle after an Established session torn down by Event 8 starts with Event 3")
}

// TestRFC4271InboundShortenedBackoffZeroesCounter covers the run loop's
// inboundNotify exit, which ends the backoff early when a connection arrives.
//
// VALIDATES: a Peer that accepts and connects, with a reconnect delay of a
// minute, whose Established session ends on a Cease NOTIFICATION, starts its
// next cycle on an inbound connection handed to it during the backoff, sends
// its OPEN on that connection within seconds, and reads ConnectRetryCounter 0
// by then.
// PREVENTS: the early restart taking a path that skips the start-event choice.
func TestRFC4271InboundShortenedBackoffZeroesCounter(t *testing.T) {
	n := startAutomaticStartBothNeighbor(t, time.Minute)
	first := n.accept(t)
	n.establish(t, first, 90)
	automaticStartCease(t, first)
	require.Eventually(t, func() bool {
		return n.peer.ConnectRetryCounter() == 1
	}, 10*time.Second, time.Millisecond, "the end of an Established session increments the counter")

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // test cleanup
	var dialer net.Dialer
	remote, err := dialer.DialContext(context.Background(), "tcp", ln.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { remote.Close() }) //nolint:errcheck // test cleanup
	local, err := ln.Accept()
	require.NoError(t, err)
	n.peer.setInboundConnection(local)

	buf := make([]byte, 4096)
	require.NoError(t, remote.SetReadDeadline(time.Now().Add(10*time.Second)))
	got, err := remote.Read(buf)
	require.NoError(t, err, "the peer never sent its OPEN on the inbound connection: the backoff was not cut short")
	require.GreaterOrEqual(t, got, message.HeaderLen)
	require.Equal(t, byte(1), buf[18], "the peer's first message on the inbound connection is its OPEN")
	require.Equal(t, uint32(0), n.peer.ConnectRetryCounter(),
		"the cycle the inbound connection starts follows an Established session, so it is Event 3")
}

// startAutomaticStartControlledNeighbor holds only the reconnect wait. Session
// timers use the Peer's real clock, so a negotiated HoldTimer still expires.
func startAutomaticStartControlledNeighbor(t *testing.T, mode ConnectionMode) (*automaticStartNeighbor, *triggerClock) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Connection = mode
	n := &automaticStartNeighbor{}
	if mode.IsActive() {
		ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp4", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { ln.Close() }) //nolint:errcheck // Test cleanup.
		var ok bool
		n.listener, ok = ln.(*net.TCPListener)
		require.True(t, ok)
		addr, ok := ln.Addr().(*net.TCPAddr)
		require.True(t, ok)
		settings.Port = uint16(addr.Port) //nolint:gosec // A listener port fits 16 bits.
	}
	n.peer = NewPeer(settings)
	backoff := newTriggerClock()
	backoff.Clock = n.peer.clock
	n.peer.SetClock(backoff)
	n.peer.setReconnectDelay(time.Minute, time.Minute)
	t.Cleanup(startAndStop(t, n.peer))
	return n, backoff
}

// automaticStartAwaitBackoff observes the actual run loop entering its wait,
// rather than racing the session's counter increment against its cleanup.
func automaticStartAwaitBackoff(t *testing.T, n *automaticStartNeighbor, backoff *triggerClock) {
	t.Helper()
	select {
	case <-backoff.waiting:
	case <-time.After(10 * time.Second):
		t.Fatal("the peer never entered reconnect backoff")
	}
	require.Nil(t, n.peer.currentSession(), "backoff begins after the old session is retired")
	require.Equal(t, uint32(1), n.peer.ConnectRetryCounter(), "the failed cycle increments the real counter")
}

// automaticStartInbound supplies a real socket after backoff has begun. No
// timer is released: receiving the OPEN proves inboundNotify woke the run loop.
func automaticStartInbound(t *testing.T, n *automaticStartNeighbor) {
	t.Helper()
	remote, local := collisionProofTCPPair(t)
	n.peer.setInboundConnection(local)
	frame, err := core4271ReadMessage(remote)
	require.NoError(t, err, "inboundNotify did not wake the peer and produce an OPEN")
	require.Equal(t, byte(1), frame[18], "the inbound socket's first message is OPEN")
	require.Eventually(t, func() bool {
		session := n.peer.currentSession()
		if session == nil {
			return false
		}
		return session.State() == fsm.StateOpenSent
	}, 5*time.Second, time.Millisecond, "the inbound cycle never reached OpenSent")
}

// automaticStartInitialEOR finishes the real initial update before a test
// observes silence or requests teardown. No routes are configured, so the one
// negotiated IPv4 family owes exactly the empty UPDATE from RFC 4724 Section 2.
func automaticStartInitialEOR(t *testing.T, peer *Peer, conn net.Conn) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	for range 16 {
		frame, err := core4271ReadMessage(conn)
		require.NoError(t, err, "the initial IPv4 End-of-RIB never arrived")
		if frame[18] == 2 {
			// RFC 4724 Section 2: IPv4 End-of-RIB is a minimum-length UPDATE.
			require.Equal(t, eorUpdate(), frame, "the initial UPDATE must be exactly IPv4 End-of-RIB")
			require.Eventually(t, func() bool {
				return peer.sendingInitialRoutes.Load() == 0
			}, 5*time.Second, time.Millisecond, "initial synchronization never released the teardown queue")
			return
		}
		require.Equal(t, byte(4), frame[18], "only KEEPALIVEs precede initial End-of-RIB")
	}
	t.Fatal("the neighbor received 16 messages without initial End-of-RIB")
}

// automaticStartTeardown completes the remote side of the TCP close after
// receiving Cease. Leaving it open would instead exercise closeConn's bounded
// read-drain timeout, not the ErrTeardown immediate-reconnect path under test.
func automaticStartTeardown(t *testing.T, peer *Peer, session *Session, remote net.Conn) {
	t.Helper()
	require.NoError(t, remote.SetReadDeadline(time.Now().Add(5*time.Second)))
	done := make(chan struct{})
	var frame []byte
	var readErr error
	t.Cleanup(func() {
		remote.Close() //nolint:errcheck // Unblocks the reader even when an assertion terminates the test.
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the teardown neighbor reader did not stop")
		}
	})
	go func() { // goroutine-lifecycle: one bounded neighbor close exchange, joined below and at cleanup.
		defer close(done)
		defer remote.Close() //nolint:errcheck // Answer the local TCP close after receiving its notification.
		for range 16 {
			frame, readErr = core4271ReadMessage(remote)
			if readErr != nil {
				return
			}
			if frame[18] != 4 {
				return
			}
		}
	}()
	// RFC 4271 Section 8.2.2, Established + AutomaticStop.
	require.NoError(t, peer.teardownAutomatic(message.NotifyCeaseOutOfResources, ""))
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the teardown neighbor never received Cease")
	}
	require.NoError(t, readErr)
	require.Len(t, frame, message.HeaderLen+2)
	require.Equal(t, byte(3), frame[18], "automatic teardown sends NOTIFICATION")
	require.Equal(t, byte(message.NotifyCease), frame[19])
	require.Equal(t, message.NotifyCeaseOutOfResources, frame[20])
	reason := session.closeReason.Load()
	require.NotNil(t, reason, "the teardown must record its cause")
	require.ErrorIs(t, *reason, ErrTeardown, "the next cycle must follow ErrTeardown, not an unrelated close")
}

// automaticStartHoldExpired reads the real timer's NOTIFICATION while leaving
// TCP open. A socket close or a manually injected FSM event cannot satisfy it.
func automaticStartHoldExpired(t *testing.T, conn net.Conn) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	for range 16 { // The silent neighbor can receive only KEEPALIVEs before the notification.
		frame, err := core4271ReadMessage(conn)
		require.NoError(t, err, "the real HoldTimer never sent its NOTIFICATION")
		if frame[18] == 3 {
			require.Len(t, frame, message.HeaderLen+2)
			require.Equal(t, byte(message.NotifyHoldTimerExpired), frame[19])
			require.Equal(t, byte(0), frame[20])
			return
		}
		require.Equal(t, byte(4), frame[18], "only KEEPALIVEs precede hold expiry")
	}
	t.Fatal("the silent neighbor received 16 messages without HoldTimer expiry")
}

// TestRFC4271FailedCycleInboundShortenedBackoffKeepsCounter proves that an
// inbound wakeup does not turn a failed OpenSent attempt into an undamped start.
//
// MUTATION: in Peer.runOnce, initialize start to session.startAutomatic instead
// of session.startDamped; the inbound cycle then incorrectly reads zero.
// RFC requirement: RFC4271-8.2.2-20 negative -- after a real Peer fails in OpenSent without reaching Established, an inbound connection wakes its held backoff and reaches OpenSent with ConnectRetryCounter still 1, not zero.
func TestRFC4271FailedCycleInboundShortenedBackoffKeepsCounter(t *testing.T) {
	n, backoff := startAutomaticStartControlledNeighbor(t, ConnectionBoth)
	first := n.accept(t)
	require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())
	automaticStartBadMarker(t, first) // RFC 4271 Section 8.2.2, OpenSent.
	automaticStartAwaitBackoff(t, n, backoff)

	automaticStartInbound(t, n)
	require.Equal(t, uint32(1), n.peer.ConnectRetryCounter(),
		"inboundNotify must keep Event 6 after a never-Established attempt")
}

// TestRFC4271HoldExpiryInboundShortenedBackoffZeroesCounter covers the other
// Established error exit combined with an inbound wakeup, not timer release.
//
// MUTATION: in Peer.runOnce, choose session.startDamped in the
// reachedEstablished branch; the inbound cycle then incorrectly keeps 1.
// RFC requirement: RFC4271-8.2.2-20 positive -- after real negotiated HoldTimer expiry ends an Established Peer session with counter 1, an inbound connection wakes the held backoff and reaches OpenSent with counter 0.
func TestRFC4271HoldExpiryInboundShortenedBackoffZeroesCounter(t *testing.T) {
	n, backoff := startAutomaticStartControlledNeighbor(t, ConnectionBoth)
	first := n.accept(t)
	n.establish(t, first, 3)
	require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())
	automaticStartInitialEOR(t, n.peer, first)
	automaticStartHoldExpired(t, first) // RFC 4271 Section 8.2.2, Established.
	automaticStartAwaitBackoff(t, n, backoff)

	automaticStartInbound(t, n)
	require.Equal(t, uint32(0), n.peer.ConnectRetryCounter(),
		"inboundNotify must keep Event 3 after an Established session expires")
}

// TestRFC4271PassiveAutomaticStartReconnectZeroesCounter observes a passive
// Peer's next Active session after each Established exit. Ze conflates Events
// 3/5 as documented in FSM.handleIdle; this does not claim a distinct Event 5.
//
// MUTATION: in Peer.runOnce, choose session.startDamped in the
// reachedEstablished branch; each new Active session then incorrectly keeps 1.
// RFC requirement: RFC4271-8.2.2-20 positive -- a passive-only real Peer leaving Established after received Cease, real HoldTimer expiry, or automatic teardown reports counter 1 on exit and reaches a new Active session with counter 0 through Ze's conflated Event 3/5 start.
func TestRFC4271PassiveAutomaticStartReconnectZeroesCounter(t *testing.T) {
	for _, tc := range []struct {
		name     string
		holdTime uint16
		teardown bool
	}{
		{name: "notification", holdTime: 90},
		{name: "hold-expiry", holdTime: 3},
		{name: "teardown", holdTime: 90, teardown: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, backoff := startAutomaticStartControlledNeighbor(t, ConnectionPassive)
			require.Eventually(t, func() bool {
				return n.peer.receivePublication.Load() != nil
			}, 5*time.Second, time.Millisecond, "the passive session never reached its receive loop")
			firstSession := n.peer.currentSession()
			require.NotNil(t, firstSession)
			require.Equal(t, fsm.StateActive, firstSession.State())
			remote, local := collisionProofTCPPair(t)
			// Only the read assertions need a deadline. The live socket must not
			// expire before the real HoldTimer on a loaded machine.
			require.NoError(t, local.SetDeadline(time.Time{}))
			require.NoError(t, n.peer.acceptConnection(local))
			open, err := core4271ReadMessage(remote)
			require.NoError(t, err)
			require.Equal(t, byte(1), open[18])

			// Capture the real counter synchronously at leave-Established:
			// ErrTeardown can start the next cycle before a poll sees the 1.
			left := make(chan uint32, 1)
			n.peer.SetCallback(func(from, _ PeerState) {
				if from == PeerStateEstablished {
					left <- n.peer.ConnectRetryCounter()
				}
			})
			n.establish(t, remote, tc.holdTime)
			require.Equal(t, uint32(0), n.peer.ConnectRetryCounter())
			automaticStartInitialEOR(t, n.peer, remote)
			switch {
			case tc.teardown:
				automaticStartTeardown(t, n.peer, firstSession, remote)
			case tc.holdTime == 3:
				automaticStartHoldExpired(t, remote) // RFC 4271 Section 8.2.2.
			default:
				automaticStartCease(t, remote) // RFC 4271 Section 8.2.2.
			}
			select {
			case counter := <-left:
				require.Equal(t, uint32(1), counter, "leaving Established increments before restart")
			case <-time.After(10 * time.Second):
				t.Fatal("the passive peer never left Established")
			}
			if !tc.teardown {
				automaticStartAwaitBackoff(t, n, backoff)
				backoff.expire()
			}
			require.Eventually(t, func() bool {
				session := n.peer.currentSession()
				if session == nil {
					return false
				}
				if session == firstSession {
					return false
				}
				return session.State() == fsm.StateActive
			}, 5*time.Second, time.Millisecond, "the next passive cycle never reached Active")
			require.Equal(t, uint32(0), n.peer.ConnectRetryCounter(),
				"the new passive session must use the zeroing Event 3/5 arm, not Event 6")
		})
	}
}
