package reactor

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/capability"
)

// openSentSessionAS builds an OpenSent session whose peer AS is chosen by the caller, so a
// test can exercise the internal-peer branch of RFC 6286 Section 2.2 (peerAS == localAS).
// The returned client connection collects whatever ze writes, so a NOTIFICATION can be read
// back off the wire.
func openSentSessionAS(t *testing.T, peerAS, localID uint32) (*Session, chan []byte) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, peerAS, localID)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

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

	require.NoError(t, session.Accept(server))
	require.Equal(t, fsm.StateOpenSent, session.State())

	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	return session, written
}

// openBodyWithIdentifier builds a minimal valid OPEN body carrying the given identifier.
func openBodyWithIdentifier(peerAS uint16, bgpID uint32) []byte {
	return []byte{
		0x04,                            // Version 4
		byte(peerAS >> 8), byte(peerAS), // My AS
		0x00, 0x5A, // HoldTime = 90
		byte(bgpID >> 24), byte(bgpID >> 16), byte(bgpID >> 8), byte(bgpID), // BGP Identifier
		0x00, // OptParamLen = 0
	}
}

// notificationFrom scans messages ze wrote for a NOTIFICATION and returns its code/subcode.
func notificationFrom(t *testing.T, written chan []byte) (code, subcode uint8, found bool) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case msg, ok := <-written:
			if !ok {
				return 0, 0, false
			}
			// header: 16 marker + 2 length + 1 type, then code + subcode.
			if len(msg) >= 21 && msg[18] == 3 {
				return msg[19], msg[20], true
			}
		case <-deadline:
			return 0, 0, false
		}
	}
}

// TestHandleOpenRejectsBadBGPIdentifier verifies the handleOpen rail enforces RFC 6286
// Section 2.2 and reports it on the wire.
//
// RFC requirement: RFC6286-2.2-1 positive -- handleOpen rejects a zero BGP Identifier with
// NOTIFICATION OPEN Message Error / Bad BGP Identifier and does not advance the FSM.
// RFC requirement: RFC6286-2.2-2 positive -- handleOpen rejects this speaker's own
// identifier from an INTERNAL peer with the same NOTIFICATION.
// RFC requirement: RFC6286-2.2-2 negative -- handleOpen accepts this speaker's own
// identifier from an EXTERNAL peer, and any distinct identifier from an internal peer:
// the session advances to OpenConfirm and no NOTIFICATION is sent.
//
// VALIDATES: zero identifier -> ErrBadBGPIdentifier, NOTIFICATION 2/3, FSM not advanced.
// VALIDATES: local identifier from an INTERNAL peer -> same rejection.
// VALIDATES: local identifier from an EXTERNAL peer is accepted (Section 2.2 gates the self
// case on an internal peer; Section 2.3 is what resolves an external clash).
// PREVENTS: silently accepting an RFC-invalid identifier, and over-rejecting the external
// peer that legitimately shares one.
func TestHandleOpenRejectsBadBGPIdentifier(t *testing.T) {
	const localID uint32 = 0x01020301

	tests := []struct {
		name       string
		peerAS     uint32
		bgpID      uint32
		wantReject bool
	}{
		{"zero identifier from external peer", 65002, 0, true},
		{"zero identifier from internal peer", 65001, 0, true},
		{"own identifier from internal peer", 65001, localID, true},
		{"own identifier from external peer", 65002, localID, false},
		{"distinct identifier from internal peer", 65001, localID + 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, written := openSentSessionAS(t, tt.peerAS, localID)

			err := session.handleOpen(openBodyWithIdentifier(uint16(tt.peerAS), tt.bgpID)) //nolint:gosec // test AS numbers < 65536

			if !tt.wantReject {
				require.NoError(t, err, "a valid identifier must not be rejected")
				assert.Equal(t, fsm.StateOpenConfirm, session.State(), "session advances")
				return
			}

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrBadBGPIdentifier)
			assert.NotEqual(t, fsm.StateOpenConfirm, session.State(),
				"a rejected OPEN must not advance the FSM")

			code, subcode, found := notificationFrom(t, written)
			require.True(t, found, "a NOTIFICATION must be sent")
			assert.Equal(t, uint8(message.NotifyOpenMessage), code, "OPEN Message Error")
			assert.Equal(t, message.NotifyOpenBadBGPID, subcode, "Bad BGP Identifier")
		})
	}
}

// TestProcessOpenRejectsBadBGPIdentifier verifies the SECOND OPEN rail -- the one a
// connection takes after winning collision resolution -- enforces Section 2.2 too.
//
// RFC requirement: RFC6286-2.2-1 positive -- processOpen (AcceptWithOpen, the
// collision-winner replay) rejects a zero BGP Identifier with OPEN Message Error / Bad BGP
// Identifier, so the rail cannot be used to bypass the check.
//
// VALIDATES: processOpen rejects a zero identifier with NOTIFICATION 2/3.
// PREVENTS: an invalid identifier being accepted by arriving as the second connection.
func TestProcessOpenRejectsBadBGPIdentifier(t *testing.T) {
	session, written := openSentSessionAS(t, 65002, 0x01020301)

	err := session.processOpen(&message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBadBGPIdentifier)

	code, subcode, found := notificationFrom(t, written)
	require.True(t, found, "a NOTIFICATION must be sent")
	assert.Equal(t, uint8(message.NotifyOpenMessage), code)
	assert.Equal(t, message.NotifyOpenBadBGPID, subcode)
}

// TestProcessOpenRunsOpenValidator verifies the collision-winner rail runs the peer OPEN
// validator (RFC 9234 role checks, and the RFC 6286 Section 2.1 identifier claim).
//
// VALIDATES: processOpen calls openValidator and honors its rejection.
// PREVENTS: the regression this fixed -- processOpen skipped the validator entirely, so any
// per-peer OPEN policy could be bypassed by winning connection collision resolution.
func TestProcessOpenRunsOpenValidator(t *testing.T) {
	session, _ := openSentSessionAS(t, 65002, 0x01020301)

	called := false
	session.openValidator = func(peerAddr string, local, remote *message.Open) error {
		called = true
		return &routerIDConflictError{
			conflictAddr: netip.MustParseAddr("192.0.2.2"),
			peerAS:       65002,
			bgpID:        remote.BGPIdentifier,
		}
	}

	err := session.processOpen(&message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x05060708})
	require.Error(t, err)
	assert.True(t, called, "the collision-winner rail must run the OPEN validator")
	assert.Contains(t, err.Error(), "open validation failed")
}
