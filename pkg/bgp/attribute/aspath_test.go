package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASPathSegmentTypes(t *testing.T) {
	assert.Equal(t, uint8(1), uint8(ASSet))
	assert.Equal(t, uint8(2), uint8(ASSequence))
	assert.Equal(t, uint8(3), uint8(ASConfedSet))
	assert.Equal(t, uint8(4), uint8(ASConfedSequence))
}

func TestASPathEmpty(t *testing.T) {
	path := &ASPath{}
	assert.Equal(t, 0, path.Len())
	assert.Equal(t, []byte{}, path.Pack())
	assert.Equal(t, 0, path.PathLength())
}

func TestASPathSimpleSequence(t *testing.T) {
	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: []uint32{65001, 65002, 65003}},
		},
	}

	assert.Equal(t, 3, path.PathLength())

	// Pack: type(1) + count(1) + 3×4 bytes = 14 bytes
	packed := path.Pack()
	assert.Equal(t, 14, len(packed))
	assert.Equal(t, byte(ASSequence), packed[0])
	assert.Equal(t, byte(3), packed[1])
}

func TestASPathParse4Byte(t *testing.T) {
	// AS_SEQUENCE with 2 ASNs: [65001, 65002]
	data := []byte{
		0x02,                   // AS_SEQUENCE
		0x02,                   // count = 2
		0x00, 0x00, 0xFD, 0xE9, // 65001
		0x00, 0x00, 0xFD, 0xEA, // 65002
	}

	path, err := ParseASPath(data, true) // 4-byte ASN
	require.NoError(t, err)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, ASSequence, path.Segments[0].Type)
	assert.Equal(t, []uint32{65001, 65002}, path.Segments[0].ASNs)
}

func TestASPathParse2Byte(t *testing.T) {
	// AS_SEQUENCE with 2 ASNs: [65001, 65002] (2-byte format)
	data := []byte{
		0x02,       // AS_SEQUENCE
		0x02,       // count = 2
		0xFD, 0xE9, // 65001
		0xFD, 0xEA, // 65002
	}

	path, err := ParseASPath(data, false) // 2-byte ASN
	require.NoError(t, err)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{65001, 65002}, path.Segments[0].ASNs)
}

func TestASPathMultipleSegments(t *testing.T) {
	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: []uint32{65001, 65002}},
			{Type: ASSet, ASNs: []uint32{65003, 65004, 65005}},
		},
	}

	// Path length counts all ASNs in SEQUENCEs plus 1 for SET
	// = 2 (sequence) + 1 (set counts as 1) = 3
	assert.Equal(t, 3, path.PathLength())
}

func TestASPathParseEmpty(t *testing.T) {
	path, err := ParseASPath([]byte{}, true)
	require.NoError(t, err)
	assert.Len(t, path.Segments, 0)
}

func TestASPathParseErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"short header", []byte{0x02}},
		{"count mismatch", []byte{0x02, 0x05, 0x00, 0x00, 0x00, 0x01}}, // says 5, has 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseASPath(tt.data, true)
			require.Error(t, err)
		})
	}
}

func TestASPathRoundTrip(t *testing.T) {
	original := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: []uint32{65001, 65002, 65003}},
			{Type: ASSet, ASNs: []uint32{65010, 65011}},
		},
	}

	packed := original.Pack()
	parsed, err := ParseASPath(packed, true)

	require.NoError(t, err)
	assert.Equal(t, original.Segments, parsed.Segments)
}

func TestASPathInterface(t *testing.T) {
	var attr Attribute = &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: []uint32{65001}},
		},
	}

	assert.Equal(t, AttrASPath, attr.Code())
	assert.Equal(t, FlagTransitive, attr.Flags())
}

func TestASPathContains(t *testing.T) {
	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: []uint32{65001, 65002}},
			{Type: ASSet, ASNs: []uint32{65003}},
		},
	}

	assert.True(t, path.Contains(65001))
	assert.True(t, path.Contains(65002))
	assert.True(t, path.Contains(65003))
	assert.False(t, path.Contains(65004))
}

func TestASPathPrepend(t *testing.T) {
	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: []uint32{65002, 65003}},
		},
	}

	path.Prepend(65001)

	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{65001, 65002, 65003}, path.Segments[0].ASNs)
}

// TestASPathPrependOverflow verifies RFC 4271 segment overflow handling.
//
// RFC 4271 Section 5.1.2:
//
//	"If the act of prepending will cause an overflow in the AS_PATH segment
//	 (i.e., more than 255 ASes), it SHOULD prepend a new segment of type
//	 AS_SEQUENCE and prepend its own AS number to this new segment."
//
// VALIDATES: Prepending to a full segment creates a new segment.
//
// PREVENTS: Segment length overflow causing malformed AS_PATH.
func TestASPathPrependOverflow(t *testing.T) {
	// Create a segment with 255 ASNs (max)
	asns := make([]uint32, MaxASPathSegmentLength)
	for i := range asns {
		asns[i] = uint32(i + 1)
	}

	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: asns},
		},
	}

	// Prepend should create a new segment
	path.Prepend(65001)

	require.Len(t, path.Segments, 2, "should have 2 segments after prepend overflow")
	assert.Len(t, path.Segments[0].ASNs, 1, "new segment should have 1 ASN")
	assert.Equal(t, uint32(65001), path.Segments[0].ASNs[0])
	assert.Equal(t, ASSequence, path.Segments[0].Type)
	assert.Len(t, path.Segments[1].ASNs, MaxASPathSegmentLength, "original segment unchanged")
}

// TestASPathPackAutoSplit verifies segments are split during encoding.
//
// RFC 4271 Section 4.3: Segment length is a 1-octet field, meaning max 255 ASNs.
//
// VALIDATES: Segments with >255 ASNs are split during Pack.
//
// PREVENTS: Encoding invalid segments with length > 255.
func TestASPathPackAutoSplit(t *testing.T) {
	// Create a segment with 300 ASNs (exceeds 255)
	asns := make([]uint32, 300)
	for i := range asns {
		asns[i] = uint32(i + 1)
	}

	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: asns},
		},
	}

	packed := path.Pack()

	// Parse it back - should have 2 segments now
	parsed, err := ParseASPath(packed, true)
	require.NoError(t, err)
	require.Len(t, parsed.Segments, 2, "should split into 2 segments")
	assert.Len(t, parsed.Segments[0].ASNs, 255, "first segment should have 255 ASNs")
	assert.Len(t, parsed.Segments[1].ASNs, 45, "second segment should have 45 ASNs")

	// Verify total ASNs preserved
	total := len(parsed.Segments[0].ASNs) + len(parsed.Segments[1].ASNs)
	assert.Equal(t, 300, total)
}

// TestASPathPackAutoSplitLarge verifies multiple splits for very large segments.
//
// VALIDATES: Segments that need 3+ splits work correctly.
//
// PREVENTS: Edge case bugs in recursive/iterative splitting.
func TestASPathPackAutoSplitLarge(t *testing.T) {
	// Create a segment with 600 ASNs (needs 3 segments: 255+255+90)
	asns := make([]uint32, 600)
	for i := range asns {
		asns[i] = uint32(i + 1)
	}

	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: asns},
		},
	}

	packed := path.Pack()

	// Parse it back
	parsed, err := ParseASPath(packed, true)
	require.NoError(t, err)
	require.Len(t, parsed.Segments, 3, "should split into 3 segments")
	assert.Len(t, parsed.Segments[0].ASNs, 255)
	assert.Len(t, parsed.Segments[1].ASNs, 255)
	assert.Len(t, parsed.Segments[2].ASNs, 90)
}
