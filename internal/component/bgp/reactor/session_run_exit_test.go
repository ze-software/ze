package reactor

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// runExitDeadline bounds every wait in this file. The defects these tests cover
// are all "Run never returns" or "nothing ever closes the socket", so a blocked
// wait IS the failure: each select below must report it rather than hang the
// package until the go test timeout kills it with no attribution.
const runExitDeadline = 5 * time.Second

// pipeSession wires a Session onto a net.Pipe the way the Run-loop tests in
// session_test.go do (conn + bufReader + bufWriter set directly, no handshake),
// and starts a drain goroutine on the client end.
//
// The drain is not optional. net.Pipe is unbuffered and synchronous, so a
// NOTIFICATION written on an error path sits in bufWriter until closeConn
// flushes it, and that flush blocks forever if nobody is reading. Without a
// reader these tests would deadlock inside the code under test instead of
// asserting on it.
//
// Returns the raw bytes ze wrote (closed once the connection is gone) and the
// error that ended the drain: io.EOF / io.ErrClosedPipe means ze closed the
// connection, os.ErrDeadlineExceeded means it did not.
func pipeSession(t *testing.T, session *Session) (client net.Conn, wire <-chan []byte, drainErr <-chan error) {
	t.Helper()

	server, client := net.Pipe()
	session.mu.Lock()
	session.conn = server
	session.bufReader = bufio.NewReaderSize(server, 65536)
	session.bufWriter = bufio.NewWriterSize(server, 16384)
	session.mu.Unlock()

	wireCh, drainCh := startDrain(t, client)
	return client, wireCh, drainCh
}

// startDrain reads the client end until the pipe dies, publishing every chunk
// and then the terminating error.
func startDrain(t *testing.T, client net.Conn) (<-chan []byte, <-chan error) {
	t.Helper()

	wire := make(chan []byte, 32)
	drainErr := make(chan error, 1)
	go func() {
		defer close(wire)
		// Bound the drain itself: a read that never ends would leak this
		// goroutine past the test and hide the very stall under test.
		_ = client.SetReadDeadline(time.Now().Add(2 * runExitDeadline))
		buf := make([]byte, 65536)
		for {
			n, err := client.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				wire <- chunk
			}
			if err != nil {
				drainErr <- err
				return
			}
		}
	}()
	return wire, drainErr
}

// collectWire drains the capture channel until it closes, concatenating what ze
// wrote. Safe to call only after the connection is known to be closed.
func collectWire(wire <-chan []byte) []byte {
	var out []byte
	for chunk := range wire {
		out = append(out, chunk...)
	}
	return out
}

// requireConnClosed asserts ze closed its end of the connection. A deadline
// error means the socket was still open, which is the AC-7 defect exactly.
func requireConnClosed(t *testing.T, drainErr <-chan error) {
	t.Helper()
	select {
	case err := <-drainErr:
		require.Error(t, err, "AC-7: drain ended without an error, which cannot happen on a live pipe")
		require.NotErrorIs(t, err, os.ErrDeadlineExceeded,
			"AC-7: the connection was STILL OPEN after Run returned -- nothing closed it")
		require.True(t, errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe),
			"AC-7: expected the connection to be closed, got %v", err)
	case <-time.After(runExitDeadline):
		t.Fatal("AC-7: the connection was still open " + runExitDeadline.String() +
			" after Run returned -- nothing closed it")
	}
}

// eorUpdate is the smallest legal UPDATE: no withdrawn routes, no path
// attributes, no NLRI (RFC 4724 End-of-RIB). It reaches the onMessageReceived
// callback without needing a negotiated family, which is what lets these tests
// exercise the post-callback teardown branch with no handshake.
func eorUpdate() []byte {
	msg := make([]byte, 23)
	for i := range 16 {
		msg[i] = 0xFF
	}
	msg[16], msg[17] = 0x00, 0x17 // length 23
	msg[18] = byte(msgtype.TypeUPDATE)
	return msg
}

// TestPolicyTeardownExitsRun proves the import-policy teardown actually ends the
// session.
//
// VALIDATES: AC-6 -- a policy filter's teardown request makes Run return promptly,
// carrying ErrPolicyTeardown, which peer_run.go classifies as BACKOFF (D-7, Q-4).
// PREVENTS: the regression where the teardown branch sent its NOTIFICATION, closed
// the connection and returned nil. closeConn nils s.conn, so Run's loop reached its
// conn == nil branch (session.go:901), found no close reason (:903) and slept 10 ms
// (:906) round the loop forever: the session was dead on the wire, the peer was
// still marked up, and its routes were never withdrawn.
func TestPolicyTeardownExitsRun(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session := NewSession(settings)

	client, wire, drainErr := pipeSession(t, session)
	defer func() { _ = client.Close() }()

	// Stand in for the import filter chain: reactor_notify.go:459-471 calls
	// requestPolicyTeardown from inside this same callback, on this same
	// goroutine, with Cease / Connection Rejected as its default code pair.
	var filterRan bool
	session.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		_ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string,
	) bool {
		filterRan = true
		session.requestPolicyTeardown(message.NotifyCease, message.NotifyCeaseConnectionRejected)
		return false
	}

	resultCh := make(chan error, 1)
	go func() { resultCh <- session.Run(t.Context()) }()
	go func() { _, _ = client.Write(eorUpdate()) }()

	select {
	case err := <-resultCh:
		require.ErrorIs(t, err, ErrPolicyTeardown,
			"AC-6: Run must return the policy-teardown sentinel")
		// D-7: the sentinel is distinct from ErrTeardown on purpose. peer_run.go:78
		// reconnects IMMEDIATELY on ErrTeardown, and the peer's config still violates
		// the filter, so that arm turns one rejected UPDATE into a NOTIFICATION storm.
		require.NotErrorIs(t, err, ErrTeardown,
			"D-7: policy teardown must take the backoff arm, not ErrTeardown's immediate reconnect")
	case <-time.After(runExitDeadline):
		t.Fatal("AC-6: Run did not return within " + runExitDeadline.String() +
			" of the policy teardown -- it is spinning on the conn == nil branch")
	}

	require.True(t, filterRan, "precondition: the filter callback never ran, so nothing requested a teardown")
	requireConnClosed(t, drainErr)

	// The NOTIFICATION is the peer-visible half of the obligation: RFC 4271
	// Section 6.7 requires a session torn down by local policy to say so.
	sent := collectWire(wire)
	require.GreaterOrEqual(t, len(sent), 21, "expected a NOTIFICATION on the wire, got %d bytes", len(sent))
	require.Equal(t, msgtype.TypeNOTIFICATION, msgtype.MessageType(sent[18]), "message type")
	require.Equal(t, uint8(message.NotifyCease), sent[19], "NOTIFICATION error code")
	require.Equal(t, message.NotifyCeaseConnectionRejected, sent[20], "NOTIFICATION error subcode")
}

// establishedOpenSent puts a session on a pipe through the REAL accept path, so
// s.localOpen is populated and the FSM is past Idle -- both preconditions for
// the OPEN-handling paths the AC-7 cases below drive.
func establishedOpenSent(t *testing.T) (*Session, net.Conn, <-chan []byte, <-chan error) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	require.NoError(t, session.Start())

	server, client := net.Pipe()
	_ = acceptWithReader(t, session, server, client)

	wire, drainErr := startDrain(t, client)
	return session, client, wire, drainErr
}

// peerOpenBytes is a well-formed peer OPEN matching the settings above.
func peerOpenBytes() []byte {
	return message.PackTo(&message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
		OptionalParams: []byte{
			0x02, 0x06, 0x41, 0x04, 0x00, 0x00, 0xFD, 0xEA, // ASN4
			0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01, // IPv4/Unicast
		},
	}, nil)
}

// TestRunClosesConnectionOnEveryExit covers the three OPEN-path returns that
// send a NOTIFICATION (or not) and then return with the socket still open.
//
// VALIDATES: AC-7 -- the TCP connection is closed by the time Run has returned,
// on every exit path (D-8: one defer in Run, not per-site closeConn calls).
// PREVENTS: half-open sockets surviving a rejected session. Each case below
// returned an error to Run without closing: the peer saw a NOTIFICATION on a
// connection that stayed up, and the fd leaked until the peer or the kernel
// timed it out.
func TestRunClosesConnectionOnEveryExit(t *testing.T) {
	cases := []struct {
		name string
		// site is the producing return this case drives.
		site string
		// arm mutates the session to force that return, and returns the bytes
		// the peer should send.
		arm func(t *testing.T, s *Session) []byte
	}{
		{
			name: "unpack OPEN error",
			site: "session_handlers.go:87-91",
			arm: func(_ *testing.T, _ *Session) []byte {
				// Legal header, legal OPEN minimum length (29), but the optional
				// parameter length claims 200 bytes that are not there, so
				// UnpackOpen fails and handleOpen returns without closing.
				msg := make([]byte, 29)
				for i := range 16 {
					msg[i] = 0xFF
				}
				msg[16], msg[17] = 0x00, 0x1D // length 29
				msg[18] = byte(msgtype.TypeOPEN)
				msg[19] = 4                   // version
				msg[20], msg[21] = 0xFD, 0xEA // my AS 65002
				msg[22], msg[23] = 0x00, 0x5A // hold time 90
				msg[24], msg[25] = 0x01, 0x02 // identifier 1.2.3.2
				msg[26], msg[27] = 0x03, 0x02 //
				msg[28] = 200                 // optional parameter length: a lie
				return msg
			},
		},
		{
			name: "openValidator rejection",
			site: "session_open_validation.go:115-116",
			arm: func(_ *testing.T, s *Session) []byte {
				// The RFC 9234 Role plugin rejects here in production: a
				// NOTIFICATION goes out at :115 and :116 returns, closing nothing.
				s.openValidator = func(_ string, _, _ *message.Open) error {
					return errors.New("open rejected by test validator")
				}
				return peerOpenBytes()
			},
		},
		{
			name: "local capability parse error",
			site: "session_handlers.go:150-153",
			arm: func(_ *testing.T, s *Session) []byte {
				// A truncated capability parameter in OUR OWN advertised OPEN.
				// Reaching this return in production means a ze bug, but the
				// return is real and it closed nothing; the exit discipline has
				// to cover it precisely because nobody predicted reaching it.
				s.mu.Lock()
				if s.localOpen != nil {
					s.localOpen.OptionalParams = []byte{0x02, 0x06, 0x41}
				}
				s.mu.Unlock()
				return peerOpenBytes()
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session, client, _, drainErr := establishedOpenSent(t)
			defer func() { _ = client.Close() }()

			toSend := tc.arm(t, session)

			resultCh := make(chan error, 1)
			go func() { resultCh <- session.Run(t.Context()) }()
			go func() { _, _ = client.Write(toSend) }()

			select {
			case err := <-resultCh:
				require.Error(t, err, "precondition: %s must return an error, or this case proves nothing", tc.site)
			case <-time.After(runExitDeadline):
				t.Fatal("precondition: Run did not return for " + tc.site)
			}

			requireConnClosed(t, drainErr)
		})
	}
}

// TestSessionRunStopsTimersOnValidationTeardown guards the defer that AC-3's
// code half already landed (session.go:846) but that no test held in place.
//
// VALIDATES: AC-3 -- when Run exits through a validation teardown, the keepalive
// timer is stopped, so the Session is not retained by a live timer closure.
// PREVENTS: the leak that motivated the single defer. The keepalive timer
// re-arms itself while keepaliveRunning (fsm/timer.go:367-405); an exit path
// that called closeConn but not StopAll left it firing on a dead session,
// its callback touching session state long after Run returned, and Peer.runOnce
// abandons the old Session each cycle without stopping anything.
func TestSessionRunStopsTimersOnValidationTeardown(t *testing.T) {
	cases := []struct {
		name string
		site string
		msg  func() []byte
	}{
		{
			name: "message length over maximum",
			site: "session_read.go:107-119",
			msg: func() []byte {
				// RFC 8654: 5000 > 4096 with the extended-message capability
				// not negotiated. Header only -- the length check fires before
				// the body is read, so no body is needed (or wanted: the peer
				// end is closed underneath us).
				msg := make([]byte, 19)
				for i := range 16 {
					msg[i] = 0xFF
				}
				msg[16], msg[17] = 0x13, 0x88 // length 5000
				msg[18] = byte(msgtype.TypeUPDATE)
				return msg
			},
		},
		{
			name: "unknown message type",
			site: "session_handlers.go:33",
			msg: func() []byte {
				msg := make([]byte, 19)
				for i := range 16 {
					msg[i] = 0xFF
				}
				msg[16], msg[17] = 0x00, 0x13 // length 19
				msg[18] = 0x7F                // not a BGP message type
				return msg
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
			session := NewSession(settings)

			client, _, _ := pipeSession(t, session)
			defer func() { _ = client.Close() }()

			// Arm the keepalive timer the way handleKeepalive does on the
			// OpenConfirm -> Established transition.
			session.timers.SetKeepaliveTime(30 * time.Second)
			session.timers.OnKeepaliveTimerExpires(func() {})
			session.timers.StartKeepaliveTimer()
			require.True(t, session.timers.IsKeepaliveTimerRunning(),
				"precondition: keepalive timer must be armed, or this case proves nothing")

			// The hold timer is armed once per connection by
			// connectionEstablished (session_connection.go:357-359); arm it here
			// too, so the assertion below covers StopAll and not just the
			// keepalive half of it.
			session.timers.SetHoldTime(90 * time.Second)
			session.timers.OnHoldTimerExpires(func() {})
			session.timers.StartHoldTimer()
			require.True(t, session.timers.IsHoldTimerRunning(),
				"precondition: hold timer must be armed, or this case proves nothing")

			resultCh := make(chan error, 1)
			go func() { resultCh <- session.Run(t.Context()) }()
			go func() { _, _ = client.Write(tc.msg()) }()

			select {
			case err := <-resultCh:
				require.Error(t, err, "precondition: %s must return an error", tc.site)
			case <-time.After(runExitDeadline):
				t.Fatal("precondition: Run did not return for " + tc.site)
			}

			require.False(t, session.timers.IsKeepaliveTimerRunning(),
				"AC-3: the keepalive timer is still armed after Run returned via %s -- "+
					"it will keep re-arming itself and keep the dead Session reachable", tc.site)
			require.False(t, session.timers.IsHoldTimerRunning(),
				"AC-3: the hold timer is still armed after Run returned via %s", tc.site)
		})
	}
}
