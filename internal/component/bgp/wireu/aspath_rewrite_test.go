package wireu

import (
	"encoding/binary"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildPayload constructs an UPDATE payload from parts.
// UPDATE body: wdLen(2) + withdrawn(wdLen) + attrLen(2) + attrs(attrLen) + nlri.
func buildPayload(withdrawn, attrs, nlri []byte) []byte {
	payload := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn))) //nolint:gosec // test data
	copy(payload[2:], withdrawn)
	off := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(payload[off:off+2], uint16(len(attrs))) //nolint:gosec // test data
	copy(payload[off+2:], attrs)
	copy(payload[off+2+len(attrs):], nlri)
	return payload
}

// buildASPathAttr constructs an AS_PATH attribute with given segments using ASN4 encoding.
// Each segment is (type, []ASN). Returns the complete attribute (header + value).
func buildASPathAttr(segments []attribute.ASPathSegment, asn4 bool) []byte { //nolint:unparam // asn4 is always true in current tests but parameter needed for correctness
	path := &attribute.ASPath{Segments: segments}
	valueLen := path.LenWithASN4(asn4)
	// Header: flags(1) + code(1) + length(1 or 2)
	hdrLen := 3
	if valueLen > 255 {
		hdrLen = 4
	}
	buf := make([]byte, hdrLen+valueLen)
	attribute.WriteHeaderTo(buf, 0, attribute.FlagTransitive, attribute.AttrASPath, uint16(valueLen)) //nolint:gosec // test data
	path.WriteToWithASN4(buf, hdrLen, asn4)
	return buf
}

// buildOriginAttr constructs a simple ORIGIN attribute (value=0 IGP).
func buildOriginAttr() []byte {
	// Flags=0x40 (transitive), Code=1 (ORIGIN), Len=1, Value=0 (IGP)
	return []byte{0x40, 0x01, 0x01, 0x00}
}

// concatAttrs concatenates attribute byte slices into a single attrs section.
func concatAttrs(parts ...[]byte) []byte {
	size := 0
	for _, p := range parts {
		size += len(p)
	}
	buf := make([]byte, 0, size)
	for _, p := range parts {
		buf = append(buf, p...)
	}
	return buf
}

// parseASPathFromPayload extracts and parses the AS_PATH from a rewritten payload.
func parseASPathFromPayload(t *testing.T, payload []byte, asn4 bool) *attribute.ASPath {
	t.Helper()
	require.True(t, len(payload) >= 4, "payload too short")

	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + wdLen
	require.True(t, len(payload) >= attrLenOff+2, "payload too short for attrLen")

	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrsStart := attrLenOff + 2
	require.True(t, len(payload) >= attrsStart+attrLen, "payload too short for attrs")

	// Scan attrs to find AS_PATH
	off := attrsStart
	for off < attrsStart+attrLen {
		flags, code, length, hl, err := attribute.ParseHeader(payload[off:])
		require.NoError(t, err, "parse attr header")
		_ = flags
		if code == attribute.AttrASPath {
			value := payload[off+hl : off+hl+int(length)]
			path, err := attribute.ParseASPath(value, asn4)
			require.NoError(t, err, "parse AS_PATH value")
			return path
		}
		off += hl + int(length)
	}
	t.Fatal("AS_PATH not found in payload")
	return nil
}

// TestRewriteASPath_ExistingSequenceASN4 verifies prepending to an existing
// AS_SEQUENCE segment with ASN4 encoding (src and dst both ASN4).
//
// VALIDATES: AC-1 — 65000 prepended as first ASN (4-byte); shift = +4; attrLen updated.
// PREVENTS: Wrong prepend position or incorrect length update.
func TestRewriteASPath_ExistingSequenceASN4(t *testing.T) {
	// Build: ORIGIN + AS_PATH with AS_SEQUENCE [64512, 64513]
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 64513}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+64)
	n, err := RewriteASPath(dst, payload, 65000, true, true)
	require.NoError(t, err)
	result := dst[:n]

	// Parse back and verify
	path := parseASPathFromPayload(t, result, true)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, attribute.ASSequence, path.Segments[0].Type)
	assert.Equal(t, []uint32{65000, 64512, 64513}, path.Segments[0].ASNs)

	// Verify shift = +4 (one 4-byte ASN added)
	assert.Equal(t, len(payload)+4, n, "result should be 4 bytes longer than original")
}

// TestRewriteASPath_ExistingSequenceASN2 verifies ASN4→ASN2 transcoding
// with prepend.
//
// VALIDATES: AC-2 — 65000 prepended (2-byte); existing ASNs transcoded from 4→2 byte.
// PREVENTS: Wrong encoding mode or failure to transcode existing ASNs.
func TestRewriteASPath_ExistingSequenceASN2(t *testing.T) {
	// Build with ASN4: AS_SEQUENCE [64512, 64513]
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 64513}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+64)
	n, err := RewriteASPath(dst, payload, 65000, true, false) // src=ASN4, dst=ASN2
	require.NoError(t, err)
	result := dst[:n]

	// Parse back with ASN2
	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, attribute.ASSequence, path.Segments[0].Type)
	// All ASNs stored as uint32 internally, but encoded as 2-byte on wire
	assert.Equal(t, []uint32{65000, 64512, 64513}, path.Segments[0].ASNs)
}

// TestRewriteASPath_NoASPath verifies inserting an AS_PATH when none exists.
//
// VALIDATES: AC-3 — Full AS_PATH attribute inserted; attrLen updated.
// PREVENTS: Crash or incorrect insertion when UPDATE has no AS_PATH.
func TestRewriteASPath_NoASPath(t *testing.T) {
	// Build: only ORIGIN, no AS_PATH
	origin := buildOriginAttr()
	payload := buildPayload(nil, origin, nil)

	dst := make([]byte, len(payload)+64)
	n, err := RewriteASPath(dst, payload, 65000, true, true)
	require.NoError(t, err)
	result := dst[:n]

	// Parse back and verify AS_PATH was inserted
	path := parseASPathFromPayload(t, result, true)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, attribute.ASSequence, path.Segments[0].Type)
	assert.Equal(t, []uint32{65000}, path.Segments[0].ASNs)
}

// TestRewriteASPath_FirstSegmentIsSet verifies that when the first segment
// is AS_SET, a new AS_SEQUENCE segment is prepended before it.
//
// VALIDATES: AC-4 — New AS_SEQUENCE{65000} segment prepended before AS_SET.
// PREVENTS: Incorrectly inserting into AS_SET (which is unordered).
func TestRewriteASPath_FirstSegmentIsSet(t *testing.T) {
	// Build: AS_PATH with AS_SET [64512, 64513]
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSet, ASNs: []uint32{64512, 64513}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+64)
	n, err := RewriteASPath(dst, payload, 65000, true, true)
	require.NoError(t, err)
	result := dst[:n]

	path := parseASPathFromPayload(t, result, true)
	require.Len(t, path.Segments, 2)
	// First segment should be new AS_SEQUENCE with our ASN
	assert.Equal(t, attribute.ASSequence, path.Segments[0].Type)
	assert.Equal(t, []uint32{65000}, path.Segments[0].ASNs)
	// Original AS_SET preserved
	assert.Equal(t, attribute.ASSet, path.Segments[1].Type)
	assert.Equal(t, []uint32{64512, 64513}, path.Segments[1].ASNs)
}

// TestRewriteASPath_FullSequence255 verifies that when the first AS_SEQUENCE
// is at max capacity (255), a new segment is created.
//
// VALIDATES: AC-5 — New AS_SEQUENCE{65000} prepended when existing segment is full.
// PREVENTS: Buffer overrun from exceeding 255 ASNs per segment.
func TestRewriteASPath_FullSequence255(t *testing.T) {
	// Build AS_SEQUENCE with exactly 255 ASNs
	asns := make([]uint32, 255)
	for i := range asns {
		asns[i] = uint32(100 + i) //nolint:gosec // test data
	}
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: asns},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+64)
	n, err := RewriteASPath(dst, payload, 65000, true, true)
	require.NoError(t, err)
	result := dst[:n]

	path := parseASPathFromPayload(t, result, true)
	require.GreaterOrEqual(t, len(path.Segments), 2)
	// First segment should be new with our ASN
	assert.Equal(t, attribute.ASSequence, path.Segments[0].Type)
	assert.Contains(t, path.Segments[0].ASNs, uint32(65000))
}

// TestRewriteASPath_ASTransEncoding verifies that when localASN > 65535
// and dstAsn4=false, AS_TRANS (23456) is used per RFC 6793.
//
// VALIDATES: AC-6 — localASN=70000 with dstAsn4=false encodes AS_TRANS=23456.
// PREVENTS: Large ASN corruption in 2-byte mode.
func TestRewriteASPath_ASTransEncoding(t *testing.T) {
	// Build with ASN4: AS_SEQUENCE [64512]
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+64)
	n, err := RewriteASPath(dst, payload, 70000, true, false) // 70000 > 65535, dst=ASN2
	require.NoError(t, err)
	result := dst[:n]

	// Parse with ASN2 — 70000 should be encoded as 23456 (AS_TRANS)
	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, attribute.ASSequence, path.Segments[0].Type)
	// First ASN should be 23456 (AS_TRANS) since 70000 > 65535
	assert.Equal(t, uint32(23456), path.Segments[0].ASNs[0], "large ASN should be AS_TRANS in ASN2 mode")
	assert.Equal(t, uint32(64512), path.Segments[0].ASNs[1])
}

// TestRewriteASPath_LengthsCorrect verifies that both the per-attribute
// length and global attrLen fields are correctly updated after rewrite.
//
// VALIDATES: attrLen and per-attr length both updated.
// PREVENTS: Mismatched lengths causing parse failures downstream.
func TestRewriteASPath_LengthsCorrect(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	origAttrLen := int(binary.BigEndian.Uint16(payload[2:4])) // wdLen=0, so attrLen at [2:4]

	dst := make([]byte, len(payload)+64)
	n, err := RewriteASPath(dst, payload, 65000, true, true)
	require.NoError(t, err)
	result := dst[:n]

	// attrLen should have increased by 4 (one ASN4 added)
	newAttrLen := int(binary.BigEndian.Uint16(result[2:4]))
	assert.Equal(t, origAttrLen+4, newAttrLen, "global attrLen should increase by shift")

	// The total result should parse without error
	path := parseASPathFromPayload(t, result, true)
	require.NotNil(t, path)
}

// TestRewriteASPath_RoundTrip verifies that a patched payload can be
// parsed back to produce the expected AS_PATH with localASN first.
//
// VALIDATES: Patched payload parses back with localASN first in AS_PATH.
// PREVENTS: Corruption of non-AS_PATH attributes or NLRI during rewrite.
func TestRewriteASPath_RoundTrip(t *testing.T) {
	// Build with ORIGIN + AS_PATH + some NLRI bytes
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 64513, 64514}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	testNLRI := []byte{24, 10, 0, 1} // /24 10.0.1.0
	payload := buildPayload(nil, attrs, testNLRI)

	dst := make([]byte, len(payload)+64)
	n, err := RewriteASPath(dst, payload, 65000, true, true)
	require.NoError(t, err)
	result := dst[:n]

	// Verify AS_PATH is correct
	path := parseASPathFromPayload(t, result, true)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{65000, 64512, 64513, 64514}, path.Segments[0].ASNs)

	// Verify NLRI is preserved: last 4 bytes should be the NLRI
	wdLen := int(binary.BigEndian.Uint16(result[0:2]))
	attrLenOff := 2 + wdLen
	attrLen := int(binary.BigEndian.Uint16(result[attrLenOff : attrLenOff+2]))
	nlriStart := attrLenOff + 2 + attrLen
	assert.Equal(t, testNLRI, result[nlriStart:], "NLRI should be preserved unchanged")
}

// TestRewriteASPath_Malformed verifies that malformed payloads return errors
// without panicking.
//
// VALIDATES: Malformed payload returns error, does not panic.
// PREVENTS: Panic or corruption on bad input.
func TestRewriteASPath_Malformed(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"too short", []byte{0, 0}},
		{"truncated attrLen", []byte{0, 0, 0}},
		{"truncated attr header", []byte{0, 0, 0, 3, 0x40, 0x02}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := make([]byte, 4096)
			_, err := RewriteASPath(dst, tt.payload, 65000, true, true)
			assert.Error(t, err, "should return error for malformed payload")
		})
	}
}

// TestRewriteASPath_WithWithdrawn verifies correct handling when the
// UPDATE contains withdrawn routes (non-zero wdLen).
//
// VALIDATES: Correct offset calculation with non-zero withdrawn length.
// PREVENTS: Off-by-one when wdLen != 0.
func TestRewriteASPath_WithWithdrawn(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	withdrawn := []byte{16, 10, 0} // /16 10.0.0.0/16

	payload := buildPayload(withdrawn, attrs, nil)

	dst := make([]byte, len(payload)+64)
	n, err := RewriteASPath(dst, payload, 65000, true, true)
	require.NoError(t, err)
	result := dst[:n]

	// Verify withdrawn is preserved
	wdLen := int(binary.BigEndian.Uint16(result[0:2]))
	assert.Equal(t, len(withdrawn), wdLen)
	assert.Equal(t, withdrawn, result[2:2+wdLen])

	// Verify AS_PATH
	path := parseASPathFromPayload(t, result, true)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{65000, 64512}, path.Segments[0].ASNs)
}

// FuzzRewriteASPath verifies RewriteASPath does not panic on arbitrary input.
// Fuzzes both srcAsn4/dstAsn4 combinations.
//
// VALIDATES: No panic on arbitrary input.
// PREVENTS: Panics from malformed wire data.
func FuzzRewriteASPath(f *testing.F) {
	// Seed with valid payloads
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	validAttrs := concatAttrs(origin, aspath)
	validPayload := buildPayload(nil, validAttrs, nil)

	f.Add(validPayload, uint32(65000), true, true)
	f.Add(validPayload, uint32(65000), true, false)
	f.Add([]byte{0, 0, 0, 0}, uint32(65000), true, true) // empty attrs
	f.Add([]byte{}, uint32(1), false, false)             // empty

	f.Fuzz(func(_ *testing.T, payload []byte, localASN uint32, srcAsn4, dstAsn4 bool) {
		if localASN == 0 {
			return // Reserved ASN, skip
		}
		dst := make([]byte, len(payload)+1024)
		// Must not panic — errors are expected for malformed input
		if _, err := RewriteASPath(dst, payload, localASN, srcAsn4, dstAsn4); err != nil {
			return // Errors are fine, only panics are bugs
		}
	})
}
