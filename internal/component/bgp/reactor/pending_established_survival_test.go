// Design: docs/architecture/behavior/peer-lifecycle.md -- pending connection isolation.
// Related: reactor_connection.go -- acceptOrReject and handlePendingCollision.
// Related: rfc7947_session_first_as_test.go -- real socket session and semantic UPDATE capture.
package reactor

import (
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestPendingFailurePreservesEstablishedSession proves useful traffic AFTER
// each pending failure, not just while the second OPEN is incomplete. The
// valid OPEN control must reach collision resolution instead of decode error.
// RFC 4271 Sections 6.2, 6.8 and 8.2.2 distinguish the connections: a malformed
// recognized optional parameter earns OPEN Error/Unspecific, a valid collision
// loser earns Cease, and neither retires the existing Established connection.
// These additive probes are not enrolled as whole-requirement RFC evidence.
func TestPendingFailurePreservesEstablishedSession(t *testing.T) {
	for _, test := range []struct {
		name         string
		kind         string
		notification []byte
	}{
		{"malformed optional parameter envelope", "malformed", []byte{2, 0}},
		{"premature EOF after partial OPEN", "eof", nil},
		{"expired pending read budget", "expired", nil},
		{"valid OPEN reaches collision resolution", "valid", []byte{6, 7}},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
			settings.ReceiveHoldTime = 90 * time.Second
			session, existing := firstASSession(t, settings, nil, nil)
			original := session.Conn()
			peer := NewPeer(settings)
			peer.session = session
			peer.setState(PeerStateEstablished)
			pendingSurvivalRoute(t, session, existing, []byte{24, 203, 0, 113})

			client, server := net.Pipe()
			pending := &pendingSurvivalSocket{Conn: server, closed: make(chan struct{}), deadline: make(chan time.Time, 1)}
			t.Cleanup(func() { _ = client.Close(); _ = pending.Close() })
			require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
			var clk clock.Clock = clock.RealClock{}
			if test.kind == "expired" {
				// The pending reader translates clock.Now plus ReceiveHoldTime
				// into a real socket deadline. This existing test technique makes
				// that deadline expire immediately without waiting ninety seconds.
				clk = sim.NewFakeClock(time.Now().Add(-91 * time.Second))
			}
			r := &Reactor{clock: clk}
			r.acceptOrReject(pending, peer, nil)
			var deadline time.Time
			select {
			case deadline = <-pending.deadline:
			case <-time.After(5 * time.Second):
				t.Fatal("the admitted pending connection never armed its OPEN read deadline")
			}
			if test.kind == "expired" {
				require.True(t, deadline.Before(time.Now()), "the actual pending socket read deadline must already be expired")
			} else {
				require.True(t, peer.hasPendingConnection(), "the second connection must be tracked until its OPEN completes or fails")
				packet := message.PackTo(&message.Open{Version: 4, MyAS: 65002, HoldTime: 0,
					BGPIdentifier: 0x02020302, OptionalParams: []byte{2, 0}}, nil)
				switch test.kind {
				case "malformed":
					// The complete BGP frame declares two optional-parameter
					// octets, but carries only the recognized capability type
					// (2), without its length. Header length matches this frame.
					packet = packet[:len(packet)-1]
					binary.BigEndian.PutUint16(packet[16:18], uint16(len(packet)))
				case "eof":
					packet = packet[:message.HeaderLen+3]
				}
				_, err := client.Write(packet)
				require.NoError(t, err)
				if test.kind == "eof" {
					require.NoError(t, client.Close())
				}
			}
			if len(test.notification) != 0 {
				reply, err := core4271ReadMessage(client)
				require.NoError(t, err)
				require.Equal(t, byte(msgtype.TypeNOTIFICATION), reply[18])
				require.Equal(t, test.notification, reply[message.HeaderLen:])
			}
			if test.kind != "eof" {
				var octet [1]byte
				n, err := client.Read(octet[:])
				require.Zero(t, n, "pending cleanup must send no extra bytes")
				require.ErrorIs(t, err, io.EOF, "the pending socket must close")
			}
			select {
			case <-pending.closed:
			case <-time.After(5 * time.Second):
				t.Fatal("the pending reader did not close its owned socket")
			}
			if len(test.notification) == 0 {
				require.Zero(t, pending.writes.Load(), "a pending transport failure must not attempt a BGP refusal")
			}
			require.False(t, peer.hasPendingConnection(), "failed or rejected pending connection must be released")
			require.Same(t, session, peer.currentSession())
			require.Same(t, original, session.Conn(), "pending cleanup must keep the original connection")
			require.Equal(t, fsm.StateEstablished, peer.SessionState())
			// This new prefix was not present before pending cleanup. Its
			// semantic delivery proves the original socket still does useful work.
			pendingSurvivalRoute(t, session, existing, []byte{24, 198, 51, 100})
		})
	}
}

func pendingSurvivalRoute(t *testing.T, session *Session, client net.Conn, prefix []byte) {
	t.Helper()
	update := firstASReceive(t, session, client, makeUpdateBody(nil, firstASAttrs(2, 65002), prefix))
	nlri, err := update.NLRI()
	require.NoError(t, err)
	require.Equal(t, prefix, nlri, "the original connection must deliver the new announcement")
	withdrawn, err := update.Withdrawn()
	require.NoError(t, err)
	require.Empty(t, withdrawn, "a rejected route cannot stand in for a useful announcement")
	require.Equal(t, fsm.StateEstablished, session.State())
}

// pendingSurvivalSocket delegates every socket operation. It observes the
// producer's actual deadline, write attempts and close without supplying input
// or inventing a successful operation. Close is also the EOF-case cleanup join.
type pendingSurvivalSocket struct {
	net.Conn
	closed    chan struct{}
	deadline  chan time.Time
	closeOnce sync.Once
	writes    atomic.Int64
}

func (conn *pendingSurvivalSocket) SetReadDeadline(deadline time.Time) error {
	err := conn.Conn.SetReadDeadline(deadline)
	if err == nil {
		select {
		case conn.deadline <- deadline:
		default:
		}
	}
	return err
}

func (conn *pendingSurvivalSocket) Write(data []byte) (int, error) {
	conn.writes.Add(1)
	return conn.Conn.Write(data)
}

func (conn *pendingSurvivalSocket) Close() error {
	err := conn.Conn.Close()
	conn.closeOnce.Do(func() { close(conn.closed) })
	return err
}
