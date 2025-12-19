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

// TestHeaderPack verifies header serialization.
//
// VALIDATES: Correct wire format output.
//
// PREVENTS: Malformed messages sent to peers.
func TestHeaderPack(t *testing.T) {
	h := Header{
		Length: 50,
		Type:   TypeUPDATE,
	}

	data := h.Pack()

	require.Len(t, data, HeaderLen)

	// Check marker
	for i := 0; i < MarkerLen; i++ {
		assert.Equal(t, byte(0xFF), data[i], "marker byte %d", i)
	}

	// Check length
	assert.Equal(t, byte(0x00), data[16])
	assert.Equal(t, byte(0x32), data[17]) // 50 = 0x32

	// Check type
	assert.Equal(t, byte(TypeUPDATE), data[18])
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

	data := original.Pack()
	parsed, err := ParseHeader(data)
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
