// Design: docs/architecture/core-design.md -- the one bounded close routine of a session connection
// Related: session_connection.go -- closeConnWithin, closeFlushed, dropConn, clearClosedConn
// Related: session_wedge_test.go -- wedgeEstablished and the three closes behind a stuck writer
// Related: session_write.go -- sendNotificationWithin, sendHoldTimerExpired

package reactor

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/test/sim"
)

// closeSendHold is the SendHoldTime of the close that proves the Send Hold
// Timer's NOTIFICATION with the write path free. It runs on the real clock and
// nothing has to happen before it expires, so it is as short as the timer
// allows the test to be.
const closeSendHold = time.Second

// closeLogSink swaps the session logger for a JSON one that keeps every line
// at WARN and above, until the test ends. A test calls it before it builds its
// session, so no line is written to the logger it replaced.
func closeLogSink(t *testing.T) *syncBuffer {
	t.Helper()
	sink := &syncBuffer{}
	restore := swapSessionLogger(func() *slog.Logger {
		return slog.New(slog.NewJSONHandler(sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	})
	t.Cleanup(restore)
	return sink
}

// closeLogRecords parses what a closeLogSink holds, one record per line.
func closeLogRecords(t *testing.T, sink *syncBuffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(sink.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record), "log line: %s", line)
		records = append(records, record)
	}
	return records
}

// closeRecordsWith answers the records whose key holds value.
func closeRecordsWith(records []map[string]any, key, value string) []map[string]any {
	var matched []map[string]any
	for _, record := range records {
		if record[key] == value {
			matched = append(matched, record)
		}
	}
	return matched
}

// closeHeldPathEvents answers the report bus events a close behind a held
// write path raised.
func closeHeldPathEvents() []report.Issue {
	var events []report.Issue
	for _, event := range report.Errors(0) {
		if event.Code == diagnostic.CodeDoctorBGPWritePathHeld {
			events = append(events, event)
		}
	}
	return events
}

// closeUnreadSession answers a Session on a net.Pipe whose peer end reads
// nothing, with one KEEPALIVE in its write buffer: a final flush of it blocks
// until something bounds it. No handshake runs and no goroutine reads either
// end. It also answers the peer end and ze's end of the pipe.
func closeUnreadSession(t *testing.T) (session *Session, client, server net.Conn) {
	t.Helper()

	session = NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301))
	server, client = net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	session.writeMu.Lock()
	_, err := session.bufWriter.Write(message.PackTo(message.NewKeepalive(), nil))
	session.writeMu.Unlock()
	require.NoError(t, err)
	return session, client, server
}

// requirePeerEndClosed asserts the peer end of an unread pipe meets ze's close
// on its first read, and none of the octets the flush could not deliver.
//
// The read is bounded by a timer and not by a read deadline: a net.Pipe end
// whose other end is closed refuses SetReadDeadline with io.ErrClosedPipe, so
// arming one fails exactly when the close under test worked. A read that is
// still blocked at the bound is ended by the fixture's cleanup, which closes
// the peer end.
func requirePeerEndClosed(t *testing.T, client net.Conn) {
	t.Helper()
	type readResult struct {
		octets int
		err    error
	}
	read := make(chan readResult, 1)
	go func() {
		buf := make([]byte, message.HeaderLen)
		n, err := client.Read(buf)
		read <- readResult{octets: n, err: err}
	}()
	select {
	case got := <-read:
		require.Zero(t, got.octets, "the peer end read octets of a flush that had already failed on its budget")
		require.True(t, errors.Is(got.err, io.EOF),
			"the socket was not closed: the peer end's read ended with %v", got.err)
	case <-time.After(wedgeSlack):
		t.Fatal("the socket was still open " + wedgeSlack.String() + " after the close returned: the peer end's read is blocked")
	}
}

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
			sendHold: closeSendHold,
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

// TestTwoClosesBehindHeldPathReportOnce proves one connection closed behind a
// held write path is reported once, however many closes meet that path. Two
// closes of the same connection start together, both wait for a path the test
// never releases, and both give up at the same moment.
//
// VALIDATES: one close step for one connection -- one ERROR line and one event
// on the report bus under doctor-bgp-write-path-held, and both closes return.
// PREVENTS: the close that lost the race reporting the held path a second time
// and dropping a socket the first one already dropped, which `show doctor`
// counts as two problems on one peer.
func TestTwoClosesBehindHeldPathReportOnce(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	sink := closeLogSink(t)

	session := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301))
	client, _, ended := pipeSession(t, session)
	defer func() { _ = client.Close() }()

	// Held until the test ends: no close ever gets the path.
	release := holdWritePath(session)
	defer release()

	const pathWait = 200 * time.Millisecond
	var closes sync.WaitGroup
	for range 2 {
		closes.Go(func() { session.closeConnWithin(pathWait) })
	}
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		closes.Wait()
	}()
	select {
	case <-returned:
	case <-time.After(pathWait + closeClearWait + wedgeSlack):
		t.Fatal("two closes of one connection had not both returned " +
			(pathWait + closeClearWait + wedgeSlack).String() + " after they started")
	}
	requireConnClosed(t, ended)

	require.Len(t, closeHeldPathEvents(), 1, "one connection closed behind a held write path raises one event")
	coded := closeRecordsWith(closeLogRecords(t, sink), "code", diagnostic.CodeDoctorBGPWritePathHeld)
	require.Len(t, coded, 1, "one connection closed behind a held write path logs one line; log:\n%s", sink.String())
	require.Equal(t, "ERROR", coded[0]["level"])
	require.LessOrEqual(t, len(writePathHeldFix), report.FixMax, "the bus refuses a longer fix")
	require.Contains(t, coded[0]["fix"], "ze show doctor", "the fix must name the step for a plugin that takes no events")
}

// TestCloseBehindAGracefulCloseReportsNothing proves a close that meets
// another close of the same connection in its final flush does not take that
// close for a stuck writer. The peer end reads nothing, so the first close
// holds the write path for its whole flush budget; a second close, whose own
// wait is far shorter, arrives while it does.
//
// VALIDATES: the second close returns with no doctor-bgp-write-path-held line
// or event, and the first one still closes the socket.
// PREVENTS: a hold timer expiry, whose close waits one second, reporting a
// held write path and blaming the peer because a teardown was flushing its
// Cease for a second and a tenth.
func TestCloseBehindAGracefulCloseReportsNothing(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	sink := closeLogSink(t)

	session, client, _ := closeUnreadSession(t)

	first := make(chan struct{})
	go func() {
		defer close(first)
		session.closeConn()
	}()
	joinOnCleanup(t, first, "the flushing close")

	// The first close holds the write path from before its flush until the
	// socket is closed. A probe that wins the path gives it straight back.
	require.Eventually(t, func() bool {
		if session.writeMu.TryLock() {
			session.writeMu.Unlock()
			return false
		}
		return true
	}, closeFlushBudget/2, time.Millisecond,
		"precondition: the first close never held the write path for its flush")

	second := make(chan struct{})
	go func() {
		defer close(second)
		session.closeConnWithin(50 * time.Millisecond)
	}()
	select {
	case <-second:
	case <-time.After(wedgeSlack):
		t.Fatal("a close behind another close's flush had not returned after " + wedgeSlack.String())
	}

	select {
	case <-first:
	case <-time.After(closeFlushBudget + wedgeSlack):
		t.Fatal("the flushing close had not returned " + (closeFlushBudget + wedgeSlack).String() + " after it started")
	}
	requirePeerEndClosed(t, client)

	require.Empty(t, closeHeldPathEvents(), "a close in its final flush is no stuck writer: nothing to report")
	require.Empty(t, closeRecordsWith(closeLogRecords(t, sink), "code", diagnostic.CodeDoctorBGPWritePathHeld),
		"log:\n%s", sink.String())
}

// closeGatedConn is ze's end of a connection whose Write blocks until the test
// releases it, whatever deadline the socket carries: a close that flushes to
// it stays in its flush, holding the claim on the socket and the write path,
// for exactly as long as the test decides.
type closeGatedConn struct {
	net.Conn
	// entered closes when the first Write began.
	entered chan struct{}
	// release ends every Write, which then reports its octets as written.
	release chan struct{}
	enter   sync.Once
	closed  atomic.Bool
}

func (c *closeGatedConn) Write(p []byte) (int, error) {
	c.enter.Do(func() { close(c.entered) })
	<-c.release
	return len(p), nil
}

func (c *closeGatedConn) Close() error {
	c.closed.Store(true)
	return c.Conn.Close()
}

// TestSecondCloseWaitsForTheClaimant proves a close that returned means the
// connection is no longer installed, when another close of the same connection
// is still at work. The first close is held inside its final flush by a
// connection whose Write the test gates, so it holds the claim on the socket
// and the write path. A second close of the same connection starts then.
//
// VALIDATES: the second close does not return while the first is in its flush;
// once the flush is released it returns with the socket closed and Conn() nil,
// and it logs and raises nothing.
// PREVENTS: a close that sees the claim and returns at once with the fields
// set: Session.Run then returns over an installed connection, and Accept,
// Connect and the Peer's next attempt read a session that still has one
// (ErrAlreadyConnected, a dial that is skipped).
func TestSecondCloseWaitsForTheClaimant(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	sink := closeLogSink(t)

	server, client := net.Pipe()
	gated := &closeGatedConn{Conn: server, entered: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gated.release) }) }
	t.Cleanup(func() {
		release()
		_ = client.Close()
		_ = server.Close()
	})

	session := NewSession(NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301))
	session.mu.Lock()
	session.conn = gated
	session.bufReader = bufio.NewReaderSize(gated, 65536)
	session.bufWriter = bufio.NewWriterSize(gated, 16384)
	session.mu.Unlock()
	session.writeMu.Lock()
	_, err := session.bufWriter.Write(message.PackTo(message.NewKeepalive(), nil))
	session.writeMu.Unlock()
	require.NoError(t, err)

	first := make(chan struct{})
	go func() {
		defer close(first)
		session.closeConn()
	}()
	joinOnCleanup(t, first, "the flushing close")
	select {
	case <-gated.entered:
	case <-time.After(runExitDeadline):
		t.Fatal("precondition: the first close never reached its flush")
	}

	second := make(chan struct{})
	go func() {
		defer close(second)
		session.closeConn()
	}()
	joinOnCleanup(t, second, "the second close")

	require.Never(t, func() bool {
		select {
		case <-second:
			return true
		default:
			return false
		}
	}, closeClaimWait/4, 10*time.Millisecond,
		"the second close returned while the first was still in its flush: it left the connection installed")
	require.False(t, gated.closed.Load(), "precondition: the first close must still be in its flush")
	require.NotNil(t, session.Conn(), "precondition: the connection is installed until a close clears it")

	release()
	select {
	case <-second:
	case <-time.After(wedgeSlack):
		t.Fatal("the second close had not returned " + wedgeSlack.String() + " after the first close's flush ended")
	}
	require.True(t, gated.closed.Load(), "a close returned with the socket still open")
	require.Nil(t, session.Conn(), "a close returned with the connection still installed")

	require.Empty(t, closeHeldPathEvents(), "a close behind another close has nothing to report")
	require.Empty(t, closeRecordsWith(closeLogRecords(t, sink), "code", diagnostic.CodeDoctorBGPWritePathHeld),
		"log:\n%s", sink.String())
}

// TestCloseFlushIsBounded proves the final flush of a close has a write budget
// of its own. The write path is free, a KEEPALIVE sits in the write buffer, and
// the peer end reads nothing, so the flush blocks until its budget ends it.
//
// VALIDATES: closeFlushed gives the buffered octets closeFlushBudget and no
// longer -- the close returns within that bound, the socket is closed and the
// connection fields are cleared.
// PREVENTS: a close that took the write path itself and then waits without
// bound on a peer that stopped reading, with the socket open.
func TestCloseFlushIsBounded(t *testing.T) {
	session, client, _ := closeUnreadSession(t)

	started := time.Now()
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		session.closeConn()
	}()
	joinOnCleanup(t, closed, "closeConn")

	select {
	case <-closed:
	case <-time.After(closeFlushBudget + wedgeSlack):
		t.Fatal("closeConn had not returned " + (closeFlushBudget + wedgeSlack).String() +
			" after it started: its final flush waits without bound on a peer that reads nothing")
	}
	require.GreaterOrEqual(t, time.Since(started), closeFlushBudget,
		"the close returned before its flush budget: the buffered octets were not given the time they are owed")

	requirePeerEndClosed(t, client)
	require.Nil(t, session.Conn(), "the close must clear the connection fields")
}

// TestTeardownDuringHeldRunDeliversCease proves an operator teardown that
// lands in the middle of a held run of UPDATEs, the form the initial sync
// writes in, still tells a peer that reads why the session ends. One goroutine
// holds the write path and sends UPDATE after UPDATE for longer than the
// control budget unless a send refuses; the teardown starts once the run is
// under way.
//
// VALIDATES: the held run ends on the session's seal within one message
// (errAdjOutSession), the teardown returns far inside the control budget, and
// the last message the peer read before the close is the Cease.
// PREVENTS: a teardown that waits its whole control budget behind a healthy
// held run, then drops the connection with no NOTIFICATION and logs at ERROR
// about a peer that was reading all along.
func TestTeardownDuringHeldRunDeliversCease(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	p := closeEstablished(t, 0)
	// The capture channel is bounded, and a peer end that stops reading would
	// block the run: collect while it writes.
	collected := make(chan []byte, 1)
	go func() { collected <- collectWire(p.wire) }()

	runLimit := p.session.controlWriteDeadline() + wedgeSlack
	underWay := make(chan struct{})
	runEnded := make(chan error, 1)
	go func() {
		p.session.holdWrites()
		defer p.session.releaseWrites()

		var once sync.Once
		stop := time.Now().Add(runLimit)
		// Ends on the first refused send, or at runLimit.
		for third := 0; time.Now().Before(stop); third++ {
			if err := p.session.sendUpdateHeld(budgetAnnounce(byte(third))); err != nil {
				runEnded <- err
				return
			}
			once.Do(func() { close(underWay) })
			time.Sleep(time.Millisecond)
		}
		runEnded <- nil
	}()
	select {
	case <-underWay:
	case <-time.After(runExitDeadline):
		t.Fatal("precondition: the held run never sent its first UPDATE")
	}

	returned := make(chan error, 1)
	go func() { returned <- p.session.Teardown(message.NotifyCeaseAdminShutdown, "") }()
	select {
	case err := <-returned:
		require.NoError(t, err)
	case <-time.After(wedgeSlack):
		t.Fatal("Session.Teardown had not returned " + wedgeSlack.String() +
			" after it started: it is waiting behind a held run that the seal did not end")
	}

	select {
	case err := <-runEnded:
		require.ErrorIs(t, err, errAdjOutSession, "the held run must end on the seal, with nothing more written")
	case <-time.After(wedgeSlack):
		t.Fatal("the held run was still writing after the teardown returned")
	}

	requireConnClosed(t, p.drainErr)
	var sent []byte
	select {
	case sent = <-collected:
	case <-time.After(wedgeSlack):
		t.Fatal("the peer end's capture did not end with the connection")
	}
	frames := closeFrames(t, sent)
	require.NotEmpty(t, frames)
	last := frames[len(frames)-1]
	require.Equal(t, msgtype.TypeNOTIFICATION, msgtype.MessageType(last[18]),
		"the last message a reading peer got before the close must be the teardown's NOTIFICATION")
	require.Equal(t, uint8(message.NotifyCease), last[19], "NOTIFICATION error code")
	require.Equal(t, uint8(message.NotifyCeaseAdminShutdown), last[20], "NOTIFICATION error subcode")
	require.Empty(t, closeHeldPathEvents(), "a held run that ended on the seal is no held write path")
}

// TestBusyNotificationIsLoggedOnceByTheClose proves a NOTIFICATION that found
// the write path held is logged once, by the close that follows it. The
// NOTIFICATION is sent through the logging sender every close site uses, with a
// short deadline, and the close is the plain one.
//
// VALIDATES: the close's ERROR line under doctor-bgp-write-path-held is the one
// record that no NOTIFICATION was sent; the sender logs nothing for a busy
// path.
// PREVENTS: one fact logged twice, at WARN as "notification send failed" and
// at ERROR as "no NOTIFICATION was sent to the peer".
func TestBusyNotificationIsLoggedOnceByTheClose(t *testing.T) {
	sink := closeLogSink(t)
	p := wedgeEstablished(t, 0)

	release := holdWritePath(p.session)
	defer release()

	p.session.logNotifyErrWithin(p.session.Conn(), message.NotifyCease,
		message.NotifyCeaseAdminShutdown, nil, 50*time.Millisecond)

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		p.session.closeConn()
	}()
	joinOnCleanup(t, closed, "closeConn")
	select {
	case <-closed:
	case <-time.After(closeClearWait + wedgeSlack):
		t.Fatal("closeConn had not returned " + (closeClearWait + wedgeSlack).String() +
			" after a NOTIFICATION that found the write path held")
	}

	records := closeLogRecords(t, sink)
	require.Empty(t, closeRecordsWith(records, "msg", "notification send failed"),
		"the sender must leave a busy write path to the close's line; log:\n%s", sink.String())
	coded := closeRecordsWith(records, "code", diagnostic.CodeDoctorBGPWritePathHeld)
	require.Len(t, coded, 1, "log:\n%s", sink.String())
	require.Equal(t, "ERROR", coded[0]["level"])
	require.Contains(t, coded[0]["msg"], "no NOTIFICATION was sent to the peer")
}
