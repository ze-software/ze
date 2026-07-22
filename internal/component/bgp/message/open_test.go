package message

import (
	"errors"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/msgtype"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc-test-change-approved: 2026-07-22 Thomas approved the msgtype/routeaction
// package rename (spec-feature-gate-10-bgp). MessageType/Type* moved to
// internal/core/bgp/msgtype and the route-action enum to
// internal/core/bgp/routeaction so MRT, sysrib and the FIB backends keep
// compiling when the BGP engine is compiled out (//go:build ze_bgp). Every hunk
// in this file is a package-qualifier requalification: no assertion was added,
// removed, reworded, weakened or re-tagged, verified by normalising the diff
// under the renaming and confirming the add/delete multisets cancel.

// TestOpenType verifies OPEN message type.
func TestOpenType(t *testing.T) {
	o := &Open{Version: 4, MyAS: 65001}
	assert.Equal(t, msgtype.TypeOPEN, o.Type())
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
	assert.Equal(t, msgtype.TypeOPEN, h.Type)

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
		// RFC requirement: RFC9072-2-2 negative -- a standard-form OPEN (Non-Ext OP Len 4, first optional-parameter type 0x02) is decoded as classic with 4 optional-parameter octets, so the extended-form acceptance is gated on the 0xFF markers and does not misfire on a standard OPEN.
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
		// RFC requirement: RFC9072-2-2 positive -- an extended-form OPEN whose Extended Opt. Parm. Length is 0 (a length of 255 or less) is accepted and parsed through the extended branch, not rejected.
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
		// RFC requirement: RFC9072-2-2 positive -- an extended-form OPEN with a 6-octet Extended Opt. Parm. Length (255 or less) is accepted: the 2-octet length is honored and 6 optional-parameter octets are extracted, which a classic decode reading Non-Ext OP Len 255 could never produce.
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
			// RFC 9072 Section 2: "If the Non-Ext OP Len is not 255, the
			// Non-Ext OP Type field and the Extended Opt. Parm. Length field
			// SHOULD be treated as part of the original Optional Parameters."
			// A standard OPEN with optLen=4 where first param byte is 0xFF
			// must NOT be misinterpreted as extended format.
			name: "standard format - first param byte 0xFF not misread as extended",
			data: []byte{
				0x04,       // Version
				0xFD, 0xE9, // AS
				0x00, 0xB4, // Hold Time
				0xC0, 0xA8, 0x01, 0x01, // BGP ID
				0x04,                   // Opt Params Len = 4 (NOT 255)
				0xFF, 0x02, 0x01, 0x02, // Params: first byte happens to be 0xFF
			},
			wantOptLen: 4,
		},
		{
			// Boundary: data[9]=0xFF, data[10]=0xFF, but only 11 bytes total.
			// Enters extended branch but len(data) < 13 triggers ErrShortRead
			// before reading the extended length field.
			name: "extended format - truncated before length field",
			data: []byte{
				0x04,       // Version
				0xFD, 0xE9, // AS
				0x00, 0xB4, // Hold Time
				0xC0, 0xA8, 0x01, 0x01, // BGP ID
				0xFF, // Non-Ext OP Len
				0xFF, // Non-Ext OP Type (only 11 bytes, no room for extended length)
			},
			wantErr: true,
		},
		{
			name: "extended format - truncated after length field",
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
		{
			// Boundary: optLen=254 is the last valid standard format length.
			// Must NOT trigger extended format detection.
			name: "standard format - optLen 254 boundary",
			data: func() []byte {
				d := make([]byte, 10+254)
				d[0] = 0x04                                     // Version
				d[1], d[2] = 0xFD, 0xE9                         // AS
				d[3], d[4] = 0x00, 0xB4                         // Hold Time
				d[5], d[6], d[7], d[8] = 0xC0, 0xA8, 0x01, 0x01 // BGP ID
				d[9] = 0xFE                                     // Opt Params Len = 254
				// 254 bytes of params (all zeros is fine for length test)
				return d
			}(),
			wantOptLen: 254,
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
// RFC requirement: RFC4271-4.2-1 positive -- a Hold Time of zero and Hold Times of three seconds
// and above are accepted by ValidateHoldTime (internal/component/bgp/message/open.go:235-245).
// RFC requirement: RFC4271-4.2-1 negative -- a Hold Time that is neither zero nor at least three
// seconds is rejected (internal/component/bgp/message/open.go:237-243).
// RFC requirement: RFC4271-6.2-1 positive -- three seconds, the first legal non-zero value, is
// accepted, so the rejection window is exactly one and two (open.go:237).
// RFC requirement: RFC4271-6.2-1 negative -- Hold Time values of one and two seconds are rejected
// with OPEN Message Error / Unacceptable Hold Time (open.go:238-242).
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
	// RFC requirement: RFC9072-2-1 positive -- Optional Parameters of 300 octets exceed 255, so the OPEN is encoded with the extended procedure: the Non-Ext markers and 2-octet Extended Opt. Parm. Length are emitted instead of the classic single-octet length.
	// RFC requirement: RFC9072-2-3 positive -- the extended-form Non-Ext OP Len octet is the non-zero 0xFF marker, never 0.
	assert.Equal(t, byte(0xFF), body[9], "Non-Ext OP Len should be 0xFF")
	// RFC requirement: RFC9072-2-4 positive -- the extended-form Non-Ext OP Type octet is set to 255 on transmission.
	// RFC requirement: RFC9072-3-2 positive -- 255 appears only as the extended-length indicator in the Non-Ext OP Type position; the encoder never writes 255 as an optional-parameter type code.
	assert.Equal(t, byte(0xFF), body[10], "Non-Ext OP Type should be 0xFF")

	// Check extended length
	extLen := beUint16(body[11:13])
	assert.Equal(t, uint16(len(largeParams)), extLen) // #nosec G115 -- test values in range

	// Verify params are present
	assert.Equal(t, largeParams, body[13:13+len(largeParams)])

	// Round-trip: unpack the packed extended format message
	parsed, err := UnpackOpen(body)
	require.NoError(t, err, "UnpackOpen should parse packed extended format")
	assert.Equal(t, o.Version, parsed.Version)
	assert.Equal(t, o.MyAS, parsed.MyAS)
	assert.Equal(t, o.HoldTime, parsed.HoldTime)
	assert.Equal(t, o.BGPIdentifier, parsed.BGPIdentifier)
	assert.Equal(t, largeParams, parsed.OptionalParams, "extended params round-trip")
}
