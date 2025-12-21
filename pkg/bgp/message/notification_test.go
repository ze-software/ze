package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotificationType verifies NOTIFICATION message type.
func TestNotificationType(t *testing.T) {
	n := &Notification{ErrorCode: NotifyMessageHeader, ErrorSubcode: 1}
	assert.Equal(t, TypeNOTIFICATION, n.Type())
}

// TestNotificationPack verifies NOTIFICATION packing.
//
// VALIDATES: Error code, subcode, and data correctly serialized.
//
// PREVENTS: Malformed NOTIFICATION causing peer confusion.
func TestNotificationPack(t *testing.T) {
	n := &Notification{
		ErrorCode:    NotifyUpdateMessage,
		ErrorSubcode: NotifyUpdateMalformedAttr,
		Data:         []byte{0x01, 0x02, 0x03},
	}

	data, err := n.Pack(nil)
	require.NoError(t, err)

	// Header (19) + code (1) + subcode (1) + data (3)
	assert.Len(t, data, HeaderLen+5)

	// Verify header
	h, err := ParseHeader(data)
	require.NoError(t, err)
	assert.Equal(t, TypeNOTIFICATION, h.Type)
	assert.Equal(t, uint16(HeaderLen+5), h.Length)

	// Verify body
	body := data[HeaderLen:]
	assert.Equal(t, NotifyUpdateMessage, NotifyErrorCode(body[0]))
	assert.Equal(t, NotifyUpdateMalformedAttr, body[1])
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, body[2:])
}

// TestNotificationPackNoData verifies NOTIFICATION without data.
func TestNotificationPackNoData(t *testing.T) {
	n := &Notification{
		ErrorCode:    NotifyCease,
		ErrorSubcode: NotifyCeaseAdminShutdown,
	}

	data, err := n.Pack(nil)
	require.NoError(t, err)

	// Header (19) + code (1) + subcode (1)
	assert.Len(t, data, HeaderLen+2)
}

// TestNotificationUnpack verifies NOTIFICATION unpacking.
func TestNotificationUnpack(t *testing.T) {
	body := []byte{
		byte(NotifyOpenMessage),            //nolint:unconvert // Required for typed constant in slice literal
		byte(NotifyOpenUnsupportedVersion), //nolint:unconvert // Required for typed constant in slice literal
		0x04,                               // Supported version = 4
	}

	msg, err := UnpackNotification(body)
	require.NoError(t, err)

	assert.Equal(t, NotifyOpenMessage, msg.ErrorCode)
	assert.Equal(t, NotifyOpenUnsupportedVersion, msg.ErrorSubcode)
	assert.Equal(t, []byte{0x04}, msg.Data)
}

// TestNotificationUnpackMinimal verifies minimal NOTIFICATION.
func TestNotificationUnpackMinimal(t *testing.T) {
	body := []byte{byte(NotifyHoldTimerExpired), 0x00}

	msg, err := UnpackNotification(body)
	require.NoError(t, err)

	assert.Equal(t, NotifyHoldTimerExpired, msg.ErrorCode)
	assert.Equal(t, uint8(0), msg.ErrorSubcode)
	assert.Empty(t, msg.Data)
}

// TestNotificationUnpackShort verifies short data handling.
func TestNotificationUnpackShort(t *testing.T) {
	_, err := UnpackNotification([]byte{0x01}) // Only 1 byte
	assert.ErrorIs(t, err, ErrShortRead)

	_, err = UnpackNotification([]byte{}) // Empty
	assert.ErrorIs(t, err, ErrShortRead)
}

// TestNotificationRoundTrip verifies pack/unpack symmetry.
func TestNotificationRoundTrip(t *testing.T) {
	original := &Notification{
		ErrorCode:    NotifyUpdateMessage,
		ErrorSubcode: NotifyUpdateInvalidNextHop,
		Data:         []byte{192, 168, 1, 1}, // Bad next-hop
	}

	data, err := original.Pack(nil)
	require.NoError(t, err)

	body := data[HeaderLen:]
	parsed, err := UnpackNotification(body)
	require.NoError(t, err)

	assert.Equal(t, original.ErrorCode, parsed.ErrorCode)
	assert.Equal(t, original.ErrorSubcode, parsed.ErrorSubcode)
	assert.Equal(t, original.Data, parsed.Data)
}

// TestNotificationErrorCodeString verifies string representation.
func TestNotificationErrorCodeString(t *testing.T) {
	tests := []struct {
		code     NotifyErrorCode
		expected string
	}{
		{NotifyMessageHeader, "Message Header Error"},
		{NotifyOpenMessage, "OPEN Message Error"},
		{NotifyUpdateMessage, "UPDATE Message Error"},
		{NotifyHoldTimerExpired, "Hold Timer Expired"},
		{NotifyFSMError, "FSM Error"},
		{NotifyCease, "Cease"},
		{NotifyErrorCode(99), "Unknown(99)"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.code.String())
	}
}

// TestNotificationCeaseSubcodes verifies Cease subcodes.
func TestNotificationCeaseSubcodes(t *testing.T) {
	tests := []struct {
		subcode  uint8
		expected string
	}{
		{NotifyCeaseMaxPrefixes, "Maximum Number of Prefixes Reached"},
		{NotifyCeaseAdminShutdown, "Administrative Shutdown"},
		{NotifyCeasePeerDeconfigured, "Peer De-configured"},
		{NotifyCeaseAdminReset, "Administrative Reset"},
		{NotifyCeaseConnectionRejected, "Connection Rejected"},
		{NotifyCeaseOtherConfigChange, "Other Configuration Change"},
		{NotifyCeaseConnectionCollision, "Connection Collision Resolution"},
		{NotifyCeaseOutOfResources, "Out of Resources"},
	}

	for _, tt := range tests {
		n := &Notification{ErrorCode: NotifyCease, ErrorSubcode: tt.subcode}
		assert.Contains(t, n.String(), tt.expected)
	}
}

// TestShutdownCommunication verifies RFC 8203/9003 shutdown message parsing.
//
// RFC 9003 Section 2: "If a BGP speaker decides to terminate its session
// with a BGP neighbor, and it sends a NOTIFICATION message with the Error
// Code 'Cease' and Error Subcode 'Administrative Shutdown' or
// 'Administrative Reset', it MAY include a UTF-8-encoded string."
//
// Data format: 1-byte length + UTF-8 message (up to 255 bytes)
//
// VALIDATES: Shutdown communication correctly parsed from NOTIFICATION data.
//
// PREVENTS: Losing operator context when peer shuts down session.
func TestShutdownCommunication(t *testing.T) {
	tests := []struct {
		name    string
		subcode uint8
		data    []byte
		wantMsg string
		wantErr bool
	}{
		{
			name:    "valid Admin Shutdown with message",
			subcode: NotifyCeaseAdminShutdown,
			data:    append([]byte{11}, []byte("maintenance")...), // 11 bytes
			wantMsg: "maintenance",
		},
		{
			name:    "valid Admin Reset with message",
			subcode: NotifyCeaseAdminReset,
			data:    append([]byte{13}, []byte("config change")...), // 13 bytes
			wantMsg: "config change",
		},
		{
			name:    "empty message (length 0)",
			subcode: NotifyCeaseAdminShutdown,
			data:    []byte{0},
			wantMsg: "",
		},
		{
			name:    "no data (old style)",
			subcode: NotifyCeaseAdminShutdown,
			data:    nil,
			wantMsg: "",
		},
		{
			name:    "UTF-8 multibyte characters",
			subcode: NotifyCeaseAdminShutdown,
			data:    append([]byte{9}, []byte("日本語")...), // 9 bytes for 3 Japanese chars (3 bytes each)
			wantMsg: "日本語",
		},
		{
			name:    "max length 255",
			subcode: NotifyCeaseAdminShutdown,
			data: func() []byte {
				msg := make([]byte, 256)
				msg[0] = 255
				for i := 1; i <= 255; i++ {
					msg[i] = 'x'
				}
				return msg
			}(),
			wantMsg: string(make([]byte, 255)),
		},
		{
			name:    "length exceeds buffer",
			subcode: NotifyCeaseAdminShutdown,
			data:    []byte{100, 'a', 'b', 'c'}, // claims 100 bytes, only 3
			wantErr: true,
		},
		{
			name:    "invalid UTF-8",
			subcode: NotifyCeaseAdminShutdown,
			data:    []byte{3, 0xff, 0xfe, 0xfd}, // invalid UTF-8 sequence
			wantErr: true,
		},
		{
			name:    "non-cease error code ignored",
			subcode: 0,
			data:    []byte{0x01, 0x02, 0x03},
			wantMsg: "", // not a shutdown, no message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &Notification{
				ErrorCode:    NotifyCease,
				ErrorSubcode: tt.subcode,
				Data:         tt.data,
			}

			// For non-cease test, use different error code
			if tt.name == "non-cease error code ignored" {
				n.ErrorCode = NotifyUpdateMessage
			}

			msg, err := n.ShutdownMessage()
			if tt.wantErr {
				require.Error(t, err, "expected error for %s", tt.name)
				return
			}
			require.NoError(t, err)

			// For max length test, check it's all 'x' chars
			if tt.name == "max length 255" {
				assert.Len(t, msg, 255)
				for _, c := range msg {
					assert.Equal(t, 'x', c)
				}
				return
			}

			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}
