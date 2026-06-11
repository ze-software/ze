package wireu

import (
	"encoding/binary"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildAS4PathAttr constructs an AS4_PATH attribute (always 4-byte ASNs).
func buildAS4PathAttr(segments []attribute.ASPathSegment) []byte {
	path := &attribute.AS4Path{Segments: segments}
	valueLen := path.Len()
	hdrLen := 3
	if valueLen > 255 {
		hdrLen = 4
	}
	buf := make([]byte, hdrLen+valueLen)
	attribute.WriteHeaderTo(buf, 0,
		attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Path, uint16(valueLen)) //nolint:gosec // test data
	path.WriteTo(buf, hdrLen)
	return buf
}

// parseAS4PathFromPayload extracts and parses the AS4_PATH from a payload.
// Returns nil if no AS4_PATH attribute is found.
func parseAS4PathFromPayload(t *testing.T, payload []byte) *attribute.AS4Path {
	t.Helper()
	require.True(t, len(payload) >= 4, "payload too short")

	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + wdLen
	require.True(t, len(payload) >= attrLenOff+2, "payload too short for attrLen")

	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrsStart := attrLenOff + 2
	require.True(t, len(payload) >= attrsStart+attrLen, "payload too short for attrs")

	off := attrsStart
	for off < attrsStart+attrLen {
		_, code, length, hl, err := attribute.ParseHeader(payload[off:])
		require.NoError(t, err, "parse attr header")
		if code == attribute.AttrAS4Path {
			value := payload[off+hl : off+hl+int(length)]
			path, err := attribute.ParseAS4Path(value)
			require.NoError(t, err, "parse AS4_PATH value")
			return path
		}
		off += hl + int(length)
	}
	return nil
}

// TestTranscodeASPath_SameEncoding verifies no-op when srcASN4 == dstASN4.
//
// VALIDATES: TranscodeASPath returns 0 when no transcoding needed.
// PREVENTS: Unnecessary payload copy when source and destination use same encoding.
func TestTranscodeASPath_SameEncoding(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 64513}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, true)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "same encoding should return 0")

	n, err = TranscodeASPath(dst, payload, false, false)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "same encoding should return 0")
}

// TestTranscodeASPath_4to2_MappableOnly verifies ASN4→ASN2 transcoding
// when all ASNs fit in 2 bytes (no AS4_PATH needed).
//
// VALIDATES: AS_PATH re-encoded with 2-byte ASNs; no AS4_PATH added.
// PREVENTS: Unnecessary AS4_PATH insertion for mappable-only paths.
func TestTranscodeASPath_4to2_MappableOnly(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 64513}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AS_PATH should be re-encoded with 2-byte ASNs.
	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{64512, 64513}, path.Segments[0].ASNs)

	// No AS4_PATH needed (all ASNs <= 65535).
	as4 := parseAS4PathFromPayload(t, result)
	assert.Nil(t, as4, "AS4_PATH should not be present for mappable ASNs")
}

// TestTranscodeASPath_4to2_NonMappable verifies ASN4→ASN2 transcoding
// when ASNs > 65535 are present (requires AS4_PATH + AS_TRANS).
//
// VALIDATES: Non-mappable ASNs become AS_TRANS in AS_PATH; AS4_PATH carries originals.
// PREVENTS: Sending 4-byte AS_PATH wire bytes to a 2-byte-only peer (the original bug).
func TestTranscodeASPath_4to2_NonMappable(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 200000, 300000}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AS_PATH: non-mappable ASNs replaced with AS_TRANS (23456).
	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{64512, attribute.ASTrans, attribute.ASTrans}, path.Segments[0].ASNs)

	// AS4_PATH: original 4-byte values preserved.
	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4, "AS4_PATH must be present for non-mappable ASNs")
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, []uint32{64512, 200000, 300000}, as4.Segments[0].ASNs)
}

// TestTranscodeASPath_4to2_MixedSegments verifies transcoding with AS_SET segments.
//
// VALIDATES: Both AS_SEQUENCE and AS_SET segments are transcoded correctly.
// PREVENTS: Segment type corruption during transcoding.
func TestTranscodeASPath_4to2_MixedSegments(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 200000}},
		{Type: attribute.ASSet, ASNs: []uint32{300000, 64513}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	result := dst[:n]

	// AS_PATH with AS_TRANS substitutions.
	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 2)
	assert.Equal(t, attribute.ASSequence, path.Segments[0].Type)
	assert.Equal(t, []uint32{64512, attribute.ASTrans}, path.Segments[0].ASNs)
	assert.Equal(t, attribute.ASSet, path.Segments[1].Type)
	assert.Equal(t, []uint32{attribute.ASTrans, 64513}, path.Segments[1].ASNs)

	// AS4_PATH preserves original values.
	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4)
	require.Len(t, as4.Segments, 2)
	assert.Equal(t, []uint32{64512, 200000}, as4.Segments[0].ASNs)
	assert.Equal(t, []uint32{300000, 64513}, as4.Segments[1].ASNs)
}

// TestTranscodeASPath_4to2_StripsOldAS4Path verifies that an existing AS4_PATH
// attribute from the source is replaced by the newly generated one.
//
// VALIDATES: Stale AS4_PATH removed; fresh AS4_PATH with correct values inserted.
// PREVENTS: Duplicate or stale AS4_PATH attributes in forwarded UPDATE.
func TestTranscodeASPath_4to2_StripsOldAS4Path(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 200000}},
	}, true)
	// Stale AS4_PATH from a previous hop.
	oldAS4 := buildAS4PathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{99999}},
	})
	attrs := concatAttrs(origin, aspath, oldAS4)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	result := dst[:n]

	// AS4_PATH should have the NEW values, not the stale ones.
	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4)
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, []uint32{64512, 200000}, as4.Segments[0].ASNs)
}

// TestTranscodeASPath_4to2_PreservesOtherAttrs verifies that non-AS_PATH
// attributes and NLRI are preserved through transcoding.
//
// VALIDATES: ORIGIN and NLRI bytes are unchanged after transcoding.
// PREVENTS: Attribute or NLRI corruption during payload rebuild.
func TestTranscodeASPath_4to2_PreservesOtherAttrs(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	nlri := []byte{24, 10, 0, 0} // 10.0.0.0/24
	payload := buildPayload(nil, attrs, nlri)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	result := dst[:n]

	// NLRI should be preserved at end of payload.
	assert.Equal(t, nlri, result[n-len(nlri):n], "NLRI should be preserved")
}

// TestTranscodeASPath_NoASPath verifies behavior when UPDATE has no AS_PATH.
//
// VALIDATES: Payload copied unchanged when no AS_PATH present.
// PREVENTS: Crash on AS_PATH-less UPDATEs (e.g., End-of-RIB markers).
func TestTranscodeASPath_NoASPath(t *testing.T) {
	origin := buildOriginAttr()
	payload := buildPayload(nil, origin, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	assert.Equal(t, payload, dst[:n])
}

// TestTranscodeASPath_TruncatedPayload verifies error handling for malformed input.
//
// VALIDATES: Truncated payloads return an error, not a panic.
// PREVENTS: Out-of-bounds read on malformed wire data.
func TestTranscodeASPath_TruncatedPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{"empty", nil},
		{"too_short", []byte{0x00}},
		{"short_3_bytes", []byte{0x00, 0x00, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := make([]byte, 128)
			_, err := TranscodeASPath(dst, tt.payload, true, false)
			assert.Error(t, err)
		})
	}
}

// TestTranscodeASPath_BoundaryASN verifies AS_TRANS boundary at 65535/65536.
//
// VALIDATES: ASN 65535 maps to itself; ASN 65536 maps to AS_TRANS.
// PREVENTS: Off-by-one in the > 65535 threshold (RFC 6793 Section 9).
func TestTranscodeASPath_BoundaryASN(t *testing.T) {
	tests := []struct {
		name     string
		asn      uint32
		wantASN2 uint32
		wantAS4  bool
	}{
		{"last_mappable_65535", 65535, 65535, false},
		{"first_nonmappable_65536", 65536, attribute.ASTrans, true},
		{"large_4byte", 4200000000, attribute.ASTrans, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origin := buildOriginAttr()
			aspath := buildASPathAttr([]attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{tt.asn}},
			}, true)
			attrs := concatAttrs(origin, aspath)
			payload := buildPayload(nil, attrs, nil)

			dst := make([]byte, len(payload)+128)
			n, err := TranscodeASPath(dst, payload, true, false)
			require.NoError(t, err)
			result := dst[:n]

			path := parseASPathFromPayload(t, result, false)
			require.Len(t, path.Segments, 1)
			assert.Equal(t, []uint32{tt.wantASN2}, path.Segments[0].ASNs)

			as4 := parseAS4PathFromPayload(t, result)
			if tt.wantAS4 {
				require.NotNil(t, as4, "AS4_PATH expected")
				assert.Equal(t, []uint32{tt.asn}, as4.Segments[0].ASNs)
			} else {
				assert.Nil(t, as4, "AS4_PATH not expected")
			}
		})
	}
}

// TestTranscodeASPath_4to2_AS4PathBeforeASPath verifies correct handling when
// the existing AS4_PATH attribute appears before AS_PATH in the attribute list.
//
// VALIDATES: AS4_PATH before AS_PATH is stripped and replaced correctly.
// PREVENTS: Offset miscalculation when AS4_PATH precedes AS_PATH.
func TestTranscodeASPath_4to2_AS4PathBeforeASPath(t *testing.T) {
	origin := buildOriginAttr()
	oldAS4 := buildAS4PathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{99999}},
	})
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 200000}},
	}, true)
	// AS4_PATH before AS_PATH in attribute list.
	attrs := concatAttrs(origin, oldAS4, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	result := dst[:n]

	// Verify new AS4_PATH, not stale.
	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4)
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, []uint32{64512, 200000}, as4.Segments[0].ASNs)

	// Verify AS_PATH is transcoded.
	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{64512, attribute.ASTrans}, path.Segments[0].ASNs)
}
