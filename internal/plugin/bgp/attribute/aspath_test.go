package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestASPathSegmentTypes verifies segment type constants match RFC values.
//
// RFC 4271 Section 4.3: AS_SET=1, AS_SEQUENCE=2
// RFC 5065 Section 3: AS_CONFED_SEQUENCE=3, AS_CONFED_SET=4
//
// VALIDATES: Constants match RFC-defined wire format values.
//
// PREVENTS: Encoding wrong segment types, causing interoperability failures.
func TestASPathSegmentTypes(t *testing.T) {
	assert.Equal(t, uint8(1), uint8(ASSet))
	assert.Equal(t, uint8(2), uint8(ASSequence))
	assert.Equal(t, uint8(3), uint8(ASConfedSequence)) // RFC 5065
	assert.Equal(t, uint8(4), uint8(ASConfedSet))      // RFC 5065
}

func TestASPathEmpty(t *testing.T) {
	path := &ASPath{}
	assert.Equal(t, 0, path.Len())
	buf := make([]byte, 64)
	n := path.WriteTo(buf, 0)
	assert.Equal(t, 0, n)
	assert.Equal(t, 0, path.PathLength())
}

func TestASPathSimpleSequence(t *testing.T) {
	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: []uint32{65001, 65002, 65003}},
		},
	}

	assert.Equal(t, 3, path.PathLength())

	// WriteTo: type(1) + count(1) + 3×4 bytes = 14 bytes
	buf := make([]byte, 64)
	n := path.WriteTo(buf, 0)
	assert.Equal(t, 14, n)
	assert.Equal(t, byte(ASSequence), buf[0])
	assert.Equal(t, byte(3), buf[1])
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

	buf := make([]byte, 4096)
	n := original.WriteTo(buf, 0)
	packed := buf[:n]
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
		asns[i] = uint32(i + 1) // #nosec G115 -- test values in range
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

// TestASPathWriteToAutoSplit verifies segments are split during encoding.
//
// RFC 4271 Section 4.3: Segment length is a 1-octet field, meaning max 255 ASNs.
//
// VALIDATES: Segments with >255 ASNs are split during WriteTo.
//
// PREVENTS: Encoding invalid segments with length > 255.
func TestASPathWriteToAutoSplit(t *testing.T) {
	// Create a segment with 300 ASNs (exceeds 255)
	asns := make([]uint32, 300)
	for i := range asns {
		asns[i] = uint32(i + 1) // #nosec G115 -- test values in range
	}

	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: asns},
		},
	}

	buf := make([]byte, 4096)
	n := path.WriteTo(buf, 0)
	packed := buf[:n]

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

// TestASPathWriteToAutoSplitLarge verifies multiple splits for very large segments.
//
// VALIDATES: Segments that need 3+ splits work correctly.
//
// PREVENTS: Edge case bugs in recursive/iterative splitting.
func TestASPathWriteToAutoSplitLarge(t *testing.T) {
	// Create a segment with 600 ASNs (needs 3 segments: 255+255+90)
	asns := make([]uint32, 600)
	for i := range asns {
		asns[i] = uint32(i + 1) // #nosec G115 -- test values in range
	}

	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: asns},
		},
	}

	buf := make([]byte, 8192)
	n := path.WriteTo(buf, 0)
	packed := buf[:n]

	// Parse it back
	parsed, err := ParseASPath(packed, true)
	require.NoError(t, err)
	require.Len(t, parsed.Segments, 3, "should split into 3 segments")
	assert.Len(t, parsed.Segments[0].ASNs, 255)
	assert.Len(t, parsed.Segments[1].ASNs, 255)
	assert.Len(t, parsed.Segments[2].ASNs, 90)
}

// TestParseASPathInvalidSegmentType verifies rejection of invalid segment types.
//
// RFC 4271 Section 4.3 defines segment types 1 (AS_SET) and 2 (AS_SEQUENCE).
// RFC 5065 adds types 3 (AS_CONFED_SET) and 4 (AS_CONFED_SEQUENCE).
// Any other segment type value is malformed per RFC 4271 Section 6.3.
//
// VALIDATES: Only segment types 1-4 are accepted per RFC 4271/5065.
//
// PREVENTS: Accepting malformed AS_PATH with invalid segment types,
// which could cause undefined behavior or interoperability issues.
func TestParseASPathInvalidSegmentType(t *testing.T) {
	tests := []struct {
		name    string
		segType byte
		wantErr bool
	}{
		{"type 0 invalid", 0, true},
		{"type 1 (AS_SET) valid", 1, false},
		{"type 2 (AS_SEQUENCE) valid", 2, false},
		{"type 3 (AS_CONFED_SET) valid", 3, false},
		{"type 4 (AS_CONFED_SEQUENCE) valid", 4, false},
		{"type 5 invalid", 5, true},
		{"type 127 invalid", 127, true},
		{"type 255 invalid", 255, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build AS path with single segment: type + count(1) + ASN(4 bytes)
			data := []byte{
				tt.segType,             // segment type
				0x01,                   // count = 1
				0x00, 0x00, 0xFD, 0xE9, // ASN 65001
			}

			_, err := ParseASPath(data, true)
			if tt.wantErr {
				require.Error(t, err, "expected error for segment type %d", tt.segType)
				require.ErrorIs(t, err, ErrMalformedASPath, "expected ErrMalformedASPath")
			} else {
				require.NoError(t, err, "expected no error for segment type %d", tt.segType)
			}
		})
	}
}

// TestParseASPathMaxLength verifies maximum path length enforcement.
//
// RFC 4271 does not specify a maximum total path length, but implementations
// should enforce a limit to prevent DoS attacks via extremely long paths.
// Real-world AS paths rarely exceed 50 ASNs.
//
// VALIDATES: Paths exceeding MaxASPathTotalLength are rejected.
//
// PREVENTS: Memory exhaustion and CPU abuse from extremely long AS paths
// that could be used for denial of service attacks.
func TestParseASPathMaxLength(t *testing.T) {
	// Build a path with exactly MaxASPathTotalLength ASNs using multiple segments
	// (since a single segment max is 255, we need 4 segments of 250 each = 1000)
	var data []byte
	totalASNs := 0
	for totalASNs < MaxASPathTotalLength {
		segmentSize := MaxASPathTotalLength - totalASNs
		if segmentSize > MaxASPathSegmentLength {
			segmentSize = MaxASPathSegmentLength
		}
		// Segment header: type + count
		segment := make([]byte, 2+segmentSize*4)
		segment[0] = byte(ASSequence)
		segment[1] = byte(segmentSize)
		// Fill with ASN values
		for i := 0; i < segmentSize; i++ {
			offset := 2 + i*4
			asnVal := totalASNs + i + 1
			segment[offset] = byte(asnVal >> 24)   //nolint:gosec // test values
			segment[offset+1] = byte(asnVal >> 16) //nolint:gosec // test values
			segment[offset+2] = byte(asnVal >> 8)  //nolint:gosec // test values
			segment[offset+3] = byte(asnVal)       //nolint:gosec // test values
		}
		data = append(data, segment...)
		totalASNs += segmentSize
	}

	_, err := ParseASPath(data, true)
	require.NoError(t, err, "path at MaxASPathTotalLength should be accepted")

	// Now add one more segment to exceed the limit
	extraSegment := []byte{
		byte(ASSequence),       // segment type
		0x01,                   // count = 1
		0x00, 0x00, 0xFF, 0xFF, // ASN 65535
	}
	data = append(data, extraSegment...)

	_, err = ParseASPath(data, true)
	require.Error(t, err, "path exceeding MaxASPathTotalLength should be rejected")
	require.ErrorIs(t, err, ErrMalformedASPath, "expected ErrMalformedASPath for oversized path")
}

// TestParseASPathEmptySegment verifies empty segments are accepted.
//
// RFC 4271 does not prohibit empty segments (count=0).
// ExaBGP also accepts empty segments.
//
// VALIDATES: Segments with count=0 are accepted.
//
// PREVENTS: False rejection of valid (albeit unusual) AS paths.
func TestParseASPathEmptySegment(t *testing.T) {
	// Empty segment: type + count(0)
	data := []byte{
		byte(ASSequence), // segment type
		0x00,             // count = 0
	}

	path, err := ParseASPath(data, true)
	require.NoError(t, err, "empty segment should be accepted")
	require.Len(t, path.Segments, 1)
	assert.Len(t, path.Segments[0].ASNs, 0)
}

// TestParseASPathConfederationTypes verifies confederation segment types are parsed correctly.
//
// RFC 5065 Section 3:
//   - AS_CONFED_SEQUENCE (3): ordered set of Member ASes in local confederation
//   - AS_CONFED_SET (4): unordered set of Member ASes in local confederation
//
// VALIDATES: Both confederation segment types parse with correct wire values.
//
// PREVENTS: Wire format mismatch causing interoperability failures with confederation peers.
func TestParseASPathConfederationTypes(t *testing.T) {
	tests := []struct {
		name     string
		wireType byte
		wantType ASPathSegmentType
	}{
		{
			name:     "AS_CONFED_SEQUENCE (type 3)",
			wireType: 0x03,
			wantType: ASConfedSequence,
		},
		{
			name:     "AS_CONFED_SET (type 4)",
			wireType: 0x04,
			wantType: ASConfedSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Segment with 2 confederation member ASNs
			data := []byte{
				tt.wireType,            // segment type
				0x02,                   // count = 2
				0x00, 0x00, 0xFC, 0x00, // 64512 (confederation member)
				0x00, 0x00, 0xFC, 0x01, // 64513 (confederation member)
			}

			path, err := ParseASPath(data, true)
			require.NoError(t, err)
			require.Len(t, path.Segments, 1)
			assert.Equal(t, tt.wantType, path.Segments[0].Type)
			assert.Equal(t, []uint32{64512, 64513}, path.Segments[0].ASNs)
		})
	}
}

// TestParseASPathConfederationPathLength verifies confederation segments don't count in path length.
//
// RFC 5065: Confederation segments are not counted in path length for route selection.
//
// VALIDATES: PathLength() excludes AS_CONFED_SEQUENCE and AS_CONFED_SET.
//
// PREVENTS: Incorrect route selection due to counting confederation hops.
func TestParseASPathConfederationPathLength(t *testing.T) {
	// Path with: AS_SEQUENCE(2) + AS_CONFED_SEQUENCE(3) + AS_CONFED_SET(2) + AS_SET(2)
	// Expected path length: 2 (sequence) + 0 (confed_seq) + 0 (confed_set) + 1 (set) = 3
	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: []uint32{65001, 65002}},
			{Type: ASConfedSequence, ASNs: []uint32{64512, 64513, 64514}},
			{Type: ASConfedSet, ASNs: []uint32{64520, 64521}},
			{Type: ASSet, ASNs: []uint32{65010, 65011}},
		},
	}

	assert.Equal(t, 3, path.PathLength(), "confed segments should not count")
}

// TestParseASPath2ByteValidation verifies segment type validation works in 2-byte ASN mode.
//
// RFC 4271 Section 4.3: Segment types 1-2 (extended by RFC 5065 to 1-4).
// Validation must work regardless of ASN size negotiation.
//
// VALIDATES: Invalid segment types rejected in 2-byte mode.
//
// PREVENTS: Validation bypass when speaking to OLD (2-byte) peers.
func TestParseASPath2ByteValidation(t *testing.T) {
	// Invalid segment type 5 with 2-byte ASNs
	data := []byte{
		0x05,       // invalid segment type
		0x01,       // count = 1
		0xFD, 0xE9, // 65001 (2-byte)
	}

	_, err := ParseASPath(data, false) // 2-byte mode
	require.Error(t, err)
	require.ErrorIs(t, err, ErrMalformedASPath)
}

// TestASPathWriteToWithASN4 verifies WriteToWithASN4 produces correct length and bytes.
//
// VALIDATES: WriteToWithASN4 writes expected bytes matching LenWithASN4.
//
// PREVENTS: Wire format errors in AS_PATH encoding.
func TestASPathWriteToWithASN4(t *testing.T) {
	tests := []struct {
		name string
		path *ASPath
		asn4 bool
	}{
		{
			name: "empty",
			path: &ASPath{},
			asn4: true,
		},
		{
			name: "simple sequence 4-byte",
			path: &ASPath{
				Segments: []ASPathSegment{
					{Type: ASSequence, ASNs: []uint32{65001, 65002, 65003}},
				},
			},
			asn4: true,
		},
		{
			name: "simple sequence 2-byte",
			path: &ASPath{
				Segments: []ASPathSegment{
					{Type: ASSequence, ASNs: []uint32{65001, 65002, 65003}},
				},
			},
			asn4: false,
		},
		{
			name: "multiple segments",
			path: &ASPath{
				Segments: []ASPathSegment{
					{Type: ASSequence, ASNs: []uint32{65001, 65002}},
					{Type: ASSet, ASNs: []uint32{65003, 65004}},
				},
			},
			asn4: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 4096)
			n := tt.path.WriteToWithASN4(buf, 0, tt.asn4)

			assert.Equal(t, tt.path.LenWithASN4(tt.asn4), n, "length mismatch with LenWithASN4")
		})
	}
}

// TestASPathWriteToExtendedLength4Byte verifies WriteTo handles >255 bytes (4-byte ASN).
//
// RFC 4271 Section 4.3: Extended Length flag (0x10) required when value > 255 bytes.
// With 4-byte ASNs: 255 bytes / 4 = 63.75, so >63 ASNs requires extended length.
// Actually: segment header is 2 bytes, so 85 ASNs = 2 + 85*4 = 342 bytes.
//
// VALIDATES: WriteTo handles large AS paths requiring extended length.
//
// PREVENTS: Length byte overflow causing malformed AS_PATH (bug found in code review).
func TestASPathWriteToExtendedLength4Byte(t *testing.T) {
	tests := []struct {
		name     string
		numASNs  int
		wantLen  int
		segments int
	}{
		{
			name:     "84 ASNs (under 255 segment limit)",
			numASNs:  84,
			wantLen:  2 + 84*4, // 338 bytes (1 segment)
			segments: 1,
		},
		{
			name:     "100 ASNs",
			numASNs:  100,
			wantLen:  2 + 100*4, // 402 bytes (1 segment)
			segments: 1,
		},
		{
			name:     "255 ASNs (max single segment)",
			numASNs:  255,
			wantLen:  2 + 255*4, // 1022 bytes (1 segment)
			segments: 1,
		},
		{
			name:     "256 ASNs (requires split)",
			numASNs:  256,
			wantLen:  2 + 255*4 + 2 + 1*4, // 1028 bytes (2 segments)
			segments: 2,
		},
		{
			name:     "300 ASNs (split into 255+45)",
			numASNs:  300,
			wantLen:  2 + 255*4 + 2 + 45*4, // 1204 bytes (2 segments)
			segments: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asns := make([]uint32, tt.numASNs)
			for i := range asns {
				asns[i] = uint32(65000 + i) //nolint:gosec // G115: test data, i bounded by small test values
			}

			path := &ASPath{
				Segments: []ASPathSegment{
					{Type: ASSequence, ASNs: asns},
				},
			}

			// Verify WriteTo produces correct length
			buf := make([]byte, 4096)
			n := path.WriteTo(buf, 0)
			packed := buf[:n]
			assert.Equal(t, tt.wantLen, n, "WriteTo length")

			// Parse back and verify segment count
			parsed, err := ParseASPath(packed, true)
			require.NoError(t, err)
			assert.Len(t, parsed.Segments, tt.segments, "segment count after split")

			// Verify all ASNs preserved
			totalASNs := 0
			for _, seg := range parsed.Segments {
				totalASNs += len(seg.ASNs)
			}
			assert.Equal(t, tt.numASNs, totalASNs, "total ASNs preserved")
		})
	}
}

// TestASPathWriteToExtendedLength2Byte verifies WriteTo handles >255 bytes (2-byte ASN).
//
// With 2-byte ASNs: segment can hold 255 ASNs = 2 + 255*2 = 512 bytes.
// Extended length needed when attribute value > 255 bytes, so >126 ASNs.
//
// VALIDATES: WriteTo handles large AS paths in 2-byte ASN mode.
//
// PREVENTS: Length byte overflow in legacy 2-byte ASN mode.
func TestASPathWriteToExtendedLength2Byte(t *testing.T) {
	tests := []struct {
		name    string
		numASNs int
		wantLen int
	}{
		{
			name:    "126 ASNs (254 bytes value)",
			numASNs: 126,
			wantLen: 2 + 126*2, // 254 bytes
		},
		{
			name:    "127 ASNs (256 bytes value - needs extended length)",
			numASNs: 127,
			wantLen: 2 + 127*2, // 256 bytes
		},
		{
			name:    "200 ASNs",
			numASNs: 200,
			wantLen: 2 + 200*2, // 402 bytes
		},
		{
			name:    "255 ASNs (max segment)",
			numASNs: 255,
			wantLen: 2 + 255*2, // 512 bytes
		},
		{
			name:    "300 ASNs (split)",
			numASNs: 300,
			wantLen: 2 + 255*2 + 2 + 45*2, // 604 bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asns := make([]uint32, tt.numASNs)
			for i := range asns {
				asns[i] = uint32(i + 1) //nolint:gosec // G115: test data, i bounded by small test values
			}

			path := &ASPath{
				Segments: []ASPathSegment{
					{Type: ASSequence, ASNs: asns},
				},
			}

			// Verify WriteToWithASN4
			buf := make([]byte, 4096)
			n := path.WriteToWithASN4(buf, 0, false)
			assert.Equal(t, tt.wantLen, n, "WriteTo length")
			assert.Equal(t, path.LenWithASN4(false), n, "WriteTo length should match LenWithASN4")
		})
	}
}

// TestASPathWriteToOffset verifies WriteTo respects offset parameter.
//
// VALIDATES: WriteTo writes at correct offset without corrupting adjacent data.
//
// PREVENTS: Buffer corruption when writing at non-zero offset.
func TestASPathWriteToOffset(t *testing.T) {
	path := &ASPath{
		Segments: []ASPathSegment{
			{Type: ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}

	// Get expected bytes by writing at offset 0
	ref := make([]byte, 4096)
	expectedLen := path.WriteTo(ref, 0)
	expected := ref[:expectedLen]

	offset := 100

	buf := make([]byte, 4096)
	// Pre-fill with sentinel value
	for i := range buf {
		buf[i] = 0xAA
	}

	n := path.WriteTo(buf, offset)

	assert.Equal(t, expectedLen, n)
	assert.Equal(t, expected, buf[offset:offset+n])

	// Verify bytes before offset are untouched
	for i := 0; i < offset; i++ {
		assert.Equal(t, byte(0xAA), buf[i], "byte %d should be untouched", i)
	}
}
