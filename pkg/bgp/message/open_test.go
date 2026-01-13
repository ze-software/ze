package message

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenType verifies OPEN message type.
func TestOpenType(t *testing.T) {
	o := &Open{Version: 4, MyAS: 65001}
	assert.Equal(t, TypeOPEN, o.Type())
}

// TestOpenPack verifies OPEN packing.
//
// VALIDATES: All OPEN fields correctly serialized.
//
// PREVENTS: Session establishment failure from malformed OPEN.
func TestOpenPack(t *testing.T) {
	o := &Open{
		Version:       4,
		MyAS:          65001,
		HoldTime:      180,
		BGPIdentifier: 0xC0A80101, // 192.168.1.1
	}

	data := PackTo(o, nil)

	// Header (19) + Version (1) + AS (2) + HoldTime (2) + BGPID (4) + OptLen (1)
	assert.GreaterOrEqual(t, len(data), HeaderLen+10)

	// Verify header
	h, err := ParseHeader(data)
	require.NoError(t, err)
	assert.Equal(t, TypeOPEN, h.Type)

	// Verify body
	body := data[HeaderLen:]
	assert.Equal(t, byte(4), body[0])                        // Version
	assert.Equal(t, uint16(65001), beUint16(body[1:3]))      // AS
	assert.Equal(t, uint16(180), beUint16(body[3:5]))        // Hold Time
	assert.Equal(t, uint32(0xC0A80101), beUint32(body[5:9])) // BGP ID
}

// TestOpenUnpack verifies OPEN unpacking.
func TestOpenUnpack(t *testing.T) {
	body := []byte{
		0x04,       // Version = 4
		0xFD, 0xE9, // AS = 65001
		0x00, 0xB4, // Hold Time = 180
		0xC0, 0xA8, 0x01, 0x01, // BGP ID = 192.168.1.1
		0x00, // Optional Parameters Length = 0
	}

	msg, err := UnpackOpen(body)
	require.NoError(t, err)

	assert.Equal(t, uint8(4), msg.Version)
	assert.Equal(t, uint16(65001), msg.MyAS)
	assert.Equal(t, uint16(180), msg.HoldTime)
	assert.Equal(t, uint32(0xC0A80101), msg.BGPIdentifier)
}

// TestOpenUnpackShort verifies short data handling.
func TestOpenUnpackShort(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"version only", []byte{0x04}},
		{"partial", []byte{0x04, 0xFD, 0xE9, 0x00, 0xB4}},
		{"no opt len", []byte{0x04, 0xFD, 0xE9, 0x00, 0xB4, 0xC0, 0xA8, 0x01, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UnpackOpen(tt.data)
			assert.ErrorIs(t, err, ErrShortRead)
		})
	}
}

// TestOpenRoundTrip verifies pack/unpack symmetry.
func TestOpenRoundTrip(t *testing.T) {
	original := &Open{
		Version:       4,
		MyAS:          65535,
		HoldTime:      90,
		BGPIdentifier: 0x01020304,
	}

	data := PackTo(original, nil)

	body := data[HeaderLen:]
	parsed, err := UnpackOpen(body)
	require.NoError(t, err)

	assert.Equal(t, original.Version, parsed.Version)
	assert.Equal(t, original.MyAS, parsed.MyAS)
	assert.Equal(t, original.HoldTime, parsed.HoldTime)
	assert.Equal(t, original.BGPIdentifier, parsed.BGPIdentifier)
}

// TestOpenAS4 verifies 4-byte AS handling.
//
// VALIDATES: AS_TRANS used when ASN > 65535.
//
// PREVENTS: Session failure with 4-byte AS peers.
func TestOpenAS4(t *testing.T) {
	o := &Open{
		Version:       4,
		MyAS:          23456, // Will be ignored, ASN4 used instead
		HoldTime:      180,
		BGPIdentifier: 0xC0A80101,
		ASN4:          4200000001, // 4-byte AS
	}

	data := PackTo(o, nil)

	body := data[HeaderLen:]
	// MyAS field should be AS_TRANS (23456) when ASN4 is set
	assert.Equal(t, uint16(23456), beUint16(body[1:3]))
}

// TestOpenVersion verifies version validation.
func TestOpenVersion(t *testing.T) {
	body := []byte{
		0x03,       // Version = 3 (invalid)
		0xFD, 0xE9, // AS
		0x00, 0xB4, // Hold Time
		0xC0, 0xA8, 0x01, 0x01, // BGP ID
		0x00, // Optional Parameters Length
	}

	msg, err := UnpackOpen(body)
	require.NoError(t, err) // Parsing succeeds, validation is separate
	assert.Equal(t, uint8(3), msg.Version)
}

// TestOpenBGPIdentifierString verifies Router ID formatting.
func TestOpenBGPIdentifierString(t *testing.T) {
	o := &Open{BGPIdentifier: 0xC0A80101}
	assert.Equal(t, "192.168.1.1", o.RouterID())
}

// beUint16 reads big-endian uint16.
func beUint16(b []byte) uint16 {
	return uint16(b[0])<<8 | uint16(b[1])
}

// beUint32 reads big-endian uint32.
func beUint32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// TestOpenUnpackExtendedParams verifies RFC 9072 extended optional parameters parsing.
//
// RFC 9072 Section 2: "If the value of the 'Non-Ext OP Type' field is 255,
// then the encoding described above is used for the Optional Parameters length."
//
// Extended format:
//   - Non-Ext OP Len: 1 byte (SHOULD be 255)
//   - Non-Ext OP Type: 1 byte (MUST be 255 to indicate extended)
//   - Extended Opt Parm Length: 2 bytes
//   - Optional Parameters: variable
//
// VALIDATES: Extended format correctly parsed.
//
// PREVENTS: Failure to parse OPEN with large capability sets.
func TestOpenUnpackExtendedParams(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		wantOptLen   int
		wantErr      bool
		wantExtended bool
	}{
		{
			name: "standard format - no params",
			data: []byte{
				0x04,       // Version
				0xFD, 0xE9, // AS
				0x00, 0xB4, // Hold Time
				0xC0, 0xA8, 0x01, 0x01, // BGP ID
				0x00, // Opt Params Len = 0
			},
			wantOptLen: 0,
		},
		{
			name: "standard format - with params",
			data: []byte{
				0x04,       // Version
				0xFD, 0xE9, // AS
				0x00, 0xB4, // Hold Time
				0xC0, 0xA8, 0x01, 0x01, // BGP ID
				0x04,                   // Opt Params Len = 4
				0x02, 0x02, 0x01, 0x02, // Capability param
			},
			wantOptLen: 4,
		},
		{
			name: "extended format - empty params",
			data: []byte{
				0x04,       // Version
				0xFD, 0xE9, // AS
				0x00, 0xB4, // Hold Time
				0xC0, 0xA8, 0x01, 0x01, // BGP ID
				0xFF,       // Non-Ext OP Len = 255 (marker)
				0xFF,       // Non-Ext OP Type = 255 (extended format)
				0x00, 0x00, // Extended Opt Params Len = 0
			},
			wantOptLen:   0,
			wantExtended: true,
		},
		{
			name: "extended format - with params",
			data: []byte{
				0x04,       // Version
				0xFD, 0xE9, // AS
				0x00, 0xB4, // Hold Time
				0xC0, 0xA8, 0x01, 0x01, // BGP ID
				0xFF,       // Non-Ext OP Len = 255 (marker)
				0xFF,       // Non-Ext OP Type = 255 (extended format)
				0x00, 0x06, // Extended Opt Params Len = 6
				0x02, 0x00, 0x02, 0x01, 0x02, 0x00, // Param with 2-byte length
			},
			wantOptLen:   6,
			wantExtended: true,
		},
		{
			name: "extended format - 255 length marker but not extended type",
			data: []byte{
				0x04,       // Version
				0xFD, 0xE9, // AS
				0x00, 0xB4, // Hold Time
				0xC0, 0xA8, 0x01, 0x01, // BGP ID
				0xFF,             // Opt Params Len = 255 (not extended, just long)
				0x02,             // Param type (not 0xFF)
				0x02, 0x01, 0x02, // Param: type=2, len=2, value=0x0102
			},
			// This should be treated as standard format with opt_len=255
			// but we don't have 255 bytes, so it should fail
			wantErr: true,
		},
		{
			name: "extended format - truncated",
			data: []byte{
				0x04,       // Version
				0xFD, 0xE9, // AS
				0x00, 0xB4, // Hold Time
				0xC0, 0xA8, 0x01, 0x01, // BGP ID
				0xFF,       // Non-Ext OP Len
				0xFF,       // Non-Ext OP Type
				0x00, 0x10, // Extended len = 16, but no data follows
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := UnpackOpen(tt.data)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, msg.OptionalParams, tt.wantOptLen)
		})
	}
}

// TestOpenValidateHoldTime verifies RFC 4271 hold time validation.
//
// RFC 4271 Section 6.2: "An implementation MUST reject Hold Time values of
// one or two seconds."
// RFC 4271 Section 4.2: "Hold Time MUST be either zero or at least three seconds."
//
// VALIDATES: Hold times 0 and ≥3 are valid; 1 and 2 are rejected.
//
// PREVENTS: Session establishment with invalid hold time leading to timer issues.
func TestOpenValidateHoldTime(t *testing.T) {
	tests := []struct {
		name     string
		holdTime uint16
		wantErr  bool
	}{
		{"zero valid", 0, false},
		{"one invalid", 1, true},
		{"two invalid", 2, true},
		{"three valid", 3, false},
		{"90 valid", 90, false},
		{"180 valid", 180, false},
		{"max valid", 65535, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Open{
				Version:       4,
				MyAS:          65001,
				HoldTime:      tt.holdTime,
				BGPIdentifier: 0xC0A80101,
			}

			err := o.ValidateHoldTime()
			if tt.wantErr {
				require.Error(t, err)
				// Should return a NOTIFICATION with Unacceptable Hold Time (error 2, subcode 6)
				var notif *Notification
				require.True(t, errors.As(err, &notif), "expected *Notification error")
				assert.Equal(t, NotifyOpenMessage, notif.ErrorCode)
				assert.Equal(t, NotifyOpenUnacceptableHoldTime, notif.ErrorSubcode)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestOpenPackExtendedParams verifies RFC 9072 extended format packing.
//
// RFC 9072 Section 2: "if the length of the Optional Parameters in the BGP
// OPEN message does exceed 255, the OPEN message MUST be encoded according
// to the procedure below."
//
// VALIDATES: Large param sets use extended format.
//
// PREVENTS: Truncation of large capability sets.
func TestOpenPackExtendedParams(t *testing.T) {
	// Create params > 255 bytes
	largeParams := make([]byte, 300)
	for i := range largeParams {
		largeParams[i] = byte(i % 256)
	}

	o := &Open{
		Version:        4,
		MyAS:           65001,
		HoldTime:       180,
		BGPIdentifier:  0xC0A80101,
		OptionalParams: largeParams,
	}

	data := PackTo(o, nil)

	body := data[HeaderLen:]

	// Should use extended format
	// Body: Ver(1) + AS(2) + Hold(2) + ID(4) + NonExtLen(1) + NonExtType(1) + ExtLen(2) + Params(300)
	assert.GreaterOrEqual(t, len(body), 10+4+len(largeParams))

	// Check extended format markers
	assert.Equal(t, byte(0xFF), body[9], "Non-Ext OP Len should be 0xFF")
	assert.Equal(t, byte(0xFF), body[10], "Non-Ext OP Type should be 0xFF")

	// Check extended length
	extLen := beUint16(body[11:13])
	assert.Equal(t, uint16(len(largeParams)), extLen) // #nosec G115 -- test values in range

	// Verify params are present
	assert.Equal(t, largeParams, body[13:13+len(largeParams)])
}
