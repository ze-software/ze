package reactor

import (
	"context"
	"encoding/binary"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// serveHandshakeFailures accepts every connection and, when answerBadHeader is
// true, answers ze's OPEN with a 19-byte message whose marker is all zeros.
// RFC 4271 Section 4.1 requires the marker to be all ones, so
// message.ParseHeader rejects it and the read loop fires Event 21
// (BGPHeaderErr) while the session sits in OpenSent (session_read.go,
// readAndProcessMessage).
//
// When answerBadHeader is false it accepts and closes, which reaches ze as EOF
// and fires Event 18 (TcpConnectionFails) from the same state (session_read.go,
// handleConnectionClose). RFC 4271 Section 8.2.2 gives that event no
// ConnectRetryCounter clause in OpenSent, so the two modes are the positive
// and the negative of one rail.
func serveHandshakeFailures(t *testing.T, answerBadHeader bool) (port uint16, handshakes *atomic.Int32, stop func()) {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	served := &atomic.Int32{}
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return // listener closed by stop()
			}
			go serveOneFailure(conn, answerBadHeader, served)
		}
	}()

	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok, "listener address must be TCP")
	return uint16(addr.Port), served, func() {
		if closeErr := ln.Close(); closeErr != nil {
			t.Logf("closing test listener: %v", closeErr)
		}
	}
}

// serveOneFailure drives one connection to the failure the caller asked for.
// Both modes first read ze's OPEN, which is what puts ze's FSM in OpenSent and
// makes the event that follows the one this test is about.
func serveOneFailure(conn net.Conn, answerBadHeader bool, served *atomic.Int32) {
	defer func() { _ = conn.Close() }() //nolint:errcheck // test peer teardown

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return
	}
	drain := make([]byte, 4096)
	if _, err := conn.Read(drain); err != nil {
		return
	}
	// ze has sent its OPEN, so its FSM is in OpenSent.
	served.Add(1)

	if !answerBadHeader {
		return // close -> EOF -> Event 18 (TcpConnectionFails) in OpenSent
	}

	var bad [message.HeaderLen]byte // marker deliberately left all zeros
	binary.BigEndian.PutUint16(bad[16:18], message.HeaderLen)
	bad[18] = 4 // KEEPALIVE: a valid type, so the marker is the only fault
	if _, err := conn.Write(bad[:]); err != nil {
		return
	}

	// Hold the socket open long enough for ze to read that header, so the
	// event it fires is the header error and not the EOF behind it.
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		return
	}
	if _, err := conn.Read(drain[:1]); err != nil {
		return
	}
}

// startPeerAgainst builds a dial-only peer pointed at 127.0.0.1:port with a
// short reconnect delay, starts it, and returns a stop function.
func startPeerAgainst(t *testing.T, port uint16) (*Peer, func()) {
	t.Helper()
	settings := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Port = port
	settings.Connection = ConnectionActive // dial only; this test runs no listener for ze

	peer := NewPeer(settings)
	peer.SetReconnectDelay(10*time.Millisecond, 20*time.Millisecond)
	return peer, startAndStop(t, peer)
}

func startAndStop(t *testing.T, peer *Peer) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	peer.StartWithContext(ctx)
	return func() {
		cancel()
		waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer waitCancel()
		if err := peer.Wait(waitCtx); err != nil {
			t.Logf("peer stop: %v", err)
		}
	}
}

// TestConnectRetryCounterAccumulatesAcrossSessions is the wiring test for RFC
// 4271 Section 8.1.1's ConnectRetryCounter: it proves the value survives the
// thing that would obviously destroy it.
//
// VALIDATES: A peer that fails its handshake over and over reaches a
// ConnectRetryCounter of three or more, even though Peer.runOnce built a new
// Session and a new FSM for each of those attempts.
//
// PREVENTS: The counter being scoped to the Session, where every retry would
// reset it to zero, and the reconnect cycle firing RFC 4271 Event 1, whose
// "sets ConnectRetryCounter to zero" clause would zero it just as effectively.
// Both failures leave the counter oscillating between 0 and 1 forever while
// every unit test in the fsm package still passes.
func TestConnectRetryCounterAccumulatesAcrossSessions(t *testing.T) {
	port, _, stopListener := serveHandshakeFailures(t, true)
	defer stopListener()

	peer, stopPeer := startPeerAgainst(t, port)
	defer stopPeer()

	require.Eventually(t, func() bool {
		return peer.ConnectRetryCounter() >= 3
	}, 20*time.Second, 10*time.Millisecond,
		"the ConnectRetryCounter must accumulate across reconnect cycles")

	// The same number reaches the operator through Stats, which is what
	// `show bgp peer <ip> detail` and the Prometheus gauge both read.
	require.Equal(t, peer.ConnectRetryCounter(), peer.Stats().ConnectRetryCounter,
		"Stats must report the live counter")
}

// TestConnectRetryCounterQuietWhenTheRFCGivesNoClause is the negative half of
// the wiring test.
//
// VALIDATES: A peer whose every connection is accepted and immediately closed
// keeps a ConnectRetryCounter of zero, because RFC 4271 Section 8.2.2 gives
// OpenSent's Event 18 (TcpConnectionFails) no ConnectRetryCounter line.
//
// PREVENTS: An increment placed on the reconnect loop itself rather than on
// the RFC's clauses, which would make the counter say "attempts" where the RFC
// says "attempts that ended in a state the standard counts". That version
// passes the positive test above and is wrong.
func TestConnectRetryCounterQuietWhenTheRFCGivesNoClause(t *testing.T) {
	port, handshakes, stopListener := serveHandshakeFailures(t, false)
	defer stopListener()

	peer, stopPeer := startPeerAgainst(t, port)
	defer stopPeer()

	// Wait for several full OpenSent-then-EOF cycles before judging. The
	// listener counts a handshake only once ze's OPEN has arrived, so each
	// tick is a cycle that really reached OpenSent.
	require.Eventually(t, func() bool {
		return handshakes.Load() >= 4
	}, 20*time.Second, 10*time.Millisecond, "peer should be cycling through failed handshakes")

	require.Equal(t, uint32(0), peer.ConnectRetryCounter(),
		"OpenSent + TcpConnectionFails has no ConnectRetryCounter clause in RFC 4271 §8.2.2")
}

// TestConnectRetryCounterZeroedByOperatorRestart verifies the RFC's reset
// clause reaches production through the peer lifecycle, not only through a
// hand-driven FSM.
//
// VALIDATES: A peer carrying a ConnectRetryCounter of two or more drops back
// below two once StartWithContext hands the next cycle RFC 4271 Event 1
// (ManualStart).
//
// PREVENTS: operatorStarted latching true forever, which would mean a peer the
// operator stopped and restarted keeps reporting the retry history of the run
// before it -- the RFC's Event 1 clause exists to end that history.
func TestConnectRetryCounterZeroedByOperatorRestart(t *testing.T) {
	port, _, stopListener := serveHandshakeFailures(t, true)
	defer stopListener()

	peer, stopPeer := startPeerAgainst(t, port)
	require.Eventually(t, func() bool {
		return peer.ConnectRetryCounter() >= 2
	}, 20*time.Second, 10*time.Millisecond, "counter must climb before the restart")
	stopPeer()

	stopAgain := startAndStop(t, peer)
	defer stopAgain()

	// The restarted peer's first cycle fires ManualStart, whose §8.2.2 clause
	// zeroes the counter. It climbs again straight afterwards, so the
	// assertion is that it DROPPED, not that it settles at zero.
	require.Eventually(t, func() bool {
		return peer.ConnectRetryCounter() < 2
	}, 20*time.Second, 2*time.Millisecond,
		"an operator restart must set the ConnectRetryCounter to zero")
}

// TestPeerSharesOneConnectRetryCounterWithEveryFSM pins the ownership decision
// the whole feature rests on, without needing a live socket.
//
// VALIDATES: Two successive Sessions wired the way Peer.runOnce wires them
// raise ONE counter, and the peer reads the sum. The second session starts
// with the damped event, so the first session's count survives it.
//
// PREVENTS: A future refactor giving each Session its own counter, or firing
// RFC 4271 Event 1 on every cycle. Either reintroduces the reset-per-cycle bug,
// and the socket-driven tests above would only catch it by timing out after
// fifteen seconds.
func TestPeerSharesOneConnectRetryCounterWithEveryFSM(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.9"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	for cycle := 1; cycle <= 2; cycle++ {
		session := NewSession(settings)
		session.SetConnectRetryCounter(&peer.connectRetryCounter)

		// Cycle 1 is the operator's start (Event 1); cycle 2 is a damped
		// retry (Event 6). This mirrors runOnce's choice.
		if cycle == 1 {
			require.NoError(t, session.Start())
		} else {
			require.NoError(t, session.StartDamped())
		}

		// Walk to Established, then hand the FSM the one event RFC 4271
		// §8.2.2 says must raise the counter in that state.
		for _, ev := range []fsm.Event{
			fsm.EventTCPConnectionConfirmed,
			fsm.EventBGPOpen,
			fsm.EventKeepaliveMsg,
		} {
			require.NoError(t, session.fsm.Event(ev), "cycle %d: driving with %s", cycle, ev)
		}
		require.Equal(t, fsm.StateEstablished, session.fsm.State(), "cycle %d", cycle)
		require.NoError(t, session.fsm.Event(fsm.EventNotifMsg))

		require.Equal(t, uint32(cycle), peer.ConnectRetryCounter(),
			"cycle %d: the peer must see every session's increments on one counter", cycle)
	}
}

// crcSession builds a session parked in Established, sharing a counter that
// already stands at `start`, so a teardown's effect on the counter is visible
// as a direction rather than as an absolute.
func crcSession(t *testing.T, start uint32) (*Session, *fsm.ConnectRetryCounter) {
	t.Helper()
	settings := NewPeerSettings(mustParseAddr("192.0.2.11"), 65000, 65001, 0x01010101)
	c := &fsm.ConnectRetryCounter{}
	for range start {
		c.Increment()
	}

	session := NewSession(settings)
	session.SetConnectRetryCounter(c)
	for _, ev := range []fsm.Event{
		fsm.EventAutomaticStartWithDampPeerOscillations,
		fsm.EventTCPConnectionConfirmed,
		fsm.EventBGPOpen,
		fsm.EventKeepaliveMsg,
	} {
		require.NoError(t, session.fsm.Event(ev), "driving the session to Established with %s", ev)
	}
	require.Equal(t, fsm.StateEstablished, session.fsm.State())
	require.Equal(t, start, c.Load(), "reaching Established must not move the counter")
	return session, c
}

// TestTeardownKindDecidesConnectRetryCounter verifies each of ze's three
// session-teardown entry points raises the RFC 4271 event its origin calls for.
//
// VALIDATES: Session.Teardown zeroes the ConnectRetryCounter (Event 2,
// ManualStop), Session.TeardownAutomatic increments it (Event 8,
// AutomaticStop), and Session.CloseWithNotification increments it (Event 23,
// OpenCollisionDump).
//
// PREVENTS: Every teardown reaching the FSM as ManualStop, which is what ze
// did before the counter existed. It was invisible then and is not now: a peer
// dropped by BFD, by an out-of-resources forward pool, or by collision
// resolution would report a ConnectRetryCounter of zero, which reads as "this
// peer has never had trouble" at the exact moment it has.
func TestTeardownKindDecidesConnectRetryCounter(t *testing.T) {
	tests := []struct {
		name string
		tear func(*Session) error
		want uint32
	}{
		{
			name: "operator teardown zeroes (RFC 4271 Event 2)",
			tear: func(s *Session) error { return s.Teardown(message.NotifyCeaseAdminShutdown, "") },
			want: 0,
		},
		{
			name: "automatic teardown increments (RFC 4271 Event 8)",
			tear: func(s *Session) error { return s.TeardownAutomatic(message.NotifyCeaseBFDDown, "") },
			want: 4,
		},
		{
			name: "collision close increments (RFC 4271 Event 23)",
			tear: func(s *Session) error {
				return s.CloseWithNotification(message.NotifyCease, message.NotifyCeaseConnectionCollision)
			},
			want: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, c := crcSession(t, 3)
			require.NoError(t, tt.tear(session))
			require.Equal(t, fsm.StateIdle, session.fsm.State(), "every teardown reaches Idle")
			require.Equal(t, tt.want, c.Load())
		})
	}
}

// TestQueuedAutomaticTeardownStaysAutomatic verifies the manual/automatic
// distinction survives the operation queue.
//
// VALIDATES: Peer.TeardownAutomatic queued while no session exists lands in
// opQueue with Automatic set, and Peer.Teardown lands with it clear.
//
// PREVENTS: The flag being dropped at the queue boundary, which would turn
// every teardown that had to wait for End-of-RIB back into an operator stop
// and zero the counter on the drain.
func TestQueuedAutomaticTeardownStaysAutomatic(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.12"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	require.NoError(t, peer.TeardownAutomatic(message.NotifyCeaseOutOfResources, ""))
	require.NoError(t, peer.Teardown(message.NotifyCeaseAdminShutdown, ""))

	peer.mu.Lock()
	queued := append([]PeerOp(nil), peer.opQueue...)
	peer.mu.Unlock()

	require.Len(t, queued, 2, "both teardowns queue when there is no session")
	require.True(t, queued[0].Automatic, "TeardownAutomatic must queue as automatic")
	require.False(t, queued[1].Automatic, "Teardown must queue as an operator stop")
}
