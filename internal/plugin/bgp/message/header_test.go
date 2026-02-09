package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseHeaderValid verifies parsing of valid BGP header.
//
// VALIDATES: Correct extraction of length and type from wire format.
//
// PREVENTS: Incorrect message framing causing session drops.
func TestParseHeaderValid(t *testing.T) {
	// Valid KEEPALIVE header: 16-byte marker + length(19) + type(4)
	data := make([]byte, HeaderLen)
	for i := 0; i < MarkerLen; i++ {
		data[i] = 0xFF
	}
	data[16] = 0x00 // Length high byte
	data[17] = 0x13 // Length low byte (19)
	data[18] = 0x04 // Type KEEPALIVE

	h, err := ParseHeader(data)
	require.NoError(t, err)
	assert.Equal(t, uint16(19), h.Length)
	assert.Equal(t, TypeKEEPALIVE, h.Type)
}

// TestParseHeaderAllTypes verifies all message type values.
//
// VALIDATES: Type byte correctly mapped to MessageType.
//
// PREVENTS: Wrong message type causing incorrect parsing.
func TestParseHeaderAllTypes(t *testing.T) {
	tests := []struct {
		typeByte byte
		expected MessageType
	}{
		{1, TypeOPEN},
		{2, TypeUPDATE},
		{3, TypeNOTIFICATION},
		{4, TypeKEEPALIVE},
		{5, TypeROUTEREFRESH},
	}

	for _, tt := range tests {
		data := makeHeader(19, tt.typeByte)
		h, err := ParseHeader(data)
		require.NoError(t, err)
		assert.Equal(t, tt.expected, h.Type)
	}
}

// TestParseHeaderInvalidMarker verifies marker validation.
//
// VALIDATES: Invalid marker is rejected.
//
// PREVENTS: Processing garbage as BGP messages.
func TestParseHeaderInvalidMarker(t *testing.T) {
	data := makeHeader(19, byte(TypeKEEPALIVE))
	data[0] = 0x00 // Corrupt marker

	_, err := ParseHeader(data)
	assert.ErrorIs(t, err, ErrInvalidMarker)
}

// TestParseHeaderShortRead verifies short input handling.
//
// VALIDATES: Insufficient data is rejected.
//
// PREVENTS: Panic on incomplete header read.
func TestParseHeaderShortRead(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"1 byte", []byte{0xFF}},
		{"marker only", make([]byte, 16)},
		{"marker + length", make([]byte, 18)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseHeader(tt.data)
			assert.ErrorIs(t, err, ErrShortRead)
		})
	}
}

// TestParseHeaderLengthBounds verifies length validation.
//
// VALIDATES: Invalid lengths are rejected.
//
// PREVENTS: Buffer overflow from malicious length values.
func TestParseHeaderLengthBounds(t *testing.T) {
	tests := []struct {
		name   string
		length uint16
		err    error
	}{
		{"too short (18)", 18, ErrInvalidLength},
		{"minimum (19)", 19, nil},
		{"typical UPDATE", 100, nil},
		{"max standard (4096)", MaxMsgLen, nil},
		{"extended (8192)", 8192, nil}, // Valid if extended message negotiated
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := makeHeader(tt.length, byte(TypeKEEPALIVE))
			_, err := ParseHeader(data)
			if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestHeaderWriteTo verifies header serialization.
//
// VALIDATES: Correct wire format output.
//
// PREVENTS: Malformed messages sent to peers.
func TestHeaderWriteTo(t *testing.T) {
	h := Header{
		Length: 50,
		Type:   TypeUPDATE,
	}

	buf := make([]byte, HeaderLen)
	n := h.WriteTo(buf, 0)

	require.Equal(t, HeaderLen, n)

	// Check marker
	for i := 0; i < MarkerLen; i++ {
		assert.Equal(t, byte(0xFF), buf[i], "marker byte %d", i)
	}

	// Check length
	assert.Equal(t, byte(0x00), buf[16])
	assert.Equal(t, byte(0x32), buf[17]) // 50 = 0x32

	// Check type
	assert.Equal(t, byte(TypeUPDATE), buf[18])
}

// TestHeaderRoundTrip verifies pack/parse symmetry.
//
// VALIDATES: Serialization is reversible.
//
// PREVENTS: Data corruption in pack/parse cycle.
func TestHeaderRoundTrip(t *testing.T) {
	original := Header{
		Length: 1234,
		Type:   TypeNOTIFICATION,
	}

	buf := make([]byte, HeaderLen)
	n := original.WriteTo(buf, 0)
	require.Equal(t, HeaderLen, n)

	parsed, err := ParseHeader(buf)
	require.NoError(t, err)

	assert.Equal(t, original.Length, parsed.Length)
	assert.Equal(t, original.Type, parsed.Type)
}

// TestMessageTypeString verifies string representation.
//
// VALIDATES: Readable message type names for logging.
//
// PREVENTS: Cryptic numeric values in logs.
func TestMessageTypeString(t *testing.T) {
	tests := []struct {
		t        MessageType
		expected string
	}{
		{TypeOPEN, "OPEN"},
		{TypeUPDATE, "UPDATE"},
		{TypeNOTIFICATION, "NOTIFICATION"},
		{TypeKEEPALIVE, "KEEPALIVE"},
		{TypeROUTEREFRESH, "ROUTE-REFRESH"},
		{MessageType(99), "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.t.String())
	}
}

// makeHeader creates a valid header with given length and type.
func makeHeader(length uint16, msgType byte) []byte {
	data := make([]byte, HeaderLen)
	for i := 0; i < MarkerLen; i++ {
		data[i] = 0xFF
	}
	data[16] = byte(length >> 8)
	data[17] = byte(length)
	data[18] = msgType
	return data
}

// TestValidateLengthWithMax verifies length validation with extended message support.
//
// RFC 8654 Section 4: "The BGP Extended Message Capability applies to all
// messages except for OPEN and KEEPALIVE messages."
//
// Upper bounds:
// - OPEN, KEEPALIVE: always 4096
// - UPDATE, NOTIFICATION, ROUTE-REFRESH: 4096 or 65535 if extended
//
// VALIDATES: Upper bound correctly enforced based on message type and extended capability.
//
// PREVENTS: Buffer overflow from maliciously large messages.
func TestValidateLengthWithMax(t *testing.T) {
	tests := []struct {
		name     string
		msgType  MessageType
		length   uint16
		extended bool
		wantErr  bool
	}{
		// OPEN: always 4096 max, regardless of extended
		{"OPEN at 4096", TypeOPEN, 4096, false, false},
		{"OPEN at 4096 with extended", TypeOPEN, 4096, true, false},
		{"OPEN over 4096", TypeOPEN, 4097, false, true},
		{"OPEN over 4096 with extended", TypeOPEN, 4097, true, true},

		// KEEPALIVE: exactly 19
		{"KEEPALIVE exact", TypeKEEPALIVE, 19, false, false},
		{"KEEPALIVE too long", TypeKEEPALIVE, 20, false, true},

		// UPDATE: 4096 or 65535
		{"UPDATE at 4096", TypeUPDATE, 4096, false, false},
		{"UPDATE over 4096 without extended", TypeUPDATE, 4097, false, true},
		{"UPDATE over 4096 with extended", TypeUPDATE, 4097, true, false},
		{"UPDATE at 65535 with extended", TypeUPDATE, 65535, true, false},

		// NOTIFICATION: 4096 or 65535
		{"NOTIFICATION at 4096", TypeNOTIFICATION, 4096, false, false},
		{"NOTIFICATION over 4096 without extended", TypeNOTIFICATION, 4097, false, true},
		{"NOTIFICATION at 65535 with extended", TypeNOTIFICATION, 65535, true, false},

		// ROUTE-REFRESH: 4096 or 65535
		{"ROUTE-REFRESH at 4096", TypeROUTEREFRESH, 4096, false, false},
		{"ROUTE-REFRESH over 4096 without extended", TypeROUTEREFRESH, 4097, false, true},
		{"ROUTE-REFRESH at 65535 with extended", TypeROUTEREFRESH, 65535, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Header{Length: tt.length, Type: tt.msgType}
			err := h.ValidateLengthWithMax(tt.extended)
			if tt.wantErr {
				require.Error(t, err)
				var notif *Notification
				require.ErrorAs(t, err, &notif)
				assert.Equal(t, NotifyMessageHeader, notif.ErrorCode)
				assert.Equal(t, NotifyHeaderBadLength, notif.ErrorSubcode)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestMaxMessageLength verifies MaxMessageLength helper function.
//
// RFC 8654 Section 4: OPEN and KEEPALIVE always 4096, others depend on extended capability.
//
// VALIDATES: Correct max length returned for each message type.
//
// PREVENTS: Using wrong buffer sizes for message reading.
func TestMaxMessageLength(t *testing.T) {
	tests := []struct {
		name     string
		msgType  MessageType
		extended bool
		want     uint16
	}{
		{"OPEN without extended", TypeOPEN, false, 4096},
		{"OPEN with extended", TypeOPEN, true, 4096},
		{"KEEPALIVE without extended", TypeKEEPALIVE, false, 4096},
		{"KEEPALIVE with extended", TypeKEEPALIVE, true, 4096},
		{"UPDATE without extended", TypeUPDATE, false, 4096},
		{"UPDATE with extended", TypeUPDATE, true, 65535},
		{"NOTIFICATION without extended", TypeNOTIFICATION, false, 4096},
		{"NOTIFICATION with extended", TypeNOTIFICATION, true, 65535},
		{"ROUTE-REFRESH without extended", TypeROUTEREFRESH, false, 4096},
		{"ROUTE-REFRESH with extended", TypeROUTEREFRESH, true, 65535},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxMessageLength(tt.msgType, tt.extended)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestValidateMessageLength verifies per-message-type length validation.
//
// RFC 4271 Section 6.1: "if the Length field of an OPEN message is less than
// the minimum length of the OPEN message" -> Bad Message Length error.
//
// Minimum lengths per RFC 4271:
// - OPEN: 29 octets (Section 4.2)
// - UPDATE: 23 octets (Section 4.3)
// - NOTIFICATION: 21 octets (Section 4.5)
// - KEEPALIVE: exactly 19 octets (Section 4.4)
//
// VALIDATES: Messages with invalid lengths for their type are rejected.
//
// PREVENTS: Processing truncated messages that could cause parsing errors.
func TestValidateMessageLength(t *testing.T) {
	tests := []struct {
		name    string
		msgType MessageType
		length  uint16
		wantErr bool
	}{
		// OPEN: minimum 29
		{"OPEN at minimum", TypeOPEN, 29, false},
		{"OPEN above minimum", TypeOPEN, 100, false},
		{"OPEN below minimum", TypeOPEN, 28, true},
		{"OPEN at header only", TypeOPEN, 19, true},

		// UPDATE: minimum 23
		{"UPDATE at minimum", TypeUPDATE, 23, false},
		{"UPDATE above minimum", TypeUPDATE, 500, false},
		{"UPDATE below minimum", TypeUPDATE, 22, true},
		{"UPDATE at header only", TypeUPDATE, 19, true},

		// NOTIFICATION: minimum 21
		{"NOTIFICATION at minimum", TypeNOTIFICATION, 21, false},
		{"NOTIFICATION above minimum", TypeNOTIFICATION, 50, false},
		{"NOTIFICATION below minimum", TypeNOTIFICATION, 20, true},
		{"NOTIFICATION at header only", TypeNOTIFICATION, 19, true},

		// KEEPALIVE: exactly 19
		{"KEEPALIVE exact", TypeKEEPALIVE, 19, false},
		{"KEEPALIVE too long", TypeKEEPALIVE, 20, true},
		{"KEEPALIVE way too long", TypeKEEPALIVE, 100, true},

		// ROUTE-REFRESH: minimum 23 (RFC 2918: 4 bytes after header)
		{"ROUTE-REFRESH at minimum", TypeROUTEREFRESH, 23, false},
		{"ROUTE-REFRESH above minimum", TypeROUTEREFRESH, 30, false},
		{"ROUTE-REFRESH below minimum", TypeROUTEREFRESH, 22, true},

		// Unknown type: only basic length check
		{"Unknown type", MessageType(99), 19, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Header{Length: tt.length, Type: tt.msgType}
			err := h.ValidateLength()
			if tt.wantErr {
				require.Error(t, err, "expected error for %s", tt.name)
				// Error should be a *Notification with code 1 (Message Header Error), subcode 2 (Bad Message Length)
				var notif *Notification
				require.ErrorAs(t, err, &notif)
				assert.Equal(t, NotifyMessageHeader, notif.ErrorCode)
				assert.Equal(t, NotifyHeaderBadLength, notif.ErrorSubcode)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
