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
	for i := 0; i < 100; i++ {
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
	var reassembled []byte
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
	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, nlri, reassembled)
}
