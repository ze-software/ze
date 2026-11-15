// Design: docs/architecture/behavior/fsm.md — observe the transition before final cleanup
// Related: peer_history.go — immutable transition history produced by the real callback
// Related: rfc9687_test.go — the complete Send Hold expiry action list and its control
// RFC: rfc/short/rfc9687.md — Section 4.3 requires the expiry to enter Idle

package reactor

import (
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestRFC9687ExpiryFirstLeavesEstablishedForIdle reads the real Peer's recorded
// transition rather than its eventual State. It explicitly dispatches the
// production expiry handler on a session with a valid configured timer; it does
// not prove elapsed-timer detection. A wrong Established-to-Active action can
// later be repaired by Active-to-Idle transport cleanup, but that repair must
// not satisfy the required first transition. The production FSM callback stays
// installed throughout; its existing history observer supplies the fact.
// RFC requirement: RFC9687-4.3-7 positive -- when the production Send Hold expiry handler is explicitly dispatched on an Established session, its first transition enters Idle; later transport cleanup cannot repair a wrong first destination. This is a conditional action proof, not an elapsed-timer integration proof.
func TestRFC9687ExpiryFirstLeavesEstablishedForIdle(t *testing.T) {
	peer, session, conn := rfc9687TransitionPeer(t)
	observer := &Reactor{peers: map[netip.AddrPort]*Peer{peer.Settings().PeerKey(): peer}}

	// Closing our side after the peer's FIN lets its graceful drain finish.
	// The test owns this reader and joins it below; cleanup also closes conn.
	drained := make(chan error, 1)
	drainDone := make(chan struct{})
	t.Cleanup(func() {
		conn.Close() //nolint:errcheck // interrupt the reader even after a fatal assertion
		select {
		case <-drainDone:
		case <-time.After(runExitDeadline):
			t.Error("the test's socket reader did not stop during cleanup")
		}
	})
	go func() {
		defer close(drainDone)
		_, err := io.Copy(io.Discard, conn)
		conn.Close() //nolint:errcheck // the peer may already have closed this test socket
		drained <- err
	}()
	require.NotZero(t, session.sendHoldDeadline.Load(), "the real established session owes a Send Hold expiry")
	require.Equal(t, sendHoldTimerMin, session.sendHoldDuration(), "the configured timer meets the production minimum")
	require.Greater(t, session.sendHoldDuration(), session.timers.HoldTime(), "SendHoldTime exceeds negotiated HoldTime")
	session.sendHoldTimerExpired()

	var firstDestination string
	require.Eventually(t, func() bool {
		for _, transition := range observer.PeerFSMHistory(peer.Settings().Address.String()) {
			if transition.From == fsm.StateEstablished.String() {
				firstDestination = transition.To
				return true
			}
		}
		return false
	}, runExitDeadline, time.Millisecond, "the real expiry must publish its leave-Established transition")
	require.Equal(t, fsm.StateIdle.String(), firstDestination,
		"RFC9687-4.3-7: the expiry itself enters Idle, before any later transport cleanup")
	select {
	case err := <-drained:
		require.NoError(t, err, "the expired session closes its socket")
	case <-time.After(runExitDeadline):
		t.Fatal("the test's socket reader did not finish after expiry")
	}
}

// rfc9687TransitionPeer establishes a real dialed Peer with a valid SendHoldTime
// and its fake clock set before Start, retaining the normal callback, timer
// ownership and down path. Its caller dispatches the expiry action explicitly.
func rfc9687TransitionPeer(t *testing.T) (*Peer, *Session, net.Conn) {
	t.Helper()
	var config net.ListenConfig
	listener, err := config.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // test listener cleanup
	address, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)

	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
	settings.Port = uint16(address.Port) //nolint:gosec // an allocated TCP port fits two octets
	settings.Connection = ConnectionActive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.SendHoldTime = sendHoldTimerMin
	peer := NewPeer(settings)
	clock := sim.NewFakeClock(time.Now())
	peer.SetClock(clock)
	peer.setReconnectDelay(time.Minute, time.Minute)
	t.Cleanup(startAndStop(t, peer))

	tcpListener, ok := listener.(*net.TCPListener)
	require.True(t, ok)
	require.NoError(t, tcpListener.SetDeadline(time.Now().Add(runExitDeadline)))
	conn, err := listener.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // EOF also closes the socket reader
	rfc9687TransitionRead(t, conn, msgtype.TypeOPEN)

	_, err = conn.Write(wellFormedOpen())
	require.NoError(t, err)
	rfc9687TransitionRead(t, conn, msgtype.TypeKEEPALIVE)
	_, err = conn.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	// Finish initial routing work before the test dispatches the expiry action.
	body := rfc9687TransitionRead(t, conn, msgtype.TypeUPDATE)
	require.Equal(t, []byte{0, 0, 0, 0}, body, "the initial IPv4 EOR is the final startup write")
	var session *Session
	require.Eventually(t, func() bool {
		session = peer.currentSession()
		if session == nil {
			return false
		}
		if session.State() != fsm.StateEstablished {
			return false
		}
		return session.sendingInitialRoutes.Load() == 0
	}, runExitDeadline, time.Millisecond, "the real peer must finish initial synchronization")
	return peer, session, conn
}

// rfc9687TransitionRead consumes a complete bounded handshake frame. The peer's
// EOR is observed before the reader starts discarding shutdown output.
func rfc9687TransitionRead(t *testing.T, conn net.Conn, expected msgtype.MessageType) []byte {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(runExitDeadline)))
	var raw [message.HeaderLen]byte
	_, err := io.ReadFull(conn, raw[:])
	require.NoError(t, err)
	header, err := message.ParseHeader(raw[:])
	require.NoError(t, err)
	require.Equal(t, expected, header.Type)
	body := make([]byte, int(header.Length)-message.HeaderLen)
	_, err = io.ReadFull(conn, body)
	require.NoError(t, err)
	require.NoError(t, conn.SetReadDeadline(time.Time{}))
	return body
}
