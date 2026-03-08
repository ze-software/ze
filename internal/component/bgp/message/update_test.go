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

// TestMessageUpdateUsesShared verifies UnpackUpdate uses wire.ParseUpdateSections.
//
// VALIDATES: UnpackUpdate delegates to shared parser for consistent behavior.
// PREVENTS: Divergent parsing logic between message.Update and WireUpdate.
func TestMessageUpdateUsesShared(t *testing.T) {
	// Build UPDATE with all sections to exercise full parsing
	withdrawn := []byte{0x10, 0x0a} // /16 prefix
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	nlri := []byte{0x18, 0xc0, 0xa8, 0x01} // /24 prefix

	body := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(nlri))
	body[0] = 0x00
	body[1] = byte(len(withdrawn))
	copy(body[2:], withdrawn)
	offset := 2 + len(withdrawn)
	body[offset] = 0x00
	body[offset+1] = byte(len(attrs))
	copy(body[offset+2:], attrs)
	copy(body[offset+2+len(attrs):], nlri)

	msg, err := UnpackUpdate(body)
	require.NoError(t, err)

	// Verify sections extracted correctly (proves shared parser works)
	assert.Equal(t, withdrawn, msg.WithdrawnRoutes, "withdrawn mismatch")
	assert.Equal(t, attrs, msg.PathAttributes, "attrs mismatch")
	assert.Equal(t, nlri, msg.NLRI, "nlri mismatch")

	// Verify zero-copy: sections should share backing array with body
	if len(msg.WithdrawnRoutes) > 0 {
		msg.WithdrawnRoutes[0] = 0xEE
		assert.Equal(t, byte(0xEE), body[2], "WithdrawnRoutes should be zero-copy")
		body[2] = withdrawn[0] // restore
	}
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

	data := PackTo(u, nil)

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

	data := PackTo(original, nil)

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

// TestUpdateWriteTo verifies zero-allocation buffer writing.
//
// VALIDATES: WriteTo produces identical bytes to PackTo.
//
// PREVENTS: Wire format mismatch between WriteTo and PackTo paths.
func TestUpdateWriteTo(t *testing.T) {
	tests := []struct {
		name   string
		update *Update
	}{
		{
			name:   "empty (EOR)",
			update: &Update{},
		},
		{
			name: "withdrawn only",
			update: &Update{
				WithdrawnRoutes: []byte{0x08, 0x0A}, // 10.0.0.0/8
			},
		},
		{
			name: "NLRI only",
			update: &Update{
				NLRI: []byte{0x18, 0xC0, 0xA8, 0x01}, // 192.168.1.0/24
			},
		},
		{
			name: "full update",
			update: &Update{
				WithdrawnRoutes: []byte{0x08, 0x0A},
				PathAttributes:  []byte{0x40, 0x01, 0x01, 0x00}, // ORIGIN IGP
				NLRI:            []byte{0x18, 0xC0, 0xA8, 0x01},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get expected from PackTo
			expected := PackTo(tt.update, nil)

			// Use WriteTo with pre-allocated buffer
			buf := make([]byte, 4096)
			n := tt.update.WriteTo(buf, 0, nil)

			assert.Equal(t, len(expected), n, "length mismatch")
			assert.Equal(t, expected, buf[:n], "content mismatch")
		})
	}
}

// TestUpdateLenMatchesWriteTo verifies Len() returns exact bytes WriteTo will use.
//
// VALIDATES: Len() accurately predicts WriteTo output size.
//
// PREVENTS: Buffer overflow from undersized allocation.
func TestUpdateLenMatchesWriteTo(t *testing.T) {
	tests := []struct {
		name   string
		update *Update
	}{
		{
			name:   "empty",
			update: &Update{},
		},
		{
			name: "NLRI only",
			update: &Update{
				NLRI: []byte{0x18, 0xC0, 0xA8, 0x01}, // 192.168.1.0/24
			},
		},
		{
			name: "withdrawn only",
			update: &Update{
				WithdrawnRoutes: []byte{0x18, 0xC0, 0xA8, 0x02}, // 192.168.2.0/24
			},
		},
		{
			name: "attributes only",
			update: &Update{
				PathAttributes: []byte{0x40, 0x01, 0x01, 0x00}, // ORIGIN IGP
			},
		},
		{
			name: "full update",
			update: &Update{
				WithdrawnRoutes: []byte{0x18, 0xC0, 0xA8, 0x02},
				PathAttributes:  []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00},
				NLRI:            []byte{0x18, 0xC0, 0xA8, 0x01},
			},
		},
		{
			name: "large update",
			update: &Update{
				WithdrawnRoutes: make([]byte, 100),
				PathAttributes:  make([]byte, 500),
				NLRI:            make([]byte, 200),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedLen := tt.update.Len(nil)

			buf := make([]byte, 65536)
			n := tt.update.WriteTo(buf, 0, nil)

			assert.Equal(t, expectedLen, n,
				"Len()=%d but WriteTo()=%d", expectedLen, n)
		})
	}
}

// TestUpdateWriteToOffset verifies WriteTo with non-zero offset.
//
// VALIDATES: WriteTo respects offset parameter.
//
// PREVENTS: Buffer corruption when writing at non-zero offset.
func TestUpdateWriteToOffset(t *testing.T) {
	u := &Update{
		NLRI: []byte{0x18, 0xC0, 0xA8, 0x01},
	}

	expected := PackTo(u, nil)

	// Write at offset 100
	buf := make([]byte, 4096)
	offset := 100
	n := u.WriteTo(buf, offset, nil)

	assert.Equal(t, len(expected), n, "length mismatch")
	assert.Equal(t, expected, buf[offset:offset+n], "content mismatch")

	// Verify bytes before offset are untouched
	for i := range offset {
		assert.Equal(t, byte(0), buf[i], "byte %d should be untouched", i)
	}
}
