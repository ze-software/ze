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

	data := PackTo(n, nil)

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

	data := PackTo(n, nil)

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

	data := PackTo(original, nil)

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

// TestBuildShutdownData verifies RFC 8203 shutdown communication data building.
//
// RFC 8203 Section 2: The Data field contains a 1-byte length followed by
// a UTF-8 encoded string of up to 128 bytes.
//
// VALIDATES: Shutdown data correctly built with length prefix and UTF-8 truncation.
//
// PREVENTS: Sending oversized or malformed shutdown messages to peers.
func TestBuildShutdownData(t *testing.T) {
	tests := []struct {
		name    string
		message string
		wantLen int // expected length byte value
		wantMsg string
	}{
		{
			name:    "empty message",
			message: "",
			wantLen: 0,
			wantMsg: "",
		},
		{
			name:    "short ASCII message",
			message: "maintenance window",
			wantLen: 18,
			wantMsg: "maintenance window",
		},
		{
			name:    "exactly 128 bytes",
			message: string(make([]byte, 128)),
			wantLen: 128,
			wantMsg: string(make([]byte, 128)),
		},
		{
			name:    "over 128 bytes truncated",
			message: string(append(make([]byte, 128), "overflow"...)),
			wantLen: 128,
			wantMsg: string(make([]byte, 128)),
		},
		{
			name:    "UTF-8 truncation at character boundary",
			message: string(append(make([]byte, 126), "日"...)), // 126 + 3 = 129, must truncate
			wantLen: 126,                                       // can't fit the 3-byte char
			wantMsg: string(make([]byte, 126)),
		},
		{
			name:    "UTF-8 multibyte fits exactly",
			message: string(append(make([]byte, 125), "日"...)), // 125 + 3 = 128, fits
			wantLen: 128,
			wantMsg: string(append(make([]byte, 125), "日"...)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := BuildShutdownData(tt.message)

			if tt.wantLen == 0 {
				assert.Equal(t, []byte{0}, data, "empty message should be length-byte only")
				return
			}

			require.GreaterOrEqual(t, len(data), 2, "data must have length byte + message")
			assert.Equal(t, tt.wantLen, int(data[0]), "length byte")
			assert.Equal(t, tt.wantMsg, string(data[1:]))
		})
	}
}

// TestBuildShutdownDataBoundary verifies RFC 8203 boundary: last valid (128), first invalid (129).
//
// VALIDATES: Boundary enforcement at exactly 128 bytes.
//
// PREVENTS: Off-by-one in truncation logic.
func TestBuildShutdownDataBoundary(t *testing.T) {
	// Last valid: exactly 128 bytes
	msg128 := string(make([]byte, 128))
	data := BuildShutdownData(msg128)
	assert.Equal(t, byte(128), data[0])
	assert.Len(t, data, 129) // 1 length + 128 message

	// First invalid: 129 bytes, must truncate to 128
	msg129 := string(make([]byte, 129))
	data = BuildShutdownData(msg129)
	assert.Equal(t, byte(128), data[0])
	assert.Len(t, data, 129) // still 1 + 128
}

// TestNotificationFSMSubcodes verifies FSM Error subcodes per RFC 6608.
//
// RFC 6608 Section 3 defines subcodes for FSM errors:
//   - 0: Unspecified Error
//   - 1: Receive Unexpected Message in OpenSent State
//   - 2: Receive Unexpected Message in OpenConfirm State
//   - 3: Receive Unexpected Message in Established State
//
// VALIDATES: FSM subcodes defined and have correct string representation.
//
// PREVENTS: Missing FSM error context when debugging state machine issues.
func TestNotificationFSMSubcodes(t *testing.T) {
	tests := []struct {
		subcode  uint8
		expected string
	}{
		{NotifyFSMUnspecified, "Unspecified Error"},
		{NotifyFSMUnexpectedOpenSent, "Receive Unexpected Message in OpenSent State"},
		{NotifyFSMUnexpectedOpenConfirm, "Receive Unexpected Message in OpenConfirm State"},
		{NotifyFSMUnexpectedEstablished, "Receive Unexpected Message in Established State"},
	}

	for _, tt := range tests {
		n := &Notification{ErrorCode: NotifyFSMError, ErrorSubcode: tt.subcode}
		assert.Contains(t, n.String(), tt.expected, "subcode %d", tt.subcode)
	}
}

// TestNotificationRouteRefreshSubcodes verifies Route Refresh Error subcodes per RFC 7313.
//
// RFC 7313 Section 5 defines Error Code 7 (Route-Refresh Message Error) with:
//   - 1: Invalid Message Length
//
// VALIDATES: Route Refresh subcodes defined and have correct string representation.
//
// PREVENTS: Missing context when enhanced route refresh errors occur.
func TestNotificationRouteRefreshSubcodes(t *testing.T) {
	tests := []struct {
		subcode  uint8
		expected string
	}{
		{NotifyRouteRefreshInvalidLength, "Invalid Message Length"},
	}

	for _, tt := range tests {
		n := &Notification{ErrorCode: NotifyRouteRefresh, ErrorSubcode: tt.subcode}
		assert.Contains(t, n.String(), tt.expected, "subcode %d", tt.subcode)
	}
}

// TestNotificationOpenRoleMismatch verifies OPEN Role Mismatch subcode per RFC 9234.
//
// RFC 9234 Section 4.2 defines subcode 11 for role mismatch errors.
//
// VALIDATES: Role Mismatch subcode defined for BGP role capability negotiation failures.
//
// PREVENTS: Ambiguous OPEN error when roles don't match.
func TestNotificationOpenRoleMismatch(t *testing.T) {
	n := &Notification{ErrorCode: NotifyOpenMessage, ErrorSubcode: NotifyOpenRoleMismatch}
	assert.Contains(t, n.String(), "Role Mismatch")
}

// TestNotificationCeaseBFDDown verifies Cease BFD Down subcode per RFC 9384.
//
// RFC 9384 defines subcode 10 for session termination due to BFD down.
//
// VALIDATES: BFD Down subcode defined for BFD-triggered session termination.
//
// PREVENTS: Ambiguous Cease notification when BFD triggers shutdown.
func TestNotificationCeaseBFDDown(t *testing.T) {
	n := &Notification{ErrorCode: NotifyCease, ErrorSubcode: NotifyCeaseBFDDown}
	assert.Contains(t, n.String(), "BFD Down")
}

// TestBuildShutdownDataRoundTrip verifies BuildShutdownData and ShutdownMessage
// are inverse operations for valid inputs.
//
// RFC 8203 Section 2: the Data field is 1-byte length + UTF-8 message.
// Building data and then parsing it back must yield the original message.
//
// VALIDATES: Round-trip symmetry between BuildShutdownData and ShutdownMessage.
//
// PREVENTS: Encoding/decoding mismatch losing shutdown communication content.
func TestBuildShutdownDataRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"ascii", "maintenance"},
		{"multibyte", "日本語"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := BuildShutdownData(tt.message)

			n := &Notification{
				ErrorCode:    NotifyCease,
				ErrorSubcode: NotifyCeaseAdminShutdown,
				Data:         data,
			}

			got, err := n.ShutdownMessage()
			require.NoError(t, err)
			assert.Equal(t, tt.message, got)
		})
	}
}

// TestBuildShutdownDataInvalidUTF8 verifies invalid UTF-8 bytes are stripped.
//
// RFC 9003 Section 2: message MUST be UTF-8 encoded.
// Invalid bytes are stripped before encoding, resulting in an empty message
// when the entire input is invalid.
//
// VALIDATES: Invalid UTF-8 input produces a valid zero-length shutdown data.
//
// PREVENTS: Sending malformed UTF-8 on the wire to peers.
func TestBuildShutdownDataInvalidUTF8(t *testing.T) {
	data := BuildShutdownData(string([]byte{0x80, 0x81}))
	assert.Equal(t, []byte{0}, data, "invalid UTF-8 stripped to empty should produce length-byte-only")
}

// TestBuildShutdownDataAllMultibyteTruncation verifies truncation with all-multibyte input.
//
// RFC 8203 Section 2: message is truncated at UTF-8 character boundary to 128 bytes max.
// 43 three-byte CJK characters = 129 bytes, which exceeds the 128-byte limit.
// Truncation must drop the last character to reach 42 chars = 126 bytes.
//
// VALIDATES: Truncation at character boundary for all-multibyte input.
//
// PREVENTS: Mid-character truncation producing invalid UTF-8.
func TestBuildShutdownDataAllMultibyteTruncation(t *testing.T) {
	// 43 copies of "日" (3 bytes each) = 129 bytes total, exceeds 128-byte limit.
	var input string
	for range 43 {
		input += "日"
	}
	require.Len(t, []byte(input), 129, "precondition: input must be 129 bytes")

	data := BuildShutdownData(input)

	// Must truncate to 42 chars = 126 bytes (can't fit the 43rd 3-byte char).
	require.GreaterOrEqual(t, len(data), 2, "data must have length byte + message")
	assert.Equal(t, 126, int(data[0]), "length byte should be 126")
	assert.Len(t, data[1:], 126, "message should be 126 bytes")

	// Verify the result is valid UTF-8 containing exactly 42 characters.
	msg := string(data[1:])
	assert.Equal(t, 42, len([]rune(msg)), "should contain exactly 42 CJK characters")
}
