package message

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

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
	for i := range MarkerLen {
		data[i] = 0xFF
	}
	data[16] = 0x00 // Length high byte
	data[17] = 0x13 // Length low byte (19)
	data[18] = 0x04 // Type KEEPALIVE

	h, err := ParseHeader(data)
	require.NoError(t, err)
	assert.Equal(t, uint16(19), h.Length)
	assert.Equal(t, msgtype.TypeKEEPALIVE, h.Type)
}

// TestParseHeaderAllTypes verifies all message type values.
//
// VALIDATES: Type byte correctly mapped to msgtype.MessageType.
//
// PREVENTS: Wrong message type causing incorrect parsing.
//
// RFC requirement: RFC2918-3-1 negative -- a header whose type byte is not 5
// (1..4) decodes to OPEN/UPDATE/NOTIFICATION/KEEPALIVE, never ROUTE-REFRESH; only
// type byte 5 yields msgtype.TypeROUTEREFRESH. Proves the type-5 assignment is exclusive,
// so a non-5 message is not mistaken for a ROUTE-REFRESH.
func TestParseHeaderAllTypes(t *testing.T) {
	tests := []struct {
		typeByte byte
		expected msgtype.MessageType
	}{
		{1, msgtype.TypeOPEN},
		{2, msgtype.TypeUPDATE},
		{3, msgtype.TypeNOTIFICATION},
		{4, msgtype.TypeKEEPALIVE},
		{5, msgtype.TypeROUTEREFRESH},
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
	data := makeHeader(19, byte(msgtype.TypeKEEPALIVE))
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
			data := makeHeader(tt.length, byte(msgtype.TypeKEEPALIVE))
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
		Type:   msgtype.TypeUPDATE,
	}

	buf := make([]byte, HeaderLen)
	n := h.WriteTo(buf, 0)

	require.Equal(t, HeaderLen, n)

	// Check marker
	for i := range MarkerLen {
		assert.Equal(t, byte(0xFF), buf[i], "marker byte %d", i)
	}

	// Check length
	assert.Equal(t, byte(0x00), buf[16])
	assert.Equal(t, byte(0x32), buf[17]) // 50 = 0x32

	// Check type
	assert.Equal(t, byte(msgtype.TypeUPDATE), buf[18])
}

// TestHeaderRoundTrip verifies pack/parse symmetry.
//
// VALIDATES: Serialization is reversible.
//
// PREVENTS: Data corruption in pack/parse cycle.
func TestHeaderRoundTrip(t *testing.T) {
	original := Header{
		Length: 1234,
		Type:   msgtype.TypeNOTIFICATION,
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
		t        msgtype.MessageType
		expected string
	}{
		{msgtype.TypeOPEN, "OPEN"},
		{msgtype.TypeUPDATE, "UPDATE"},
		{msgtype.TypeNOTIFICATION, "NOTIFICATION"},
		{msgtype.TypeKEEPALIVE, "KEEPALIVE"},
		{msgtype.TypeROUTEREFRESH, "ROUTE-REFRESH"},
		{msgtype.MessageType(99), "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.t.String())
	}
}

// makeHeader creates a valid header with given length and type.
func makeHeader(length uint16, msgType byte) []byte {
	data := make([]byte, HeaderLen)
	for i := range MarkerLen {
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
		msgType  msgtype.MessageType
		length   uint16
		extended bool
		wantErr  bool
	}{
		// OPEN: always 4096 max, regardless of extended
		{"OPEN at 4096", msgtype.TypeOPEN, 4096, false, false},
		{"OPEN at 4096 with extended", msgtype.TypeOPEN, 4096, true, false},
		{"OPEN over 4096", msgtype.TypeOPEN, 4097, false, true},
		// RFC requirement: RFC8654-4-3 negative -- an OPEN of 4097 octets is rejected even with
		// the Extended Message capability negotiated; the extension never raises the OPEN/KEEPALIVE
		// cap above 4096 (internal/component/bgp/message/header.go:197-199).
		{"OPEN over 4096 with extended", msgtype.TypeOPEN, 4097, true, true},

		// KEEPALIVE: exactly 19
		{"KEEPALIVE exact", msgtype.TypeKEEPALIVE, 19, false, false},
		// RFC requirement: RFC8654-6-1 negative -- a KEEPALIVE of 20 octets violates its per-type
		// length and is rejected by ValidateLengthWithMax (internal/component/bgp/message/header.go:187,155-162).
		{"KEEPALIVE too long", msgtype.TypeKEEPALIVE, 20, false, true},

		// UPDATE: 4096 or 65535
		// RFC requirement: RFC8654-6-1 positive -- an UPDATE at exactly the 4096-octet per-type
		// maximum passes ValidateLengthWithMax (internal/component/bgp/message/header.go:200-207).
		{"UPDATE at 4096", msgtype.TypeUPDATE, 4096, false, false},
		// RFC requirement: RFC8654-4-1 negative -- without the Extended Message capability an UPDATE
		// above 4096 is rejected, so the up-to-65535 receive capacity is conditioned on advertising
		// the capability (internal/component/bgp/message/header.go:200-213).
		// RFC requirement: RFC8654-5-1 negative -- an over-4096 UPDATE received while extended is NOT
		// negotiated is rejected, never accepted (internal/component/bgp/message/header.go:200-213).
		// RFC requirement: RFC8654-5-2 negative -- there is no liberal-acceptance bypass: the same
		// length gate rejects the over-4096 UPDATE when extended is not negotiated (header.go:200-213).
		{"UPDATE over 4096 without extended", msgtype.TypeUPDATE, 4097, false, true},
		{"UPDATE over 4096 with extended", msgtype.TypeUPDATE, 4097, true, false},
		// RFC requirement: RFC8654-4-1 positive -- with the Extended Message capability negotiated an
		// UPDATE of 65535 octets is accepted (internal/component/bgp/message/header.go:200-205).
		// RFC requirement: RFC8654-5-1 positive -- when extended IS negotiated the over-4096 (65535)
		// UPDATE is accepted, the behavior 5-1 forbids only when unnegotiated (header.go:200-205).
		// RFC requirement: RFC8654-5-2 positive -- the length gate accepts the large UPDATE exactly
		// when the capability is negotiated, so the policy is no more liberal than negotiation allows
		// (internal/component/bgp/message/header.go:200-205).
		{"UPDATE at 65535 with extended", msgtype.TypeUPDATE, 65535, true, false},

		// NOTIFICATION: 4096 or 65535
		{"NOTIFICATION at 4096", msgtype.TypeNOTIFICATION, 4096, false, false},
		{"NOTIFICATION over 4096 without extended", msgtype.TypeNOTIFICATION, 4097, false, true},
		{"NOTIFICATION at 65535 with extended", msgtype.TypeNOTIFICATION, 65535, true, false},

		// ROUTE-REFRESH: 4096 or 65535
		{"ROUTE-REFRESH at 4096", msgtype.TypeROUTEREFRESH, 4096, false, false},
		{"ROUTE-REFRESH over 4096 without extended", msgtype.TypeROUTEREFRESH, 4097, false, true},
		{"ROUTE-REFRESH at 65535 with extended", msgtype.TypeROUTEREFRESH, 65535, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Header{Length: tt.length, Type: tt.msgType}
			err := h.ValidateLengthWithMax(tt.extended)
			if tt.wantErr {
				// RFC requirement: RFC8654-5-4 negative -- a bad message length yields a NOTIFICATION
				// with Message Header Error / Bad Message Length (header.go:207-213).
				require.Error(t, err)
				var notif *Notification
				require.ErrorAs(t, err, &notif)
				assert.Equal(t, NotifyMessageHeader, notif.ErrorCode)
				assert.Equal(t, NotifyHeaderBadLength, notif.ErrorSubcode)
			} else {
				// RFC requirement: RFC8654-5-4 positive -- a within-limit length produces no error and
				// thus no NOTIFICATION (internal/component/bgp/message/header.go:207-215).
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
		msgType  msgtype.MessageType
		extended bool
		want     uint16
	}{
		{"OPEN without extended", msgtype.TypeOPEN, false, 4096},
		// RFC requirement: RFC8654-4-3 positive -- MaxMessageLength for OPEN stays 4096 even when
		// extended is true, confirming OPEN/KEEPALIVE are never extended (header.go:234-235).
		{"OPEN with extended", msgtype.TypeOPEN, true, 4096},
		{"KEEPALIVE without extended", msgtype.TypeKEEPALIVE, false, 4096},
		{"KEEPALIVE with extended", msgtype.TypeKEEPALIVE, true, 4096},
		{"UPDATE without extended", msgtype.TypeUPDATE, false, 4096},
		{"UPDATE with extended", msgtype.TypeUPDATE, true, 65535},
		{"NOTIFICATION without extended", msgtype.TypeNOTIFICATION, false, 4096},
		{"NOTIFICATION with extended", msgtype.TypeNOTIFICATION, true, 65535},
		{"ROUTE-REFRESH without extended", msgtype.TypeROUTEREFRESH, false, 4096},
		{"ROUTE-REFRESH with extended", msgtype.TypeROUTEREFRESH, true, 65535},
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
		msgType msgtype.MessageType
		length  uint16
		wantErr bool
	}{
		// OPEN: minimum 29
		{"OPEN at minimum", msgtype.TypeOPEN, 29, false},
		{"OPEN above minimum", msgtype.TypeOPEN, 100, false},
		{"OPEN below minimum", msgtype.TypeOPEN, 28, true},
		{"OPEN at header only", msgtype.TypeOPEN, 19, true},

		// UPDATE: minimum 23
		{"UPDATE at minimum", msgtype.TypeUPDATE, 23, false},
		{"UPDATE above minimum", msgtype.TypeUPDATE, 500, false},
		{"UPDATE below minimum", msgtype.TypeUPDATE, 22, true},
		{"UPDATE at header only", msgtype.TypeUPDATE, 19, true},

		// NOTIFICATION: minimum 21
		{"NOTIFICATION at minimum", msgtype.TypeNOTIFICATION, 21, false},
		{"NOTIFICATION above minimum", msgtype.TypeNOTIFICATION, 50, false},
		{"NOTIFICATION below minimum", msgtype.TypeNOTIFICATION, 20, true},
		{"NOTIFICATION at header only", msgtype.TypeNOTIFICATION, 19, true},

		// KEEPALIVE: exactly 19
		{"KEEPALIVE exact", msgtype.TypeKEEPALIVE, 19, false},
		{"KEEPALIVE too long", msgtype.TypeKEEPALIVE, 20, true},
		{"KEEPALIVE way too long", msgtype.TypeKEEPALIVE, 100, true},

		// ROUTE-REFRESH: header floor only. RFC 7313 exact body length errors
		// are emitted by receive-path validation as ROUTE-REFRESH Message Error.
		{"ROUTE-REFRESH header only", msgtype.TypeROUTEREFRESH, 19, false},
		{"ROUTE-REFRESH below body length", msgtype.TypeROUTEREFRESH, 22, false},
		{"ROUTE-REFRESH at exact body length", msgtype.TypeROUTEREFRESH, 23, false},
		{"ROUTE-REFRESH above body length", msgtype.TypeROUTEREFRESH, 30, false},
		{"ROUTE-REFRESH below header", msgtype.TypeROUTEREFRESH, 18, true},

		// Unknown type: only basic length check
		{"Unknown type", msgtype.MessageType(99), 19, false},
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
