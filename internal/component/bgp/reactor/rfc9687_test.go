// Design: docs/architecture/core-design.md — BGP session send-side liveness
// RFC: rfc/short/rfc9687.md — the Send Hold Timer
// Related: session_write.go — startSendHoldTimer, resetSendHoldTimer, sendHoldTimerExpired
// Related: config.go — parsePeerFromTree, the Section 4.4 constraint on send-hold-time

package reactor

import (
	"log/slog"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/test/sim"
)

// rfc9687SendHold is the SendHoldTime every session below runs with. It is far
// under the 480-second floor parsePeerFromTree enforces on an operator's
// send-hold-time, which is deliberate: these tests drive the FSM attribute
// directly through PeerSettings, and a fake clock makes the wall value
// irrelevant. It sits under the negotiated hold time (90s), the keepalive
// interval (30s) and the connect-retry time (120s), so an advance that crosses
// it crosses no other timer and every assertion below names one mechanism.
const rfc9687SendHold = 10 * time.Second

// rfc9687Peer holds the pieces of one established session the RFC 9687
// assertions read.
type rfc9687Peer struct {
	session   *Session
	clock     *sim.FakeClock
	crc       *fsm.ConnectRetryCounter
	runResult <-chan error
	drainErr  <-chan error
	wire      <-chan []byte
	log       *syncBuffer
}

// armed reports whether the Send Hold Timer is running. sendHoldDeadline is the
// timer's own "is it running" flag: startSendHoldTimer stores a deadline into
// it and stopSendHoldTimerLocked stores zero, so a non-zero value is the timer
// being armed and nothing else (session_write.go).
func (p *rfc9687Peer) armed() bool { return p.session.sendHoldDeadline.Load() != 0 }

// rfc9687PeerOpen is a well-formed peer OPEN proposing holdTime seconds. The
// hold time is a parameter because RFC 4271 Section 4.2 permits zero, and RFC
// 9687 Section 4.3 gives a zero negotiated HoldTime its own outcome.
func rfc9687PeerOpen(holdTime uint16) []byte {
	return message.PackTo(&message.Open{
		Version: 4, MyAS: 65002, HoldTime: holdTime, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			0x02, 0x06, 0x41, 0x04, 0x00, 0x00, 0xFD, 0xEA, // ASN4
			0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01, // IPv4/Unicast
		},
	}, nil)
}

// rfc9687Established drives a real Session to Established over a net.Pipe with
// Run executing and a FakeClock behind every timer, then arms the resources RFC
// 9687 Section 4.3's Event 29 action list names so their clearing is
// observable.
//
// The ConnectRetryTimer is armed deliberately: ze has no production caller for
// StartConnectRetryTimer, so an assertion that it is zero after Event 29 would
// pass with the whole teardown deleted. Arming it first makes the assertion
// measure the mechanism that zeroes it.
//
// The ConnectRetryCounter is handed in for the same reason. A session with no
// counter counts nothing (fsm.ConnectRetryCounter.Increment on a nil receiver),
// so the increment clause would be unobservable without one.
func rfc9687Established(t *testing.T, peerHoldTime uint16) *rfc9687Peer {
	t.Helper()

	sink := &syncBuffer{}
	lg := slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.SendHoldTime = rfc9687SendHold
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	fc := sim.NewFakeClock(time.Now())
	session.SetClock(fc)

	crc := &fsm.ConnectRetryCounter{}
	session.SetConnectRetryCounter(crc)
	require.NoError(t, session.Start())

	server, client := net.Pipe()
	_ = acceptWithReader(t, session, server, client)
	wire, drainErr := startDrain(t, client)

	runResult := make(chan error, 1)
	go func() { runResult <- session.Run(t.Context()) }()

	// net.Pipe is synchronous, so each write needs its own goroutine and the
	// two must be sequenced: concurrent writers interleave their bytes and ze
	// reads a corrupt header.
	go func() { _, _ = client.Write(rfc9687PeerOpen(peerHoldTime)) }()
	require.Eventually(t, func() bool { return session.State() == fsm.StateOpenConfirm },
		runExitDeadline, 5*time.Millisecond,
		"precondition: ze did not reach OpenConfirm, so the peer OPEN was not accepted")

	go func() { _, _ = client.Write(message.PackTo(message.NewKeepalive(), nil)) }()
	require.Eventually(t, func() bool { return session.State() == fsm.StateEstablished },
		runExitDeadline, 5*time.Millisecond,
		"precondition: the session never reached Established, so Event 29 has nothing to tear down")

	session.timers.StartConnectRetryTimer()
	require.True(t, session.timers.IsConnectRetryTimerRunning(),
		"precondition: the ConnectRetryTimer must be armed for its zeroing to be observable")

	return &rfc9687Peer{
		session: session, clock: fc, crc: crc,
		runResult: runResult, drainErr: drainErr, wire: wire, log: sink,
	}
}

// TestRFC9687SendHoldExpiryRunsTheEvent29ActionList proves that a Send Hold
// Timer expiry runs the WHOLE of the Event 29 action list, not only the
// NOTIFICATION that opens it.
//
// VALIDATES: RFC 9687 Section 4.3, SendHoldTimer_Expires (Event 29), and
// Section 5, which restates the close and the log as capitalised MUSTs.
//
// PREVENTS: the failure RFC 9687 Section 3 exists for surviving the timer that
// detects it -- a local system that has been unable to transmit for the whole
// SendHoldTime keeping its socket, its timers, its retry count and its
// Established state, so the routes it learned over a connection it cannot use
// stay in the RIB.
//
// RFC requirement: RFC9687-4.3-2 positive -- sendHoldTimerExpired logs
// "send hold timer expired (RFC 9687)", which is the Error Code's own name.
// RFC requirement: RFC9687-4.3-3 positive -- Run's exit calls Timers.StopAll,
// releasing the BGP resources the session holds; the KeepaliveTimer is the
// externally visible one.
// RFC requirement: RFC9687-4.3-4 positive -- the same StopAll zeroes the
// ConnectRetryTimer.
// RFC requirement: RFC9687-4.3-5 positive -- sendHoldTimerExpired calls
// closeConn, and Run's defer backs it.
// RFC requirement: RFC9687-4.3-6 positive -- the FSM's HoldTimer_Expires
// handler increments the ConnectRetryCounter by 1.
// RFC requirement: RFC9687-4.3-7 positive -- the same handler changes the state
// to Idle.
// RFC requirement: RFC9687-4.3-10 positive -- the transition out of Established
// leaves the Send Hold Timer stopped.
// RFC requirement: RFC9687-5-1 positive -- "the BGP connection MUST be closed".
// RFC requirement: RFC9687-5-2 positive -- "an error MUST be logged in the
// local system, indicating the 'Send Hold Timer Expired' Error Code".
func TestRFC9687SendHoldExpiryRunsTheEvent29ActionList(t *testing.T) {
	p := rfc9687Established(t, 90)
	require.True(t, p.armed(), "precondition: the Send Hold Timer must be armed to expire")

	p.clock.Add(rfc9687SendHold)

	select {
	case <-p.runResult:
	case <-time.After(runExitDeadline):
		t.Fatal("RFC 9687 Section 4.3 Event 29: the Send Hold Timer expired and Run " +
			"never returned, so none of the action list ran")
	}

	assert.False(t, p.armed(),
		"RFC9687-4.3-10: the Send Hold Timer is stopped following any transition out "+
			"of Established; one left armed here fires against a dead session")
	assert.False(t, p.session.timers.IsConnectRetryTimerRunning(),
		"RFC9687-4.3-4: Event 29 sets the ConnectRetryTimer to zero")
	assert.False(t, p.session.timers.IsKeepaliveTimerRunning(),
		"RFC9687-4.3-3: Event 29 releases all BGP resources; a KeepaliveTimer left "+
			"running keeps writing to a session torn down because writes do not complete")
	assert.Equal(t, uint32(1), p.crc.Load(),
		"RFC9687-4.3-6: Event 29 increments the ConnectRetryCounter by 1; a session "+
			"lost to a blocked socket is an attempt that failed")
	assert.Equal(t, fsm.StateIdle, p.session.State(),
		"RFC9687-4.3-7: Event 29 changes the state to Idle, which is what withdraws "+
			"the routes learned over this connection")
	assert.Contains(t, p.log.String(), "send hold timer expired",
		"RFC9687-4.3-2 and RFC9687-5-2: an error MUST be logged indicating the "+
			"\"Send Hold Timer Expired\" Error Code. RFC 9687 Section 7 makes the local "+
			"log the ONLY reliable record: the peer usually cannot be told, because the "+
			"stuck write is the one that would carry the NOTIFICATION")

	// RFC9687-5-1: "the BGP connection MUST be closed". Read after Run returned,
	// so the drain's terminating error is the close ze performed.
	requireConnClosed(t, p.drainErr)

	// Section 4.3 puts the optional NOTIFICATION first in the action list and
	// the connection drop fifth, so a peer that CAN still read must have been
	// told before the socket went away. net.Pipe never blocks the write, which
	// is the case Section 4.3 conditions the NOTIFICATION on.
	sent := collectWire(p.wire)
	var sawSendHoldExpired bool
	for off := 0; off+message.HeaderLen+2 <= len(sent); {
		hdr, err := message.ParseHeader(sent[off : off+message.HeaderLen])
		if err != nil || hdr.Length < message.HeaderLen {
			break
		}
		if hdr.Type == msgtype.TypeNOTIFICATION &&
			sent[off+message.HeaderLen] == uint8(message.NotifySendHoldTimerExpired) {
			assert.Equal(t, uint8(0), sent[off+message.HeaderLen+1],
				"RFC 9687 Section 6: the subcode for \"Send Hold Timer Expired\" is set to 0")
			assert.Equal(t, uint16(message.HeaderLen+2), hdr.Length,
				"RFC 9687 Section 6: no additional data is appended to a "+
					"\"Send Hold Timer Expired\" NOTIFICATION message")
			sawSendHoldExpired = true
		}
		off += int(hdr.Length)
	}
	assert.True(t, sawSendHoldExpired,
		"RFC 9687 Section 4.3 lists the Error Code 8 NOTIFICATION before it drops the "+
			"TCP connection, and this peer's socket accepts writes, so the attempt "+
			"Section 7 asks for must have reached it")
}

// TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact is the negative polarity for
// the whole Event 29 action list at once: none of it may run until the Send
// Hold Timer has actually expired.
//
// VALIDATES: RFC 9687 Section 4.3 -- the action list is bound to
// SendHoldTimer_Expires. An Established session inside its SendHoldTime keeps
// its timers, its socket, its retry count and its state.
//
// PREVENTS: reading the positive test as satisfied by an implementation that
// tears sessions down unconditionally. Every assertion below is the exact
// inverse of one above, on a session that differs only in whether the Send Hold
// Timer has expired.
//
// RFC requirement: RFC9687-4.3-2 negative -- the error is logged BY the expiry.
// RFC requirement: RFC9687-4.3-3 negative -- BGP resources are released BY the
// expiry, so the KeepaliveTimer keeps running on a live session.
// RFC requirement: RFC9687-4.3-4 negative -- an armed ConnectRetryTimer
// survives a session that has not expired.
// RFC requirement: RFC9687-4.3-5 negative -- the TCP connection is dropped BY
// the expiry, not by establishment.
// RFC requirement: RFC9687-4.3-6 negative -- the ConnectRetryCounter counts the
// expiry, not the establishment.
// RFC requirement: RFC9687-4.3-7 negative -- the state changes to Idle BY the
// expiry; before it the session is Established.
// RFC requirement: RFC9687-4.3-10 negative -- the Send Hold Timer is stopped BY
// the transition out of Established, so it stays armed while the session is up.
// RFC requirement: RFC9687-5-1 negative -- the connection is closed BY the
// expiry.
// RFC requirement: RFC9687-5-2 negative -- nothing is logged about a Send Hold
// Timer that has not expired.
func TestRFC9687NoSendHoldExpiryLeavesTheSessionIntact(t *testing.T) {
	p := rfc9687Established(t, 90)

	// One nanosecond short of the SendHoldTime: the timer is armed, the local
	// system has written nothing, and Event 29 has NOT fired.
	p.clock.Add(rfc9687SendHold - time.Nanosecond)

	select {
	case err := <-p.runResult:
		t.Fatalf("Run returned (%v) inside the SendHoldTime: nothing may tear an "+
			"Established session down before the Send Hold Timer expires", err)
	case <-time.After(250 * time.Millisecond):
	}

	assert.True(t, p.armed(),
		"RFC9687-4.3-10 negative: the Send Hold Timer is stopped by the transition "+
			"out of Established, so it must still be armed here")
	assert.True(t, p.session.timers.IsConnectRetryTimerRunning(),
		"RFC9687-4.3-4 negative: the ConnectRetryTimer is zeroed by Event 29")
	assert.True(t, p.session.timers.IsKeepaliveTimerRunning(),
		"RFC9687-4.3-3 negative: BGP resources are released by Event 29, so the "+
			"KeepaliveTimer must still be running on a live session")
	assert.Equal(t, uint32(0), p.crc.Load(),
		"RFC9687-4.3-6 negative: the ConnectRetryCounter counts the expiry, and this "+
			"session has not had one")
	assert.Equal(t, fsm.StateEstablished, p.session.State(),
		"RFC9687-4.3-7 negative: the state changes to Idle on Event 29, not before")
	assert.NotContains(t, p.log.String(), "send hold timer expired",
		"RFC9687-4.3-2 and RFC9687-5-2 negative: the error names an expiry that has "+
			"not happened; logging it here would tell an operator a healthy session "+
			"cannot transmit")

	select {
	case err := <-p.drainErr:
		t.Fatalf("the connection closed (%v) inside the SendHoldTime: RFC9687-5-1 "+
			"closes it on the expiry, not on establishment", err)
	default:
	}
}

// TestRFC9687SendHoldTimerArmedOnEstablished covers the OpenConfirm half of the
// Section 4.3 changes: the timer starts on the KEEPALIVE that makes the session
// Established, which is the first moment the local system owes the peer traffic.
//
// VALIDATES: RFC 9687 Section 4.3, the revised OpenConfirm KeepAliveMsg (Event
// 26) action list: "starts the SendHoldTimer if the SendHoldTime is non-zero".
//
// PREVENTS: a Send Hold Timer that is declared but never armed, which would
// leave every assertion in this file passing over a session that can never
// reach Event 29.
//
// RFC requirement: RFC9687-4.3-1 positive -- handleKeepalive calls
// startSendHoldTimer on the OpenConfirm transition.
func TestRFC9687SendHoldTimerArmedOnEstablished(t *testing.T) {
	p := rfc9687Established(t, 90)

	assert.True(t, p.armed(),
		"RFC9687-4.3-1: Event 26 in OpenConfirm starts the SendHoldTimer, so an "+
			"Established session must carry an armed one")
}

// TestRFC9687SendHoldTimerNotArmedBeforeEstablished is the negative polarity:
// the timer belongs to Event 26, not to the connection.
//
// VALIDATES: RFC 9687 Section 4.3 -- Section 4.1 says SendHoldTime governs "how
// long a BGP speaker will stay in the Established state", so a session that has
// not reached Established has nothing to measure.
//
// PREVENTS: a timer armed at Accept or at OPEN. The OPEN exchange is bounded by
// RFC 4271's own OpenSent hold time of 4 minutes, and a Send Hold Timer running
// beside it would tear down handshakes on a second, unrelated clock.
//
// RFC requirement: RFC9687-4.3-1 negative -- OpenConfirm is reached by the peer
// OPEN and left by the peer KEEPALIVE; only the second arms the timer.
func TestRFC9687SendHoldTimerNotArmedBeforeEstablished(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.SendHoldTime = rfc9687SendHold
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	fc := sim.NewFakeClock(time.Now())
	session.SetClock(fc)
	require.NoError(t, session.Start())

	server, client := net.Pipe()
	_ = acceptWithReader(t, session, server, client)
	startDrain(t, client)

	runResult := make(chan error, 1)
	go func() { runResult <- session.Run(t.Context()) }()
	t.Cleanup(func() { _ = client.Close() })

	go func() { _, _ = client.Write(rfc9687PeerOpen(90)) }()
	require.Eventually(t, func() bool { return session.State() == fsm.StateOpenConfirm },
		runExitDeadline, 5*time.Millisecond,
		"precondition: ze must reach OpenConfirm, one KEEPALIVE short of Established")

	assert.Zero(t, session.sendHoldDeadline.Load(),
		"RFC9687-4.3-1 negative: the SendHoldTimer starts on the OpenConfirm "+
			"KeepAliveMsg (Event 26), so a session still IN OpenConfirm must not "+
			"carry an armed one")
}

// TestRFC9687SendRestartsTheSendHoldTimer proves the restart is driven by the
// local system SENDING, which is the only thing that shows the socket drains.
//
// VALIDATES: RFC 9687 Section 4.3 -- "Each time the local system sends a BGP
// message, it restarts the SendHoldTimer".
//
// PREVENTS: a timer that expires on a healthy session. Without the restart the
// SendHoldTime becomes a hard cap on session lifetime, and every peering resets
// on that period regardless of how well it is transmitting.
//
// RFC requirement: RFC9687-4.3-8 positive -- writeMessage and SendRawMessage
// call resetSendHoldTimer after the flush succeeds, and sendHoldTimerCheck
// reschedules to the deadline the reset stored.
func TestRFC9687SendRestartsTheSendHoldTimer(t *testing.T) {
	p := rfc9687Established(t, 90)

	// Six tenths of the way to the deadline, then a real send, then six tenths
	// again. The total advance exceeds the SendHoldTime; the gap between the
	// send and the end of it does not.
	p.clock.Add(6 * rfc9687SendHold / 10)
	require.NoError(t, p.session.SendRawMessage(uint8(msgtype.TypeKEEPALIVE), nil),
		"precondition: the send under test must succeed")
	p.clock.Add(6 * rfc9687SendHold / 10)

	select {
	case err := <-p.runResult:
		t.Fatalf("Run returned (%v) after a total advance of 1.2x the SendHoldTime, "+
			"with a BGP message sent in the middle: RFC 9687 Section 4.3 restarts the "+
			"SendHoldTimer on that send, so no expiry may occur", err)
	case <-time.After(250 * time.Millisecond):
	}

	assert.True(t, p.armed(),
		"RFC9687-4.3-8: the send restarts the timer rather than stopping it")
	assert.Equal(t, fsm.StateEstablished, p.session.State(),
		"RFC9687-4.3-8: a session that is transmitting stays Established")
}

// TestRFC9687SilenceDoesNotRestartTheSendHoldTimer is the negative polarity: the
// restart is bound to a message actually being SENT, not to time passing.
//
// VALIDATES: RFC 9687 Section 4.3 -- the same clock advance that leaves a
// transmitting session up must tear down a silent one, or the restart is not
// what kept the first one alive.
//
// PREVENTS: reading the positive test as satisfied by a timer nothing ever
// fires. This test differs from it in exactly one respect: no send.
//
// RFC requirement: RFC9687-4.3-8 negative -- with no send, the deadline
// resetSendHoldTimer would have pushed forward stays where
// startSendHoldTimer put it, and the expiry runs.
func TestRFC9687SilenceDoesNotRestartTheSendHoldTimer(t *testing.T) {
	p := rfc9687Established(t, 90)

	p.clock.Add(6 * rfc9687SendHold / 10)
	p.clock.Add(6 * rfc9687SendHold / 10)

	select {
	case <-p.runResult:
	case <-time.After(runExitDeadline):
		t.Fatal("RFC9687-4.3-8 negative: 1.2x the SendHoldTime passed with nothing " +
			"sent, so the SendHoldTimer was never restarted and Event 29 must have " +
			"torn the session down")
	}

	assert.Equal(t, fsm.StateIdle, p.session.State(),
		"RFC9687-4.3-8 negative: only a send restarts the timer, and this session "+
			"made none")
}

// TestRFC9687ZeroNegotiatedHoldTimeStopsTheSendHoldTimer covers the exception
// Section 4.3 attaches to the restart clause.
//
// VALIDATES: RFC 9687 Section 4.3 -- the SendHoldTimer is stopped when "the
// negotiated HoldTime value is zero".
//
// PREVENTS: ze dropping a session that is behaving exactly as configured. RFC
// 4271 Section 4.2 lets a speaker propose a Hold Time of zero, which stops
// KEEPALIVEs in both directions, so an idle session writes nothing at all. A
// Send Hold Timer left armed there expires on its own schedule and resets the
// peering, and it does so every eight minutes under ze's automatic
// SendHoldTime.
//
// RFC requirement: RFC9687-4.3-9 positive -- startSendHoldTimer reads
// Timers.HoldTime, the negotiated min(local, peer), and stops rather than arms
// when it is zero.
func TestRFC9687ZeroNegotiatedHoldTimeStopsTheSendHoldTimer(t *testing.T) {
	p := rfc9687Established(t, 0)

	require.Equal(t, time.Duration(0), p.session.timers.HoldTime(),
		"precondition: a peer proposing zero negotiates a zero HoldTime")
	assert.False(t, p.armed(),
		"RFC9687-4.3-9: a zero negotiated HoldTime stops the SendHoldTimer")

	// Ten times the SendHoldTime, on a session that sends nothing. A timer left
	// armed anywhere in that window tears the session down.
	p.clock.Add(10 * rfc9687SendHold)

	select {
	case err := <-p.runResult:
		t.Fatalf("Run returned (%v) on a session whose negotiated HoldTime is zero: "+
			"RFC 9687 Section 4.3 stops the SendHoldTimer for exactly this case, and "+
			"RFC 4271 Section 4.2 makes a zero Hold Time a legal configuration that "+
			"sends no KEEPALIVEs at all", err)
	case <-time.After(250 * time.Millisecond):
	}
	assert.Equal(t, fsm.StateEstablished, p.session.State(),
		"RFC9687-4.3-9: the session stays up, because nothing measures its silence")
}

// TestRFC9687NonZeroNegotiatedHoldTimeArmsTheSendHoldTimer is the negative
// polarity: the stop is bound to the negotiated HoldTime being ZERO, and the
// timer runs for every other value.
//
// VALIDATES: RFC 9687 Section 4.3 -- the exception is narrow. A guard that read
// any hold time as a reason to stop would disable the mechanism entirely.
//
// PREVENTS: the fix for the zero case turning into a disabled feature. This
// session differs from the one above in the peer's proposed Hold Time and in
// nothing else.
//
// RFC requirement: RFC9687-4.3-9 negative -- a negotiated HoldTime of 90
// seconds arms the timer, and it expires.
func TestRFC9687NonZeroNegotiatedHoldTimeArmsTheSendHoldTimer(t *testing.T) {
	p := rfc9687Established(t, 90)

	require.Equal(t, 90*time.Second, p.session.timers.HoldTime(),
		"precondition: the negotiated HoldTime is min(90, 90)")
	assert.True(t, p.armed(),
		"RFC9687-4.3-9 negative: only a ZERO negotiated HoldTime stops the timer")

	p.clock.Add(rfc9687SendHold)
	select {
	case <-p.runResult:
	case <-time.After(runExitDeadline):
		t.Fatal("RFC9687-4.3-9 negative: with a non-zero negotiated HoldTime the " +
			"SendHoldTimer runs, so a silent local system must reach Event 29")
	}
}

// TestRFC9687TeardownStopsTheSendHoldTimer covers the last Section 4.3 clause
// on a transition out of Established that is NOT the Send Hold Timer's own
// expiry.
//
// VALIDATES: RFC 9687 Section 4.3 -- "The SendHoldTimer is stopped following
// any transition out of the Established state as part of the 'release all BGP
// resources' action."
//
// PREVENTS: a timer surviving a Cease. Its callback reads the session's
// connection and error channel, so one left running after an operator teardown
// fires against a session that is already gone.
//
// RFC requirement: RFC9687-4.3-10 positive -- closeConn calls
// stopSendHoldTimer before it takes the session lock, so every teardown path
// that closes the connection clears the timer.
func TestRFC9687TeardownStopsTheSendHoldTimer(t *testing.T) {
	p := rfc9687Established(t, 90)
	require.True(t, p.armed(), "precondition: the Send Hold Timer must be armed")

	require.NoError(t, p.session.CloseWithNotification(message.NotifyCease, 0))

	assert.False(t, p.armed(),
		"RFC9687-4.3-10: a transition out of Established releases all BGP resources, "+
			"and the SendHoldTimer is one of them")
}

// TestRFC9687SendHoldTimeMustExceedHoldTime covers the Section 4.4 constraint on
// the FSM attribute itself, at the one place an operator sets it.
//
// VALIDATES: RFC 9687 Section 4.4 -- "If SendHoldTime is non-zero, then it MUST
// be greater than the value of HoldTime". HoldTime is the negotiated value,
// min(local, peer), so a send-hold-time above the LOCAL receive-hold-time is
// above the negotiated value for every peer the configuration can reach.
//
// PREVENTS: a configuration in which the send side times out before the receive
// side. ze's YANG floors send-hold-time at 480 seconds and lets
// receive-hold-time reach 65535, so the pair below is reachable and was
// accepted until this check existed: two peers proposing 3600 negotiate 3600,
// and a 480-second SendHoldTimer then expires eight times inside one hold time,
// tearing down a session whose peer is answering.
//
// RFC requirement: RFC9687-4.4-1 positive -- parsePeerFromTree refuses the
// pairing rather than storing an attribute Section 4.4 forbids.
func TestRFC9687SendHoldTimeMustExceedHoldTime(t *testing.T) {
	cases := []struct {
		name        string
		receiveHold string
		sendHold    string
	}{
		{"send_hold_below_hold_time", "3600", "480"},
		{"send_hold_equals_hold_time", "600", "600"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}, "local": map[string]any{"ip": "auto"}},
				"session":    map[string]any{"asn": map[string]any{"remote": "65001"}},
				"timer":      map[string]any{"receive-hold-time": tc.receiveHold, "send-hold-time": tc.sendHold},
			}
			_, err := parsePeerFromTree("peer1", tree, 65000, 0)
			require.Error(t, err,
				"RFC 9687 Section 4.4 forbids a SendHoldTime that is not greater than "+
					"the HoldTime, so the configuration must be refused rather than "+
					"loaded into an attribute the RFC does not permit")
			assert.Contains(t, err.Error(), "invalid send-hold-time")
		})
	}
}

// TestRFC9687SendHoldTimeAboveHoldTimeAccepted is the negative polarity: the
// Section 4.4 guard must not over-fire.
//
// VALIDATES: RFC 9687 Section 4.4 -- the constraint is "greater than", and
// Section 6 recommends a default of the greater of 8 minutes or twice the
// negotiated HoldTime, which every case below satisfies.
//
// PREVENTS: a guard that refuses every send-hold-time, which would pass the
// positive test above while making the feature unconfigurable. The zero case is
// here because Section 4.4 conditions the constraint on a NON-zero
// SendHoldTime, and ze reads zero as the automatic duration.
//
// RFC requirement: RFC9687-4.4-1 negative -- a send-hold-time greater than the
// receive-hold-time, and the zero the constraint exempts, both parse.
func TestRFC9687SendHoldTimeAboveHoldTimeAccepted(t *testing.T) {
	cases := []struct {
		name        string
		receiveHold string
		sendHold    string
		want        time.Duration
	}{
		{"auto_is_exempt", "3600", "0", 0},
		{"one_second_above", "3600", "3601", 3601 * time.Second},
		{"default_hold_time", "90", "480", 480 * time.Second},
		{"zero_hold_time", "0", "480", 480 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}, "local": map[string]any{"ip": "auto"}},
				"session":    map[string]any{"asn": map[string]any{"remote": "65001"}},
				"timer":      map[string]any{"receive-hold-time": tc.receiveHold, "send-hold-time": tc.sendHold},
			}
			ps, err := parsePeerFromTree("peer1", tree, 65000, 0)
			require.NoError(t, err,
				"RFC 9687 Section 4.4 constrains a non-zero SendHoldTime to be GREATER "+
					"than the HoldTime; this pairing satisfies it and must load")
			assert.Equal(t, tc.want, ps.SendHoldTime)
		})
	}
}
