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

// TestChunkNLRI_SmallFits tests that small NLRI sets are not chunked.
//
// VALIDATES: Updates smaller than max size are returned as-is.
//
// PREVENTS: Unnecessary fragmentation of small updates.
func TestChunkNLRI_SmallFits(t *testing.T) {
	// Small NLRI that fits easily
	nlri := []byte{0x18, 0xC0, 0xA8, 0x01} // 192.168.1.0/24

	chunks := ChunkNLRI(nlri, 4096)

	assert.Len(t, chunks, 1, "small NLRI should not be chunked")
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkNLRI_LargeChunked tests splitting large NLRI sets.
//
// VALIDATES: RFC 4271 Section 4 - UPDATE must not exceed max message size.
// Large NLRI sets must be split across multiple UPDATE messages.
//
// PREVENTS: Oversized messages causing session reset or dropped routes.
func TestChunkNLRI_LargeChunked(t *testing.T) {
	// Create NLRI larger than maxSize (100 /24 prefixes = 400 bytes)
	// Use small maxSize (100 bytes) to force chunking
	var nlri []byte
	for i := range 100 {
		// Each /24 prefix is 4 bytes: length(1) + 3 bytes
		nlri = append(nlri, 0x18, 0xC0, 0xA8, byte(i))
	}

	// Chunk with small max to force splitting
	maxSize := 50
	chunks := ChunkNLRI(nlri, maxSize)

	// Should have multiple chunks
	assert.Greater(t, len(chunks), 1, "large NLRI should be chunked")

	// Each chunk should be <= maxSize
	for i, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk), maxSize,
			"chunk %d exceeds maxSize: %d > %d", i, len(chunk), maxSize)
	}

	// Reassembled chunks should equal original
	reassembled := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, nlri, reassembled, "chunked NLRI should reassemble to original")
}

// TestChunkNLRI_ExactBoundary tests chunking at exact boundary.
//
// VALIDATES: Correct handling when NLRI exactly fills the chunk.
//
// PREVENTS: Off-by-one errors at chunk boundaries.
func TestChunkNLRI_ExactBoundary(t *testing.T) {
	// 4 prefixes of 4 bytes each = 16 bytes
	nlri := []byte{
		0x18, 0x0A, 0x00, 0x01, // 10.0.1.0/24
		0x18, 0x0A, 0x00, 0x02, // 10.0.2.0/24
		0x18, 0x0A, 0x00, 0x03, // 10.0.3.0/24
		0x18, 0x0A, 0x00, 0x04, // 10.0.4.0/24
	}

	// Max size of 8 bytes = exactly 2 prefixes per chunk
	chunks := ChunkNLRI(nlri, 8)

	assert.Len(t, chunks, 2, "should split into exactly 2 chunks")
	assert.Len(t, chunks[0], 8)
	assert.Len(t, chunks[1], 8)
}

// TestChunkNLRI_SinglePrefixTooLarge tests error for oversized prefix.
//
// VALIDATES: Graceful handling of edge case where single prefix exceeds max.
//
// PREVENTS: Panic or infinite loop when a single prefix is too large.
func TestChunkNLRI_SinglePrefixTooLarge(t *testing.T) {
	// Single /24 prefix = 4 bytes
	nlri := []byte{0x18, 0xC0, 0xA8, 0x01}

	// Max size smaller than single prefix
	chunks := ChunkNLRI(nlri, 2)

	// Should still return the prefix (best effort) rather than panic
	// Implementation should warn/log but not fail
	assert.Len(t, chunks, 1, "oversized single prefix should still be returned")
}

// TestChunkNLRI_Empty tests empty NLRI handling.
func TestChunkNLRI_Empty(t *testing.T) {
	chunks := ChunkNLRI(nil, 4096)
	assert.Len(t, chunks, 0, "empty NLRI should produce no chunks")

	chunks = ChunkNLRI([]byte{}, 4096)
	assert.Len(t, chunks, 0, "empty NLRI should produce no chunks")
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

// TestChunkNLRI_VariablePrefixLengths tests chunking with mixed prefix sizes.
//
// VALIDATES: Correct handling of variable-length prefixes.
//
// PREVENTS: Incorrect byte counting for different prefix lengths.
func TestChunkNLRI_VariablePrefixLengths(t *testing.T) {
	// Mixed prefix lengths:
	// /8 = 2 bytes (len + 1 byte prefix)
	// /16 = 3 bytes (len + 2 bytes prefix)
	// /24 = 4 bytes (len + 3 bytes prefix)
	// /32 = 5 bytes (len + 4 bytes prefix)
	nlri := []byte{
		0x08, 0x0A, // 10.0.0.0/8 (2 bytes)
		0x10, 0xAC, 0x10, // 172.16.0.0/16 (3 bytes)
		0x18, 0xC0, 0xA8, 0x01, // 192.168.1.0/24 (4 bytes)
		0x20, 0x08, 0x08, 0x08, 0x08, // 8.8.8.8/32 (5 bytes)
	}

	// Max 7 bytes: first prefix (2) + second (3) = 5, then 4+5 for rest
	chunks := ChunkNLRI(nlri, 7)

	// Verify all chunks are within size limit
	for i, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk), 7,
			"chunk %d exceeds maxSize", i)
	}

	// Reassemble and verify
	reassembled := make([]byte, 0, len(nlri))
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, nlri, reassembled)
}
