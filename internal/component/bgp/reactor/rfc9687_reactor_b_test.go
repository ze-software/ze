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
	"github.com/ze-software/ze/internal/test/sim"
)

// rfc9687EstablishedWithClient is rfc9687Established with the peer's end of
// the net.Pipe returned, so a test can act as the peer: send a NOTIFICATION or
// close the transport. The peer's hold time is 90 seconds.
func rfc9687EstablishedWithClient(t *testing.T) (*rfc9687Peer, net.Conn) {
	t.Helper()
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
	wire, drainErr := startDrain(t, client)
	runResult := rfc9687Run(t, session)

	go func() { _, _ = client.Write(rfc9687PeerOpen(90)) }()
	require.Eventually(t, func() bool { return session.State() == fsm.StateOpenConfirm },
		runExitDeadline, 5*time.Millisecond, "precondition: the peer OPEN was not accepted")
	go func() { _, _ = client.Write(message.PackTo(message.NewKeepalive(), nil)) }()
	require.Eventually(t, func() bool { return session.State() == fsm.StateEstablished },
		runExitDeadline, 5*time.Millisecond, "precondition: the session never reached Established")

	return &rfc9687Peer{session: session, clock: fc, runResult: runResult, drainErr: drainErr, wire: wire}, client
}

// rfc9687Sender is one way ze writes a BGP message on an Established session.
type rfc9687Sender struct {
	name string
	send func(s *Session) error
}

// rfc9687Senders returns the session's message writers other than
// SendRawMessage, which TestRFC9687SendRestartsTheSendHoldTimer drives: the
// generic writer that carries KEEPALIVE, NOTIFICATION and OPEN, the UPDATE
// writer, the withdraw writer and the raw UPDATE body writer.
func rfc9687Senders() []rfc9687Sender {
	return []rfc9687Sender{
		{name: "writeMessage-keepalive", send: func(s *Session) error {
			s.mu.Lock()
			conn := s.conn
			s.mu.Unlock()
			return s.writeMessage(conn, message.NewKeepalive())
		}},
		{name: "SendUpdate-withdraw", send: func(s *Session) error {
			return s.SendUpdate(&message.Update{WithdrawnRoutes: []byte{24, 10, 0, 0}})
		}},
		{name: "sendWithdraw", send: func(s *Session) error {
			return s.sendWithdraw(netip.MustParsePrefix("10.0.0.0/24"), false)
		}},
		{name: "sendRawUpdateBody", send: func(s *Session) error {
			return s.sendRawUpdateBody([]byte{0, 4, 24, 10, 0, 0, 0, 0})
		}},
	}
}

// Goal: prove every message writer of the session restarts the SendHoldTimer,
// not only SendRawMessage.
// Method: for each writer, advance the fake clock six tenths of the
// SendHoldTime, send one message through that writer, advance six tenths
// again, and check the session is still up with the timer armed. The silent
// session of TestRFC9687SilenceDoesNotRestartTheSendHoldTimer is torn down by
// the same advance.
//
// VALIDATES: RFC 9687 Section 4.3, the restart on each sent BGP message.
// PREVENTS: a send path that bypasses resetSendHoldTimer, which tears down a
// session that is transmitting.
//
// RFC requirement: RFC9687-4.3-8 positive -- a KEEPALIVE through writeMessage, an UPDATE through SendUpdate, a withdrawal through sendWithdraw and an UPDATE body through sendRawUpdateBody each restart the SendHoldTimer: after 1.2x the SendHoldTime with the send in the middle, Run has not returned, the timer is armed and the session is Established.
func TestRFC9687EveryWriterRestartsTheSendHoldTimer(t *testing.T) {
	for _, sender := range rfc9687Senders() {
		t.Run(sender.name, func(t *testing.T) {
			p := rfc9687Established(t, 90)
			p.clock.Add(6 * rfc9687SendHold / 10)
			require.NoError(t, sender.send(p.session), "precondition: the send under test must succeed")
			p.clock.Add(6 * rfc9687SendHold / 10)

			select {
			case err := <-p.runResult:
				t.Fatalf("Run returned (%v): the %s send did not restart the SendHoldTimer", err, sender.name)
			case <-time.After(250 * time.Millisecond):
			}
			require.True(t, p.armed())
			require.Equal(t, fsm.StateEstablished, p.session.State())
		})
	}
}

// Goal: prove the SendHoldTimer is stopped on transitions out of Established
// that neither its own expiry nor a local Cease drives.
// Method: from Established, the peer sends a NOTIFICATION, or the peer closes
// the transport; wait for Run to return and read the timer.
//
// VALIDATES: RFC 9687 Section 4.3, the SendHoldTimer stopped following any
// transition out of Established.
// PREVENTS: a teardown path that skips the release and leaves the timer armed.
//
// RFC requirement: RFC9687-4.3-10 positive -- after a NOTIFICATION received from the peer, and after the peer closes the TCP connection, Run returns, the session leaves Established, and the SendHoldTimer is stopped.
func TestRFC9687PeerDrivenTeardownStopsTheSendHoldTimer(t *testing.T) {
	cases := []struct {
		name     string
		teardown func(client net.Conn)
	}{
		{name: "notification-received", teardown: func(client net.Conn) {
			notification := message.PackTo(&message.Notification{ErrorCode: message.NotifyCease, ErrorSubcode: 2}, nil)
			go func() { _, _ = client.Write(notification) }()
		}},
		{name: "tcp-closed", teardown: func(client net.Conn) {
			require.NoError(t, client.Close())
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, client := rfc9687EstablishedWithClient(t)
			require.True(t, p.armed(), "precondition: the Send Hold Timer must be armed")
			tc.teardown(client)
			select {
			case <-p.runResult:
			case <-time.After(runExitDeadline):
				t.Fatal("the session did not leave Established")
			}
			require.NotEqual(t, fsm.StateEstablished, p.session.State())
			require.False(t, p.armed(), "the transition out of Established stops the SendHoldTimer")
		})
	}
}

// rfc9687SessionResources reports which of the session's BGP resources are
// still held: the HoldTimer, the KeepaliveTimer and the TCP connection.
func rfc9687SessionResources(s *Session) (hold, keepalive, conn bool) {
	s.mu.RLock()
	conn = s.conn != nil
	s.mu.RUnlock()
	return s.timers.IsHoldTimerRunning(), s.timers.IsKeepaliveTimerRunning(), conn
}

// Goal: prove Event 29 releases the session's BGP resources beyond the
// KeepaliveTimer: the HoldTimer and the TCP connection are released too.
// Method: drive two Established sessions; expire the Send Hold Timer on one and
// stop one nanosecond short on the other, then read the three resources.
//
// VALIDATES: RFC 9687 Section 4.3, Event 29 "releases all BGP resources" the
// session holds.
// PREVENTS: a teardown that stops the KeepaliveTimer and leaves the HoldTimer
// armed against a dead session, or keeps the socket.
//
// RFC requirement: RFC9687-4.3-3 positive -- after SendHoldTimer_Expires the session's HoldTimer and KeepaliveTimer are stopped and it holds no TCP connection.
// RFC requirement: RFC9687-4.3-3 negative -- one nanosecond short of the SendHoldTime the same session still runs its HoldTimer and KeepaliveTimer and holds its TCP connection.
func TestRFC9687Event29ReleasesHoldTimerAndConnection(t *testing.T) {
	live := rfc9687Established(t, 90)
	live.clock.Add(rfc9687SendHold - time.Nanosecond)
	select {
	case err := <-live.runResult:
		t.Fatalf("Run returned (%v) inside the SendHoldTime", err)
	case <-time.After(250 * time.Millisecond):
	}
	hold, keepalive, conn := rfc9687SessionResources(live.session)
	require.True(t, hold, "a live session keeps its HoldTimer")
	require.True(t, keepalive, "a live session keeps its KeepaliveTimer")
	require.True(t, conn, "a live session keeps its TCP connection")

	expired := rfc9687Established(t, 90)
	expired.clock.Add(rfc9687SendHold)
	select {
	case <-expired.runResult:
	case <-time.After(runExitDeadline):
		t.Fatal("the Send Hold Timer expired and Run never returned")
	}
	hold, keepalive, conn = rfc9687SessionResources(expired.session)
	require.False(t, hold, "Event 29 releases the HoldTimer")
	require.False(t, keepalive, "Event 29 releases the KeepaliveTimer")
	require.False(t, conn, "Event 29 releases the TCP connection")
}
