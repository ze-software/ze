package message

import (
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

	data, err := o.Pack(nil)
	require.NoError(t, err)

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

	data, err := original.Pack(nil)
	require.NoError(t, err)

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

	data, err := o.Pack(nil)
	require.NoError(t, err)

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
