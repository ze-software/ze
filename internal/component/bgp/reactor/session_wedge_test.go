// Design: docs/architecture/core-design.md -- BGP session close behind a held write path
// Related: session_connection.go -- closeConn, teardownSession, ceaseAfterPanic
// Related: session_write.go -- writeMessageWithin, sendHoldTimerExpired
// Related: session.go -- the OnHoldTimerExpires callback, Session.Run

package reactor

import (
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/test/sim"
)

// wedgeHold is the hold time every session below negotiates. It is short so
// the control-message budget (Session.controlWriteDeadline) sits at its
// ten-second floor, which is the bound the operator teardown test waits out.
const wedgeHold = 9 * time.Second

// wedgeSendHold is the SendHoldTime of the Send Hold Timer test. It runs on the
// real clock, so it is the time the test has to take the write path after the
// session reached Established: five seconds, because a loaded machine can
// take longer than one second to get there, and the expiry then runs with the
// path free and the test fails on its own precondition. The test waits it out once, and its bound on
// a wedge runs from the expiry: what is left of this when the test holds the
// write path, plus the NOTIFICATION bound and wedgeSlack. PeerSettings
// carries it directly, so the 480-second floor parsePeerFromTree puts on an
// operator's value does not apply.
const wedgeSendHold = 5 * time.Second

// wedgeSlack is the real time a loaded machine may add to a bound before a test
// calls it missed. It bounds every wait that has no protocol bound of its own.
const wedgeSlack = 2 * time.Second

// wedgePeer is one Established session whose peer end reads everything and
// writes nothing after the handshake.
type wedgePeer struct {
	session *Session
	// timerClock drives the FSM timers (hold, keepalive) and nothing else. The
	// session's own clock stays the real one, so a bounded wait for the write
	// path elapses in real time whichever clock primitive it is built on.
	timerClock *sim.FakeClock
	runResult  <-chan error
	// peerEnded carries the error that ended the peer end's reads: io.EOF once
	// ze closed its end of the connection.
	peerEnded <-chan error
}

// wedgeEstablished drives a real Session to Established over a net.Pipe with
// Run executing, the way rfc9687Established does, with two differences the
// wedge tests need: only the FSM timers run on a fake clock, and the peer end
// is read with no deadline, because the operator teardown test waits longer
// than startDrain's own bound.
func wedgeEstablished(t *testing.T, sendHold time.Duration) *wedgePeer {
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
	peerEnded := wedgeReadPeerEnd(client)

	runResult := rfc9687Run(t, session)

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

	return &wedgePeer{session: session, timerClock: timerClock, runResult: runResult, peerEnded: peerEnded}
}

// wedgeReadPeerEnd reads the peer end of the pipe until the pipe dies and
// publishes the error that ended it. The loop has no bound of its own: it ends
// when ze closes its end (io.EOF) or when the fixture's cleanup closes the peer
// end (io.ErrClosedPipe).
func wedgeReadPeerEnd(client net.Conn) <-chan error {
	ended := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := client.Read(buf); err != nil {
				ended <- err
				return
			}
		}
	}()
	return ended
}

// holdWritePath takes the session's write path the way a writer blocked in a
// socket write holds it, and answers its release. The release is safe to call
// more than once, and each test MUST defer it: a wedged close path is waiting
// on this lock, and the fixture's cleanups can only finish once it is free.
func holdWritePath(session *Session) (release func()) {
	session.writeMu.Lock()
	var once sync.Once
	return func() { once.Do(session.writeMu.Unlock) }
}

// requirePeerSawClose asserts the peer end read ze's close of the connection
// no later than within. It fails, and never hangs, when the connection is still
// open then: that is the wedge.
func requirePeerSawClose(t *testing.T, p *wedgePeer, within time.Duration, event string) {
	t.Helper()
	select {
	case err := <-p.peerEnded:
		require.True(t, errors.Is(err, io.EOF),
			"the peer end stopped reading for another reason than ze closing the connection: %v", err)
	case <-time.After(within):
		t.Fatal("the connection was still open " + within.String() + " after " + event +
			": the close is waiting for the write path another writer holds, with the socket open")
	}
}

// requireRunReturned asserts Session.Run returned want and left the FSM Idle.
func requireRunReturned(t *testing.T, p *wedgePeer, want error) {
	t.Helper()
	select {
	case err := <-p.runResult:
		require.ErrorIs(t, err, want)
	case <-time.After(runExitDeadline):
		t.Fatal("Run did not return within " + runExitDeadline.String() +
			" of the writer releasing the write path")
	}
	require.Equal(t, fsm.StateIdle, p.session.State(), "the session's FSM must be left Idle")
}

// joinOnCleanup makes the test wait for a goroutine it started, so a callback
// the wedge parked cannot run on into the next test.
func joinOnCleanup(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	t.Cleanup(func() {
		select {
		case <-done:
		case <-time.After(runExitDeadline):
			t.Error(what + " did not return after the write path was released")
		}
	})
}

// TestHoldExpiryClosesBehindStuckWriter proves a receive hold timer expiry
// drops the connection while another writer holds the session's write path.
// The session is Established over a pipe, the test holds writeMu the way a
// writer blocked in a socket write does, and the production OnHoldTimerExpires
// callback is fired through the timers' clock.
//
// VALIDATES: AC-2 -- the peer end sees the close within the one-second
// NOTIFICATION bound, then Run returns ErrHoldTimerExpired with the FSM Idle
// once the writer lets go, as a writer whose socket was closed does.
// PREVENTS: the expiry waiting on writeMu for its NOTIFICATION with the socket
// open, which leaves a peer that stopped reading in a dead session for good.
//
// RFC 4271 Section 8.2.2, Established, HoldTimer_Expires (Event 10):
// "drops the TCP connection".
//
// RFC requirement: RFC4271-8.2.2-4 positive -- with the session's write path
// held by another writer, a HoldTimer expiry in Established closes the TCP
// connection: the peer end reads EOF within holdExpiryNotifyDeadline, plus two
// seconds of scheduling slack, of the expiry.
func TestHoldExpiryClosesBehindStuckWriter(t *testing.T) {
	p := wedgeEstablished(t, 0)

	// The keepalive timer is a writer of its own and fires before the hold
	// timer on the same clock. Stopped, the hold expiry is the one callback the
	// advance below runs.
	p.session.timers.StopKeepaliveTimer()
	require.True(t, p.session.timers.IsHoldTimerRunning(), "precondition: the hold timer must be armed to expire")

	release := holdWritePath(p.session)
	defer release()

	// The fake clock runs the callback on the goroutine that advances it, and
	// the callback is what the wedge parks, so the advance gets its own.
	fired := make(chan struct{})
	go func() {
		defer close(fired)
		p.timerClock.Add(wedgeHold)
	}()
	joinOnCleanup(t, fired, "the hold timer callback")

	requirePeerSawClose(t, p, holdExpiryNotifyDeadline+wedgeSlack, "the hold timer expired")

	release()
	requireRunReturned(t, p, ErrHoldTimerExpired)
}

// TestSendHoldExpiryClosesBehindStuckWriter proves an RFC 9687 Send Hold Timer
// expiry drops the connection while another writer holds the session's write
// path, which is the state the timer exists to end: a writer that cannot
// complete. The Send Hold Timer runs on the real clock with a SendHoldTime of
// wedgeSendHold, and the test takes writeMu before it expires.
//
// VALIDATES: AC-1 -- the peer end sees the close within the one-second
// NOTIFICATION bound of the expiry, then Run returns ErrSendHoldTimerExpired
// with the FSM Idle once the writer lets go.
// PREVENTS: sendHoldTimerExpired waiting on writeMu for its optional
// NOTIFICATION, so the timer that detects a stuck write is stuck behind it.
//
// RFC 9687 Section 4.3, SendHoldTimer_Expires (Event 29):
// "drops the TCP connection".
//
// RFC requirement: RFC9687-4.3-5 positive -- with the session's write path held
// by another writer, a Send Hold Timer expiry closes the TCP connection: the
// peer end reads EOF within holdExpiryNotifyDeadline, plus two seconds of
// scheduling slack, of the expiry.
func TestSendHoldExpiryClosesBehindStuckWriter(t *testing.T) {
	p := wedgeEstablished(t, wedgeSendHold)

	release := holdWritePath(p.session)
	defer release()

	// sendHoldDeadline is zero once the timer expired or was stopped
	// (stopSendHoldTimerLocked), so non-zero here means the expiry is still to
	// come and will meet the held write path.
	expiry := p.session.sendHoldDeadline.Load()
	require.NotZero(t, expiry,
		"precondition: the Send Hold Timer expired before the test held the write path, "+
			"so the expiry ran with the path free and proved nothing about the wedge")

	// The bound runs from the expiry, as the claim says, and not from here:
	// the wait is what is left until the timer expires, then the NOTIFICATION
	// bound and the slack. The deadline read above is final: only a send
	// moves it (resetSendHoldTimer), under the write path this test holds.
	untilExpiry := max(time.Until(time.Unix(0, expiry)), 0)
	requirePeerSawClose(t, p, untilExpiry+holdExpiryNotifyDeadline+wedgeSlack,
		"the write path was held, "+untilExpiry.String()+" before the Send Hold Timer expiry")

	release()
	requireRunReturned(t, p, ErrSendHoldTimerExpired)
}

// TestTeardownClosesBehindStuckWriter proves an operator teardown closes the
// connection and returns while another writer holds the session's write path.
// The wait it is allowed is the control-message budget, ten seconds here, so
// this test takes that long when the teardown behaves.
//
// VALIDATES: AC-4, operator teardown -- the peer end sees the close within the
// control budget, and Session.Teardown returns once the writer lets go.
// PREVENTS: teardownSession waiting on writeMu for its Cease NOTIFICATION, and
// closeConn waiting on it for its final flush, with the socket open: an
// operator who removes a peer that stopped reading then never gets the prompt
// back.
func TestTeardownClosesBehindStuckWriter(t *testing.T) {
	p := wedgeEstablished(t, 0)

	release := holdWritePath(p.session)
	defer release()

	returned := make(chan struct{})
	var teardownErr error
	go func() {
		defer close(returned)
		teardownErr = p.session.Teardown(message.NotifyCeaseAdminShutdown, "")
	}()
	joinOnCleanup(t, returned, "Session.Teardown")

	requirePeerSawClose(t, p, p.session.controlWriteDeadline()+wedgeSlack, "the operator teardown started")

	release()
	select {
	case <-returned:
		require.NoError(t, teardownErr)
	case <-time.After(wedgeSlack):
		t.Fatal("Session.Teardown did not return within " + wedgeSlack.String() +
			" of the socket closing and the writer releasing the write path")
	}
}

// TestDeadlinePokeFailsBlockedTCPWrite proves the Go runtime contract the close
// routine relies on to interrupt a writer: on a real TCP connection, a Write
// blocked because the peer reads nothing fails with a timeout when another
// goroutine sets a write deadline in the past. No ze code is under test.
//
// VALIDATES: assumption A-1 of spec-bgp-session-write-wedge.
// PREVENTS: a close routine that expires the write deadline and then waits for
// a writer the poke never woke.
func TestDeadlinePokeFailsBlockedTCPWrite(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // Test code
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	writer, err := net.Dial("tcp", listener.Addr().String()) //nolint:noctx // Test code
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	// The peer end stays open and is never read, so its receive window shuts.
	reader, err := listener.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })

	var octetsWritten atomic.Int64
	writeEnded := make(chan error, 1)
	go func() {
		chunk := make([]byte, 64<<10)
		// Ends on the first failed Write: the poke below, or the cleanup's Close.
		for {
			n, writeErr := writer.Write(chunk)
			octetsWritten.Add(int64(n))
			if writeErr != nil {
				writeEnded <- writeErr
				return
			}
		}
	}()

	// The writer is blocked once the socket buffers are full: the count of
	// octets it got through stops moving between two looks.
	octetsSeen := int64(-1)
	require.Eventually(t, func() bool {
		octets := octetsWritten.Load()
		stalled := octets > 0 && octets == octetsSeen
		octetsSeen = octets
		return stalled
	}, runExitDeadline, 200*time.Millisecond,
		"precondition: the writer never blocked, so there is no pending Write to interrupt")

	require.NoError(t, writer.SetWriteDeadline(time.Unix(1, 0)))

	select {
	case writeErr := <-writeEnded:
		require.ErrorIs(t, writeErr, os.ErrDeadlineExceeded,
			"A-1: a write deadline in the past must fail the blocked Write with a timeout")
	case <-time.After(wedgeSlack):
		t.Fatal("A-1: the Write was still blocked " + wedgeSlack.String() +
			" after its deadline was set in the past")
	}
}
