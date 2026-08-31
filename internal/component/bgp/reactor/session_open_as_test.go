// VALIDATES: a received OPEN whose peer AS is zero is aborted with NOTIFICATION 2/2
// (OPEN Message Error / Bad Peer AS) on both OPEN rails, and ze never puts AS 0 in an
// OPEN it sends.
// PREVENTS: accepting a peering with the AS that RFC 6491 uses to mark a prefix as not
// routable, and originating one. It also pins the separation from ze's internal AS 0
// sentinel: a peer that advertises a real AS is accepted even when no AS is configured
// for it, which is the dynamic-peer case.

package reactor

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// openBodyWithASN4Capability builds an OPEN body whose My AS is AS_TRANS and whose
// Four-octet AS capability carries asn4. That is the shape RFC 6793 Section 3 gives a
// speaker with a four-octet AS, and it is the second field a peer AS of zero can hide in.
func openBodyWithASN4Capability(asn4 uint32) []byte {
	capValue := []byte{
		byte(capability.CodeASN4), 4,
		byte(asn4 >> 24), byte(asn4 >> 16), byte(asn4 >> 8), byte(asn4),
	}
	optParams := append([]byte{2, byte(len(capValue))}, capValue...)

	body := []byte{
		0x04,       // Version 4
		0x5B, 0xA0, // My AS = 23456 (AS_TRANS)
		0x00, 0x5A, // HoldTime = 90
		0x0A, 0x00, 0x00, 0x01, // BGP Identifier 10.0.0.1
		byte(len(optParams)),
	}
	return append(body, optParams...)
}

// TestHandleOpenRejectsPeerASZero drives handleOpen, the rail every non-colliding
// connection takes, over the two fields an OPEN can carry a peer AS in.
//
// RFC requirement: RFC7607-2-4 positive -- a My AS of zero, and an AS_TRANS My AS with a
// Four-octet AS capability of zero, are each aborted with NOTIFICATION 2/2 Bad Peer AS
// and neither advances the FSM.
// RFC requirement: RFC7607-2-4 negative -- a real AS in either field is accepted, the
// session advances to OpenConfirm, and no NOTIFICATION is sent. Without this arm the
// positive arm would also pass against code that refused every OPEN.
func TestHandleOpenRejectsPeerASZero(t *testing.T) {
	const localID uint32 = 0x0A000002

	tests := []struct {
		name       string
		body       []byte
		wantReject bool
	}{
		{"my-as zero", openBodyWithIdentifier(0, 0x0A000001), true},
		{"as-trans with four-octet AS zero", openBodyWithASN4Capability(0), true},
		{"my-as is a real AS", openBodyWithIdentifier(65002, 0x0A000001), false},
		{"as-trans with a real four-octet AS", openBodyWithASN4Capability(196608), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, written := openSentSessionAS(t, 65002, localID)

			err := session.handleOpen(tt.body)

			if !tt.wantReject {
				require.NoError(t, err, "a real peer AS must not be rejected")
				assert.Equal(t, fsm.StateOpenConfirm, session.State(), "the session advances")
				code, subcode, found := notificationFrom(t, written)
				assert.False(t, found,
					"an accepted OPEN must draw no NOTIFICATION, got %d/%d", code, subcode)
				return
			}

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrBadPeerAS)
			assert.NotEqual(t, fsm.StateOpenConfirm, session.State(),
				"an aborted connection must not advance the FSM")

			code, subcode, found := notificationFrom(t, written)
			require.True(t, found, "RFC 7607 Section 2 requires a NOTIFICATION")
			assert.Equal(t, message.NotifyOpenMessage, message.NotifyErrorCode(code),
				"error code must be OPEN Message Error")
			assert.Equal(t, message.NotifyOpenBadPeerAS, subcode, "subcode must be Bad Peer AS")
		})
	}
}

// TestProcessOpenRejectsPeerASZero holds the sibling rail. processOpen is what a
// connection takes after WINNING collision resolution, and a check on only one rail is
// bypassed by arriving as the second connection.
//
// RFC requirement: RFC7607-2-4 positive -- the collision-winner rail aborts a peer AS of
// zero with the same NOTIFICATION.
// RFC requirement: RFC7607-2-4 negative -- the same rail accepts a real peer AS.
func TestProcessOpenRejectsPeerASZero(t *testing.T) {
	for _, tt := range []struct {
		name       string
		peerAS     uint16
		wantReject bool
	}{
		{"peer AS zero", 0, true},
		{"real peer AS", 65002, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session, written := openSentSessionAS(t, 65002, 0x0A000002)

			open, err := message.UnpackOpen(openBodyWithIdentifier(tt.peerAS, 0x0A000001))
			require.NoError(t, err)

			err = session.processOpen(open)

			if !tt.wantReject {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, ErrBadPeerAS)
			code, subcode, found := notificationFrom(t, written)
			require.True(t, found, "RFC 7607 Section 2 requires a NOTIFICATION")
			assert.Equal(t, message.NotifyOpenMessage, message.NotifyErrorCode(code))
			assert.Equal(t, message.NotifyOpenBadPeerAS, subcode)
		})
	}
}

// acceptWithLocalAS builds a session whose LOCAL AS the caller chooses and hands it an
// accepted connection, returning what Accept answered and everything ze put on the socket.
//
// Accept is the entry point an inbound peering takes, and connectionEstablished calls
// sendOpen from inside it, so the answer here is the answer a real peering gets. Calling
// sendOpen on a session that was never connected would answer ErrNotConnected instead and
// prove nothing about the AS (writeMessageWithin, session_write.go).
func acceptWithLocalAS(t *testing.T, localAS uint32) (*Session, chan []byte, error) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), localAS, 65002, 0x0A000002)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	written := make(chan []byte, 8)
	go func() {
		for {
			buf := make([]byte, 4096)
			n, err := client.Read(buf)
			if err != nil {
				close(written)
				return
			}
			written <- buf[:n]
		}
	}()

	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	acceptErr := session.Accept(server)
	return session, written, acceptErr
}

// openFrom returns the first OPEN ze wrote, or reports that none arrived.
func openFrom(t *testing.T, written chan []byte) (open []byte, found bool) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case msg, ok := <-written:
			if !ok {
				return nil, false
			}
			// header: 16 marker + 2 length + 1 type.
			if len(msg) >= 19 && msg[18] == 1 {
				return msg, true
			}
		case <-deadline:
			return nil, false
		}
	}
}

// TestSendOpenRefusesLocalASZero drives the accept rail, which reaches sendOpen -- the one
// place an OPEN ze built reaches a socket.
//
// RFC requirement: RFC7607-2-5 positive -- a session whose local AS is zero puts no OPEN
// on the socket and refuses the connection, so ze never initiates a connection claiming to
// be AS 0.
// RFC requirement: RFC7607-2-5 negative -- a session with a real local AS sends its OPEN
// carrying that AS in the My Autonomous System field, so the refusal is bound to the zero
// and does not disable OPEN altogether.
func TestSendOpenRefusesLocalASZero(t *testing.T) {
	for _, tt := range []struct {
		name       string
		localAS    uint32
		wantRefuse bool
	}{
		{"local AS zero", 0, true},
		{"real local AS", 65001, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session, written, err := acceptWithLocalAS(t, tt.localAS)

			if tt.wantRefuse {
				require.ErrorIs(t, err, ErrLocalASZero)
				_, found := openFrom(t, written)
				assert.False(t, found, "no OPEN may reach the socket when the local AS is zero")
				return
			}

			require.NoError(t, err)
			require.Equal(t, fsm.StateOpenSent, session.State(),
				"OpenSent is reachable only when sendOpen succeeded")

			open, found := openFrom(t, written)
			require.True(t, found, "no OPEN reached the socket")
			require.GreaterOrEqual(t, len(open), 22, "the OPEN is short")
			myAS := uint16(open[20])<<8 | uint16(open[21])
			assert.Equal(t, uint16(65001), myAS, "ze must announce its configured AS")
		})
	}
}
