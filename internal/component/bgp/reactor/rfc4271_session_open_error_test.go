// VALIDATES: the four OPEN Error Subcodes RFC 4271 Section 6.2 assigns to a field of
// the received OPEN reach the wire as NOTIFICATION 2/<subcode>, and a conforming OPEN
// draws none of them.
// PREVENTS: a peer learning the wrong reason for a refused OPEN, and a check that refuses
// every OPEN passing the positive arm alone.

package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// rfc4271OpenBody builds an OPEN body field by field so one case can bend exactly one
// of them. The optional parameters are empty: the Section 6.2 subcodes under test are
// about the fixed fields.
func rfc4271OpenBody(version uint8, peerAS, holdTime uint16, bgpID uint32) []byte {
	return []byte{
		version,
		byte(peerAS >> 8), byte(peerAS),
		byte(holdTime >> 8), byte(holdTime),
		byte(bgpID >> 24), byte(bgpID >> 16), byte(bgpID >> 8), byte(bgpID),
		0x00, // Optional Parameters Length
	}
}

// rfc4271NotificationFrom scans what ze wrote for a NOTIFICATION and returns its code,
// subcode and Data field. notificationFrom (session_open_validation_test.go) drops the
// Data, and two of the subcodes below echo a value in it.
func rfc4271NotificationFrom(t *testing.T, written chan []byte) (code, subcode uint8, data []byte, found bool) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case msg, ok := <-written:
			if !ok {
				return 0, 0, nil, false
			}
			// Header: 16 marker + 2 length + 1 type, then code, subcode, data.
			if len(msg) >= 21 && msg[18] == 3 {
				return msg[19], msg[20], msg[21:], true
			}
		case <-deadline:
			return 0, 0, nil, false
		}
	}
}

// TestRFC4271OpenErrorSubcodes drives handleOpen, the rail every non-colliding
// connection takes, with one field of the OPEN wrong at a time and reads the
// NOTIFICATION back off the socket.
//
// Each subcode has a conforming twin: the same OPEN with that field valid is accepted,
// the session advances to OpenConfirm and nothing of type NOTIFICATION is written. That
// arm is what stops a handleOpen that refuses everything from passing the positive arm.
//
// RFC requirement: RFC4271-6.2-5 positive — Version 5 draws NOTIFICATION 2/1 Unsupported
// Version Number, and its two-octet Data field names version 4.
// RFC requirement: RFC4271-6.2-5 negative — Version 4 is accepted with no NOTIFICATION.
// RFC requirement: RFC4271-6.2-6 positive — a My AS the peer is not configured for draws
// NOTIFICATION 2/2 Bad Peer AS.
// RFC requirement: RFC4271-6.2-6 negative — the configured peer AS is accepted with no
// NOTIFICATION.
// RFC requirement: RFC4271-6.2-7 positive — Hold Time 1 and Hold Time 2 each draw
// NOTIFICATION 2/6 Unacceptable Hold Time, and the Data field echoes the value.
// RFC requirement: RFC4271-6.2-7 negative — Hold Time 90 is accepted with no NOTIFICATION.
// RFC requirement: RFC4271-6.2-8 positive — BGP Identifier 0.0.0.0 draws NOTIFICATION 2/3
// Bad BGP Identifier.
// RFC requirement: RFC4271-6.2-8 negative — a unicast host identifier is accepted with no
// NOTIFICATION.
func TestRFC4271OpenErrorSubcodes(t *testing.T) {
	const (
		localID   uint32 = 0x0A000002
		peerID    uint32 = 0x0A000001
		peerAS    uint16 = 65002
		holdTime  uint16 = 90
		goodOpen         = 4
		wantNoSub        = 255 // wantSubcode value meaning "accepted, no NOTIFICATION"
	)

	tests := []struct {
		name        string
		body        []byte
		wantSubcode uint8
		wantData    []byte
	}{
		{"version 5", rfc4271OpenBody(5, peerAS, holdTime, peerID), message.NotifyOpenUnsupportedVersion, []byte{0, 4}},
		{"version 4", rfc4271OpenBody(goodOpen, peerAS, holdTime, peerID), wantNoSub, nil},
		{"peer AS not configured", rfc4271OpenBody(goodOpen, 65003, holdTime, peerID), message.NotifyOpenBadPeerAS, nil},
		{"peer AS configured", rfc4271OpenBody(goodOpen, peerAS, holdTime, peerID), wantNoSub, nil},
		{"hold time 1", rfc4271OpenBody(goodOpen, peerAS, 1, peerID), message.NotifyOpenUnacceptableHoldTime, []byte{0, 1}},
		{"hold time 2", rfc4271OpenBody(goodOpen, peerAS, 2, peerID), message.NotifyOpenUnacceptableHoldTime, []byte{0, 2}},
		{"hold time 90", rfc4271OpenBody(goodOpen, peerAS, holdTime, peerID), wantNoSub, nil},
		{"identifier zero", rfc4271OpenBody(goodOpen, peerAS, holdTime, 0), message.NotifyOpenBadBGPID, nil},
		{"identifier unicast host", rfc4271OpenBody(goodOpen, peerAS, holdTime, peerID), wantNoSub, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, written := openSentSessionAS(t, uint32(peerAS), localID)

			err := session.handleOpen(tt.body)

			if tt.wantSubcode == wantNoSub {
				require.NoError(t, err, "a conforming OPEN must be accepted")
				assert.Equal(t, fsm.StateOpenConfirm, session.State(), "the session advances")
				code, subcode, _, found := rfc4271NotificationFrom(t, written)
				assert.False(t, found,
					"an accepted OPEN must draw no NOTIFICATION, got %d/%d", code, subcode)
				return
			}

			require.Error(t, err, "a refused OPEN surfaces as an error")
			assert.NotEqual(t, fsm.StateOpenConfirm, session.State(),
				"a refused OPEN must not advance the FSM")

			code, subcode, data, found := rfc4271NotificationFrom(t, written)
			require.True(t, found, "RFC 4271 Section 6.2: every OPEN error is indicated by a NOTIFICATION")
			assert.Equal(t, message.NotifyOpenMessage, message.NotifyErrorCode(code),
				"error code must be OPEN Message Error")
			assert.Equal(t, tt.wantSubcode, subcode, "the subcode names the field")
			if tt.wantData != nil {
				assert.Equal(t, tt.wantData, data, "the Data field echoes the value")
			}
		})
	}
}
