// Design: docs/architecture/behavior/fsm-open-sent.md -- OPEN errors reach the wire
// Related: rfc4271_session_core4271_test.go -- core4271ReadMessage
// Related: rfc4271_collision_open_receipt_test.go -- second-connection controls
// RFC: rfc/short/rfc4271.md -- Section 6.2, including outer parameter framing

package reactor

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/clock"
)

// OPEN framing errors reach the session decoder with a valid message-header
// length. A nine-octet body instead belongs to Section 6.1's header-length
// error, so it is deliberately not used as a Section 6.2 witness here.
// RFC requirement: RFC4271-6.2-3 positive -- standard and RFC 9072 outer lengths
// that exceed the received OPEN body produce NOTIFICATION 2/0 and close the
// ordinary session, rather than returning a decode error without a reply.
func TestRFC4271OuterOPENFramingErrorsNotify(t *testing.T) {
	for _, tc := range rfc4271ProofMalformedOPENs() {
		t.Run(tc.name, func(t *testing.T) {
			session, reply, err := rfc4271ProofOPENExchange(t, tc.body)
			require.ErrorIs(t, err, message.ErrShortRead)
			require.Equal(t, byte(msgtype.TypeNOTIFICATION), reply[18])
			require.Equal(t, []byte{2, 0}, reply[message.HeaderLen:])
			require.Equal(t, fsm.StateIdle, session.State())
		})
	}
}

// The exact-length twins prove the receiver does useful work, rather than
// rejecting every standard or extended OPEN at the error boundary.
// RFC requirement: RFC4271-6.2-3 negative -- correctly framed standard and
// RFC 9072 extended OPEN bodies reach OpenConfirm and write KEEPALIVE, not an
// OPEN Message Error NOTIFICATION.
func TestRFC4271ExactOuterOPENFramingCompletesTheHandshake(t *testing.T) {
	standard := validOpenBody()
	extended := validOpenBody()[:9]
	extended = append(extended, 255, 255, 0, 0)
	for _, body := range [][]byte{standard, extended} {
		session, reply, err := rfc4271ProofOPENExchange(t, body)
		require.NoError(t, err)
		require.Equal(t, byte(msgtype.TypeKEEPALIVE), reply[18])
		require.Len(t, reply, message.HeaderLen)
		require.Equal(t, fsm.StateOpenConfirm, session.State())
	}
}

// A pending connection parses the same OPEN before collision resolution. A
// malformed body is an OPEN error; it is not a successful collision loser.
// RFC requirement: RFC4271-6.2-3 positive -- an outer OPEN decode failure on
// the second connection produces OPEN Message Error 2/0 on that connection,
// while the existing OpenConfirm connection remains in place.
func TestRFC4271PendingOuterOPENFramingErrorsNotify(t *testing.T) {
	for _, tc := range rfc4271ProofMalformedOPENs() {
		t.Run(tc.name, func(t *testing.T) {
			session, existing, server := setupOpenConfirmSession(t, 0x0a000002)
			t.Cleanup(func() {
				session.timers.StopAll()
				require.NoError(t, existing.Close())
				require.NoError(t, server.Close())
			})
			peer := NewPeer(session.settings)
			peer.session = session
			client, pending := net.Pipe()
			require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
			require.NoError(t, peer.setPendingConnection(pending))
			r := &Reactor{clock: clock.RealClock{}}
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				r.handlePendingCollision(peer, pending)
			}()
			t.Cleanup(func() {
				require.NoError(t, client.Close())
				require.NoError(t, pending.Close())
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Error("pending OPEN reader did not stop")
				}
			})
			localOpen, openErr := core4271ReadMessage(client)
			require.NoError(t, openErr)
			require.Equal(t, byte(msgtype.TypeOPEN), localOpen[18], "the incoming connection sends its own OPEN before remote input")
			_, err := client.Write(rfc4271ProofOPENPacket(tc.body))
			require.NoError(t, err)
			reply, err := core4271ReadMessage(client)
			require.NoError(t, err)
			require.Equal(t, byte(msgtype.TypeNOTIFICATION), reply[18])
			require.Equal(t, []byte{2, 0}, reply[message.HeaderLen:])
			require.False(t, peer.hasPendingConnection())
			require.Equal(t, fsm.StateOpenConfirm, session.State())
		})
	}
}

// A valid collision loser keeps the collision-specific answer. Replacing all
// pending-connection refusals with OPEN errors would break this control.
// RFC requirement: RFC4271-6.2-3 negative -- a correctly framed second OPEN
// that loses the identifier comparison receives Cease 6/7, not an OPEN error,
// and leaves the existing OpenConfirm session intact.
func TestRFC4271ValidPendingOPENRemainsACollision(t *testing.T) {
	session, existing, server := setupOpenConfirmSession(t, 0x0a000002)
	t.Cleanup(func() {
		session.timers.StopAll()
		require.NoError(t, existing.Close())
		require.NoError(t, server.Close())
	})
	peer := NewPeer(session.settings)
	peer.session = session
	second := sendSecondOpen(t, peer, 0x0a000001)
	requireCollisionCease(t, second, "valid pending connection")
	require.Equal(t, fsm.StateOpenConfirm, session.State())
}

func rfc4271ProofMalformedOPENs() []struct {
	name string
	body []byte
} {
	standard := validOpenBody()[:9]
	standard = append(standard, 4, 2, 0)
	shortExtended := validOpenBody()[:9]
	shortExtended = append(shortExtended, 255, 255)
	extended := validOpenBody()[:9]
	extended = append(extended, 255, 255, 0, 4, 2, 0)
	return []struct {
		name string
		body []byte
	}{
		{"standard length overruns body", standard},
		{"extended length field truncated", shortExtended},
		{"extended length overruns body", extended},
	}
}

func rfc4271ProofOPENPacket(body []byte) []byte {
	packet := make([]byte, message.HeaderLen+len(body))
	for i := range 16 {
		packet[i] = 0xff
	}
	binary.BigEndian.PutUint16(packet[16:18], uint16(len(packet))) //nolint:gosec // bounded fixture bodies
	packet[18] = byte(msgtype.TypeOPEN)
	copy(packet[message.HeaderLen:], body)
	return packet
}

func rfc4271ProofOPENExchange(t *testing.T, body []byte) (*Session, []byte, error) {
	t.Helper()
	session, client := newOpenSentSessionWithClient(t)
	t.Cleanup(session.timers.StopAll)
	require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
	type response struct {
		packet []byte
		err    error
	}
	read := make(chan response, 1)
	go func() {
		_, err := client.Write(rfc4271ProofOPENPacket(body))
		if err != nil {
			read <- response{err: err}
			return
		}
		packet, readErr := core4271ReadMessage(client)
		read <- response{packet: packet, err: readErr}
	}()
	err := session.ReadAndProcess()
	reply := <-read
	require.NoError(t, reply.err)
	return session, reply.packet, err
}
