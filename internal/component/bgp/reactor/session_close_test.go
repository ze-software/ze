// Design: docs/architecture/core-design.md -- the one bounded close routine of a session connection
// Related: session_connection.go -- closeConnWithin, closeFlushed, dropConn, clearClosedConn
// Related: session_wedge_test.go -- wedgeEstablished and the three closes behind a stuck writer
// Related: session_write.go -- sendNotificationWithin, sendHoldTimerExpired

package reactor

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/test/sim"
)

// closePeer is one Established session whose peer end keeps every octet ze
// writes, so a test can read the last message that came before the close.
type closePeer struct {
	session *Session
	// timerClock drives the FSM timers only; the session's own clock is real.
	timerClock *sim.FakeClock
	client     net.Conn
	// wire carries what ze wrote after its OPEN, and closes with the pipe.
	wire <-chan []byte
	// drainErr carries the error that ended the peer end's reads.
	drainErr <-chan error
}

// closeEstablished drives a real Session to Established over a net.Pipe with
// Run executing, as wedgeEstablished does, and captures the peer end's reads
// (startDrain) where wedgeEstablished discards them.
func closeEstablished(t *testing.T, sendHold time.Duration) *closePeer {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = wedgeHold
	settings.SendHoldTime = sendHold
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	timerClock := sim.NewFakeClock(time.Now())
	session.timers.SetClock(timerClock)
	require.NoError(t, session.Start())

	server, client := net.Pipe()
	_ = acceptWithReader(t, session, server, client)
	wire, drainErr := startDrain(t, client)

	_ = rfc9687Run(t, session)

	// net.Pipe is synchronous, so each write needs its own goroutine and the
	// two are sequenced on the state each one produces.
	go func() { _, _ = client.Write(rfc9687PeerOpen(uint16(wedgeHold / time.Second))) }()
	require.Eventually(t, func() bool { return session.State() == fsm.StateOpenConfirm },
		runExitDeadline, 5*time.Millisecond,
		"precondition: ze did not reach OpenConfirm, so the peer OPEN was not accepted")

	go func() { _, _ = client.Write(message.PackTo(message.NewKeepalive(), nil)) }()
	require.Eventually(t, func() bool { return session.State() == fsm.StateEstablished },
		runExitDeadline, 5*time.Millisecond,
		"precondition: the session never reached Established, so there is nothing to close")

	return &closePeer{session: session, timerClock: timerClock, client: client, wire: wire, drainErr: drainErr}
}

// closeFrames splits what ze wrote into BGP messages. Each pass takes one
// whole message of at least a header, so the loop ends with the octets.
//
// RFC 4271 Section 4.1: the header is a 16-octet Marker, a 2-octet Length that
// counts the header, and a 1-octet Type.
//
//	Byte offset: | 0..15 Marker | 16..17 Length | 18 Type |
func closeFrames(t *testing.T, sent []byte) [][]byte {
	t.Helper()
	var frames [][]byte
	for len(sent) > 0 {
		require.GreaterOrEqual(t, len(sent), message.HeaderLen, "ze wrote a truncated message header")
		length := int(sent[16])<<8 | int(sent[17])
		require.GreaterOrEqual(t, length, message.HeaderLen, "ze wrote a Length below the header")
		require.LessOrEqual(t, length, len(sent), "ze wrote a truncated message")
		frames = append(frames, sent[:length])
		sent = sent[length:]
	}
	return frames
}

// closeAttachPeer answers a Peer that holds session the way runOnce publishes
// it, so the peer-level stops reach the session through their own entry point.
func closeAttachPeer(session *Session) *Peer {
	peer := NewPeer(session.settings)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	return peer
}

// TestCloseDeliversNotificationWhenPathFree proves the bounded close costs a
// healthy session nothing: with the write path free, every close that owes a
// NOTIFICATION puts it on the wire, and the peer reads it before it reads the
// close. The peer end is a pipe read by one goroutine, which publishes what it
// read and then the error that ended it, so the NOTIFICATION it holds was read
// before the close. The read task's panic guard is the one shape not repeated
// here: TestSessionReadPanicSendsCeaseAndCloses asserts it.
//
// VALIDATES: AC-5 -- the last message the peer read before the close is the
// NOTIFICATION that close owes, with its code and subcode.
// PREVENTS: a close routine that, bounded for the stuck writer, skips the
// NOTIFICATION or the final FIN on a session whose peer is reading.
func TestCloseDeliversNotificationWhenPathFree(t *testing.T) {
	cases := []struct {
		name     string
		sendHold time.Duration
		// trigger starts the close on the Established session.
		trigger func(t *testing.T, p *closePeer)
		code    message.NotifyErrorCode
		subcode uint8
	}{
		{
			name: "operator teardown",
			trigger: func(t *testing.T, p *closePeer) {
				require.NoError(t, p.session.Teardown(message.NotifyCeaseAdminShutdown, ""))
			},
			code: message.NotifyCease, subcode: message.NotifyCeaseAdminShutdown,
		},
		{
			name: "automatic teardown",
			trigger: func(t *testing.T, p *closePeer) {
				require.NoError(t, p.session.teardownAutomatic(message.NotifyCeaseOutOfResources, ""))
			},
			code: message.NotifyCease, subcode: message.NotifyCeaseOutOfResources,
		},
		{
			name: "peer removal by reload",
			trigger: func(_ *testing.T, p *closePeer) {
				closeAttachPeer(p.session).stopWithCease(message.NotifyCeasePeerDeconfigured)
			},
			code: message.NotifyCease, subcode: message.NotifyCeasePeerDeconfigured,
		},
		{
			name: "daemon stop",
			trigger: func(_ *testing.T, p *closePeer) {
				closeAttachPeer(p.session).shutdownNotify()
			},
			code: message.NotifyCease, subcode: message.NotifyCeaseAdminShutdown,
		},
		{
			name: "collision resolution",
			trigger: func(t *testing.T, p *closePeer) {
				require.NoError(t, p.session.CloseWithNotification(message.NotifyCease, message.NotifyCeaseConnectionCollision))
			},
			code: message.NotifyCease, subcode: message.NotifyCeaseConnectionCollision,
		},
		{
			name: "hold timer expiry",
			trigger: func(t *testing.T, p *closePeer) {
				// The keepalive timer fires first on the same clock; stopped, the
				// hold expiry is the one callback the advance runs.
				p.session.timers.StopKeepaliveTimer()
				fired := make(chan struct{})
				go func() {
					defer close(fired)
					p.timerClock.Add(wedgeHold)
				}()
				joinOnCleanup(t, fired, "the hold timer callback")
			},
			code: message.NotifyHoldTimerExpired, subcode: 0,
		},
		{
			name:     "send hold timer expiry",
			sendHold: wedgeSendHold,
			// The Send Hold Timer runs on the real clock: it expires a
			// SendHoldTime after the session's last write, with no trigger.
			trigger: func(*testing.T, *closePeer) {},
			code:    message.NotifySendHoldTimerExpired, subcode: 0,
		},
		{
			name: "read loop error",
			trigger: func(_ *testing.T, p *closePeer) {
				// A header whose Type is no BGP message type.
				bad := make([]byte, message.HeaderLen)
				for i := range 16 {
					bad[i] = 0xFF
				}
				bad[16], bad[17] = 0x00, 0x13
				bad[18] = 0x7F
				go func() { _, _ = p.client.Write(bad) }()
			},
			code: message.NotifyMessageHeader, subcode: message.NotifyHeaderBadType,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := closeEstablished(t, tc.sendHold)

			tc.trigger(t, p)

			requireConnClosed(t, p.drainErr)
			frames := closeFrames(t, collectWire(p.wire))
			require.NotEmpty(t, frames, "AC-5: the connection closed with nothing written to it after the OPEN")
			last := frames[len(frames)-1]
			require.Equal(t, msgtype.TypeNOTIFICATION, msgtype.MessageType(last[18]),
				"AC-5: the last message before the close must be the NOTIFICATION")
			require.GreaterOrEqual(t, len(last), message.HeaderLen+2, "a NOTIFICATION carries a code and a subcode")
			require.Equal(t, uint8(tc.code), last[19], "NOTIFICATION error code")
			require.Equal(t, tc.subcode, last[20], "NOTIFICATION error subcode")
		})
	}
}

// TestPeerRemovalAndStopCloseBehindStuckWriter proves the two peer-level stops
// that send a Cease close the connection and return while another writer holds
// the session's write path: the removal of a peer by a reload
// (Peer.stopWithCease) and the daemon stop (Peer.shutdownNotify). Each waits
// the control budget for its Cease, ten seconds here, so each sub-test takes
// that long when the stop behaves.
//
// VALIDATES: AC-4, reload removal and daemon stop -- the peer end sees the
// close within the control budget, the call returns while the writer still
// holds the path, and Run returns ErrTeardown.
// PREVENTS: a stop that waits for the write path a second time in the close,
// or that returns only once the stuck writer lets go.
func TestPeerRemovalAndStopCloseBehindStuckWriter(t *testing.T) {
	cases := []struct {
		name string
		stop func(peer *Peer)
	}{
		{"peer removal by reload", func(peer *Peer) { peer.stopWithCease(message.NotifyCeasePeerDeconfigured) }},
		{"daemon stop", func(peer *Peer) { peer.shutdownNotify() }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := wedgeEstablished(t, 0)
			peer := closeAttachPeer(p.session)

			release := holdWritePath(p.session)
			defer release()

			returned := make(chan struct{})
			go func() {
				defer close(returned)
				tc.stop(peer)
			}()
			joinOnCleanup(t, returned, "the "+tc.name)

			requirePeerSawClose(t, p, p.session.controlWriteDeadline()+wedgeSlack, "the "+tc.name+" started")

			// The writer still holds the path: the stop gives it closeClearWait
			// to let go, then returns without it.
			select {
			case <-returned:
			case <-time.After(closeClearWait + wedgeSlack):
				t.Fatal("the " + tc.name + " had not returned " + (closeClearWait + wedgeSlack).String() +
					" after the socket closed: it is waiting for the writer that still holds the write path")
			}

			release()
			requireRunReturned(t, p, ErrTeardown)
		})
	}
}

// TestCloseHoldsNoSessionLockWhileWaiting proves a close that waits for the
// write path holds no session lock meanwhile. Another writer holds the path, a
// close waits behind it for the control budget, and a reader of the session's
// connection (Session.Conn, which takes s.mu) must still return.
//
// closeConn stops the Send Hold Timer before anything else, so the timer's
// cleared deadline shows the close has started.
//
// VALIDATES: the Critical Review "Correctness" row -- no path holds s.mu across
// a wait for the write path.
// PREVENTS: the close taking s.mu and then waiting on writeMu, which spread one
// stuck writer to every reader of the session, to a source peer's read loop
// and to a policy reload.
func TestCloseHoldsNoSessionLockWhileWaiting(t *testing.T) {
	p := wedgeEstablished(t, 0)
	require.NotZero(t, p.session.sendHoldDeadline.Load(),
		"precondition: the Send Hold Timer must be armed, or its stop cannot show the close started")

	release := holdWritePath(p.session)
	defer release()

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		p.session.closeConn()
	}()
	joinOnCleanup(t, closed, "closeConn")

	require.Eventually(t, func() bool { return p.session.sendHoldDeadline.Load() == 0 },
		runExitDeadline, time.Millisecond, "precondition: closeConn never started")
	require.Never(t, func() bool {
		select {
		case <-closed:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond,
		"precondition: closeConn returned with the write path held, so it was not waiting for it")

	read := make(chan net.Conn, 1)
	go func() { read <- p.session.Conn() }()
	select {
	case conn := <-read:
		require.NotNil(t, conn, "the connection stays installed while the close waits for the write path")
	case <-time.After(wedgeSlack):
		t.Fatal("a reader of the session's connection was still blocked " + wedgeSlack.String() +
			" into a close that waits for the write path: the close holds s.mu across that wait")
	}

	// The writer lets go inside the close's wait: the close then takes the
	// path, closes the connection and returns.
	release()
	select {
	case <-closed:
	case <-time.After(wedgeSlack):
		t.Fatal("closeConn did not return within " + wedgeSlack.String() + " of the write path coming free")
	}
	requirePeerSawClose(t, p, wedgeSlack, "the close took the write path")
}

// TestSessionEndsWhileWriterParkedOnPlugin proves a session ends while a writer
// holds its write path for good, the way a writer that waits on a plugin does:
// no socket close wakes it. The hold timer expires, the test never releases the
// path, and the socket must still close and Run must still return.
//
// It claims nothing about the peer reconnecting: the peer's session-end path
// joins work that waits on the same writer.
//
// VALIDATES: "Consequences of A-3" -- no step of the close routine or of Run's
// exit waits without bound on the write path.
// PREVENTS: a close that clears the connection fields under a blocking take of
// the write path, which leaves Run and the hold timer callback behind a writer
// that never returns.
func TestSessionEndsWhileWriterParkedOnPlugin(t *testing.T) {
	p := wedgeEstablished(t, 0)
	p.session.timers.StopKeepaliveTimer()
	require.True(t, p.session.timers.IsHoldTimerRunning(), "precondition: the hold timer must be armed to expire")

	// Held until the test ends: the deferred release runs after every assertion.
	release := holdWritePath(p.session)
	defer release()

	fired := make(chan struct{})
	go func() {
		defer close(fired)
		p.timerClock.Add(wedgeHold)
	}()
	joinOnCleanup(t, fired, "the hold timer callback")

	requirePeerSawClose(t, p, holdExpiryNotifyDeadline+wedgeSlack, "the hold timer expired")
	requireRunReturned(t, p, ErrHoldTimerExpired)
}

// TestCloseDoesNotWaitTwiceAfterBusyNotification proves the bound a holder of
// openMu relies on: a close that follows a NOTIFICATION which found the write
// path held does not wait for the path again. The NOTIFICATION is sent with a
// short deadline so the test does not pay the control budget for it; the close
// is the plain closeConn, whose own wait would be the control budget.
//
// VALIDATES: the binding Phase 3 decision -- openMu is held across one bounded
// wait for the write path, never two.
// PREVENTS: every close site that sends a NOTIFICATION and then closes waiting
// twice the control budget behind one stuck writer, with openMu held.
func TestCloseDoesNotWaitTwiceAfterBusyNotification(t *testing.T) {
	p := wedgeEstablished(t, 0)

	release := holdWritePath(p.session)
	defer release()

	secondWait := p.session.controlWriteDeadline()
	require.Less(t, closeClearWait+wedgeSlack, secondWait,
		"precondition: the bound below must be shorter than a second wait for the write path")

	err := p.session.sendNotificationWithin(p.session.Conn(), message.NotifyCease,
		message.NotifyCeaseAdminShutdown, nil, 50*time.Millisecond)
	require.ErrorIs(t, err, errWritePathBusy,
		"precondition: the NOTIFICATION must find the write path held, or the close has nothing to remember")

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		p.session.closeConn()
	}()
	joinOnCleanup(t, closed, "closeConn")

	requirePeerSawClose(t, p, wedgeSlack, "the close started after a NOTIFICATION that found the write path held")
	select {
	case <-closed:
	case <-time.After(closeClearWait + wedgeSlack):
		t.Fatal("closeConn had not returned " + (closeClearWait + wedgeSlack).String() +
			" after a NOTIFICATION that found the write path held: it waited for the path a second time")
	}
}

// TestSecondConnectionOnAReusedSessionCloses proves the close is idempotent per
// connection and not per session. The first connection is closed behind a held
// write path, which leaves the connection fields set and the socket marked
// closed; a second close of it returns at once. A second connection installed
// on the same Session, as a passive peer's Accept does, must then be closed by
// its own close.
//
// VALIDATES: the binding Phase 3 decision -- the key of the close's idempotence
// is the connection, and the routine does not seal the session.
// PREVENTS: a per-session "closed" latch, which would make the close of every
// later connection of a reused Session a no-op and leak its socket.
func TestSecondConnectionOnAReusedSessionCloses(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session := NewSession(settings)

	first, _, firstEnded := pipeSession(t, session)
	defer func() { _ = first.Close() }()

	release := holdWritePath(session)
	defer release()

	session.closeConnWithin(50 * time.Millisecond)
	requireConnClosed(t, firstEnded)
	require.NotNil(t, session.Conn(),
		"precondition: the first close must leave the fields set behind the held write path, "+
			"or nothing distinguishes a close per connection from a latch per session")

	again := make(chan struct{})
	go func() {
		defer close(again)
		session.closeConn()
	}()
	select {
	case <-again:
	case <-time.After(wedgeSlack):
		t.Fatal("a second close of the same connection had not returned after " + wedgeSlack.String() +
			": it waited for the write path again")
	}
	require.False(t, session.tearingDown.Load(), "the close routine must not seal the session")

	release()

	second, _, secondEnded := pipeSession(t, session)
	defer func() { _ = second.Close() }()

	session.closeConn()
	requireConnClosed(t, secondEnded)
	require.Nil(t, session.Conn(), "the close of the second connection must clear the connection fields")
}
