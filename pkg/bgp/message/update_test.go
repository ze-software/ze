package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateType verifies UPDATE message type.
func TestUpdateType(t *testing.T) {
	u := &Update{}
	assert.Equal(t, TypeUPDATE, u.Type())
}

// TestUpdateUnpackMinimal verifies minimal UPDATE (EOR).
//
// VALIDATES: End-of-RIB marker parsing.
//
// PREVENTS: EOR not recognized, causing session issues.
func TestUpdateUnpackMinimal(t *testing.T) {
	// Minimal UPDATE (End-of-RIB for IPv4 Unicast)
	// Withdrawn Length = 0, Attributes Length = 0
	body := []byte{
		0x00, 0x00, // Withdrawn Routes Length = 0
		0x00, 0x00, // Total Path Attribute Length = 0
		// No NLRI
	}

	msg, err := UnpackUpdate(body)
	require.NoError(t, err)

	assert.Equal(t, 0, len(msg.WithdrawnRoutes))
	assert.Equal(t, 0, len(msg.PathAttributes))
	assert.Equal(t, 0, len(msg.NLRI))
}

// TestUpdateUnpackWithWithdrawn verifies withdrawn routes parsing.
func TestUpdateUnpackWithWithdrawn(t *testing.T) {
	body := []byte{
		0x00, 0x05, // Withdrawn Routes Length = 5
		// Withdrawn prefix: 10.0.0.0/8
		0x08, // Prefix length = 8
		0x0A, // 10.x.x.x
		// Another: 192.168.0.0/16
		0x10,       // Prefix length = 16
		0xC0, 0xA8, // 192.168.x.x
		0x00, 0x00, // Total Path Attribute Length = 0
		// No NLRI
	}

	msg, err := UnpackUpdate(body)
	require.NoError(t, err)

	assert.Len(t, msg.WithdrawnRoutes, 5) // Raw bytes
}

// TestUpdateUnpackShort verifies short data handling.
func TestUpdateUnpackShort(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"withdrawn len only", []byte{0x00, 0x05}},
		{"no attr len", []byte{0x00, 0x00, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UnpackUpdate(tt.data)
			assert.ErrorIs(t, err, ErrShortRead)
		})
	}
}

// TestUpdatePackEmpty verifies empty UPDATE packing.
func TestUpdatePackEmpty(t *testing.T) {
	u := &Update{}

	data, err := u.Pack(nil)
	require.NoError(t, err)

	// Header (19) + WithdrawnLen (2) + AttrLen (2)
	assert.Len(t, data, HeaderLen+4)

	h, err := ParseHeader(data)
	require.NoError(t, err)
	assert.Equal(t, TypeUPDATE, h.Type)
}

// TestUpdateRoundTrip verifies pack/unpack symmetry.
func TestUpdateRoundTrip(t *testing.T) {
	original := &Update{
		WithdrawnRoutes: []byte{0x08, 0x0A}, // 10.0.0.0/8
		PathAttributes:  []byte{},
		NLRI:            []byte{0x18, 0xC0, 0xA8, 0x01}, // 192.168.1.0/24
	}

	data, err := original.Pack(nil)
	require.NoError(t, err)

	body := data[HeaderLen:]
	parsed, err := UnpackUpdate(body)
	require.NoError(t, err)

	assert.Equal(t, original.WithdrawnRoutes, parsed.WithdrawnRoutes)
	assert.Equal(t, original.NLRI, parsed.NLRI)
}

// TestUpdatePassthrough verifies raw data preservation.
//
// VALIDATES: Unchanged messages can be forwarded without re-parsing.
//
// PREVENTS: Unnecessary repacking causing CPU overhead.
func TestUpdatePassthrough(t *testing.T) {
	originalBody := []byte{
		0x00, 0x00, // Withdrawn = 0
		0x00, 0x05, // Attrs = 5
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x00,                   // Extra byte
		0x18, 0xC0, 0xA8, 0x01, // NLRI: 192.168.1.0/24
	}

	msg, err := UnpackUpdate(originalBody)
	require.NoError(t, err)

	// Passthrough should return original raw data
	raw := msg.RawData()
	assert.Equal(t, originalBody, raw)
}
