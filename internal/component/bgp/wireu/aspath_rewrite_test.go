package wireu

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"

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
// and dstASN4=false, AS_TRANS (23456) is used per RFC 6793.
//
// VALIDATES: AC-6 — localASN=70000 with dstASN4=false encodes AS_TRANS=23456.
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

// TestRewriteASPathDual_ExistingSequence verifies dual-AS prepend to an
// existing AS_SEQUENCE. The primary ASN must end up closest to the peer
// (outermost) and the secondary ASN must sit between primary and existing.
//
// VALIDATES: cmd-2 local-as dual-AS mode AC-9/AC-13 baseline -- the default
// (no modifiers) prepends [override, real, ...existing].
// PREVENTS: Wrong prepend order that would mis-report the AS path topology
// to downstream peers when local-as override is set.
func TestRewriteASPathDual_ExistingSequence(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 64513}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+64)
	// primary = 65100 (override, closest to peer), secondary = 65000 (real behind)
	n, err := RewriteASPathDual(dst, payload, 65100, 65000, true, true)
	require.NoError(t, err)
	result := dst[:n]

	path := parseASPathFromPayload(t, result, true)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, attribute.ASSequence, path.Segments[0].Type)
	assert.Equal(t, []uint32{65100, 65000, 64512, 64513}, path.Segments[0].ASNs,
		"primary must be outermost, secondary behind it")

	// Verify shift = +8 (two 4-byte ASNs added)
	assert.Equal(t, len(payload)+8, n, "result should be 8 bytes longer than original")
}

// TestRewriteASPathDual_NoASPath verifies dual-AS insert when no AS_PATH exists.
// The inserted segment must be [primary, secondary] in order.
//
// VALIDATES: Insert path for dual-AS prepend produces the same ordering as
// prepend path.
// PREVENTS: Inconsistent ASN ordering between insert and prepend branches.
func TestRewriteASPathDual_NoASPath(t *testing.T) {
	origin := buildOriginAttr()
	payload := buildPayload(nil, origin, nil)

	dst := make([]byte, len(payload)+64)
	n, err := RewriteASPathDual(dst, payload, 65100, 65000, true, true)
	require.NoError(t, err)
	result := dst[:n]

	path := parseASPathFromPayload(t, result, true)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, attribute.ASSequence, path.Segments[0].Type)
	assert.Equal(t, []uint32{65100, 65000}, path.Segments[0].ASNs,
		"inserted segment must have primary at index 0 and secondary at index 1")
}

// TestRewriteASPathDual_ASN2 verifies dual-AS prepend in ASN2 encoding.
//
// VALIDATES: ASN4 to ASN2 transcoding still produces correct ordering
// for dual-prepend.
// PREVENTS: ASN2 peer seeing swapped primary/secondary order.
func TestRewriteASPathDual_ASN2(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	attrs := concatAttrs(origin, aspath)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+64)
	n, err := RewriteASPathDual(dst, payload, 65100, 65000, true, false) // dst=ASN2
	require.NoError(t, err)
	result := dst[:n]

	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{65100, 65000, 64512}, path.Segments[0].ASNs)
}

// FuzzRewriteASPath verifies RewriteASPath does not panic on arbitrary input.
// Fuzzes both srcASN4/dstASN4 combinations.
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

	f.Fuzz(func(_ *testing.T, payload []byte, localASN uint32, srcASN4, dstASN4 bool) {
		if localASN == 0 {
			return // Reserved ASN, skip
		}
		dst := make([]byte, len(payload)+1024)
		// Must not panic — errors are expected for malformed input
		if _, err := RewriteASPath(dst, payload, localASN, srcASN4, dstASN4); err != nil {
			return // Errors are fine, only panics are bugs
		}
	})
}

// TestRewriteASPath_AggregatorNonMappable verifies that RewriteASPath transcodes
// AGGREGATOR when crossing ASN4→ASN2 encoding and the aggregator ASN is non-mappable.
//
// VALIDATES: RFC 6793 Section 4.2.2: AGGREGATOR re-encoded with AS_TRANS; AS4_AGGREGATOR inserted.
// PREVENTS: Sending 8-byte AGGREGATOR to a 2-byte-only peer on the EBGP prepend path.
func TestRewriteASPath_AggregatorNonMappable(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{200000}},
	}, true)
	aggAddr := netip.MustParseAddr("10.0.0.1")
	agg := buildAggregatorAttr(4200000000, aggAddr, true)
	attrs := concatAttrs(origin, aspath, agg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPath(dst, payload, 65000, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AS_PATH: prepended + transcoded to 2-byte.
	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{65000, attribute.ASTrans}, path.Segments[0].ASNs)

	// AGGREGATOR: 6 bytes with AS_TRANS.
	asn, valLen, found := parseAggregatorFromPayload(t, result)
	require.True(t, found, "AGGREGATOR must be present")
	assert.Equal(t, 6, valLen)
	assert.Equal(t, uint32(23456), asn, "AGGREGATOR ASN should be AS_TRANS")

	// AS4_AGGREGATOR: original 4-byte ASN + IP.
	as4ASN, as4Addr, found := parseAS4AggregatorFromPayload(t, result)
	require.True(t, found, "AS4_AGGREGATOR must be present")
	assert.Equal(t, uint32(4200000000), as4ASN)
	assert.Equal(t, aggAddr, as4Addr)
}

// TestRewriteASPath_AggregatorMappable verifies that RewriteASPath transcodes
// AGGREGATOR from 8→6 bytes without inserting AS4_AGGREGATOR when the ASN fits.
//
// VALIDATES: RFC 6793 Section 4.2.2: mappable AGGREGATOR re-encoded; no AS4_AGGREGATOR.
// PREVENTS: Sending 8-byte AGGREGATOR to a 2-byte-only peer for mappable ASNs.
func TestRewriteASPath_AggregatorMappable(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	aggAddr := netip.MustParseAddr("192.0.2.1")
	agg := buildAggregatorAttr(65001, aggAddr, true)
	attrs := concatAttrs(origin, aspath, agg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPath(dst, payload, 65000, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AGGREGATOR: 6 bytes, ASN preserved.
	asn, valLen, found := parseAggregatorFromPayload(t, result)
	require.True(t, found)
	assert.Equal(t, 6, valLen)
	assert.Equal(t, uint32(65001), asn)

	// No AS4_AGGREGATOR.
	_, _, found = parseAS4AggregatorFromPayload(t, result)
	assert.False(t, found, "AS4_AGGREGATOR should not be present for mappable ASN")
}

// TestRewriteASPath_AggregatorSameEncoding verifies that AGGREGATOR is
// preserved as-is when srcASN4 == dstASN4 (no transcoding needed).
//
// VALIDATES: AGGREGATOR untouched when encoding matches.
// PREVENTS: Unnecessary AGGREGATOR rewrite on same-encoding path.
func TestRewriteASPath_AggregatorSameEncoding(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	aggAddr := netip.MustParseAddr("10.0.0.1")
	agg := buildAggregatorAttr(4200000000, aggAddr, true)
	attrs := concatAttrs(origin, aspath, agg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPath(dst, payload, 65000, true, true)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AGGREGATOR: 8 bytes, unchanged.
	asn, valLen, found := parseAggregatorFromPayload(t, result)
	require.True(t, found)
	assert.Equal(t, 8, valLen, "AGGREGATOR should stay 8 bytes when encoding matches")
	assert.Equal(t, uint32(4200000000), asn)

	// No AS4_AGGREGATOR.
	_, _, found = parseAS4AggregatorFromPayload(t, result)
	assert.False(t, found)
}

// TestRewriteASPath_MalformedAggregatorTombstone verifies that RewriteASPath
// replaces a malformed AGGREGATOR with an ATTR_TOMBSTONE when crossing
// ASN4→ASN2 encoding.
//
// VALIDATES: Malformed AGGREGATOR produces tombstone on the EBGP prepend path.
// PREVENTS: Forwarding unparseable AGGREGATOR bytes after AS_PATH prepend.
func TestRewriteASPath_MalformedAggregatorTombstone(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	// Malformed AGGREGATOR: 5 bytes instead of 8 for 4-byte encoding.
	malformedAgg := []byte{0xC0, byte(attribute.AttrAggregator), 5, 0x01, 0x02, 0x03, 0x04, 0x05}
	attrs := concatAttrs(origin, aspath, malformedAgg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPath(dst, payload, 65000, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AS_PATH should be prepended + transcoded.
	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{65000, 64512}, path.Segments[0].ASNs)

	// AGGREGATOR should be replaced by ATTR_TOMBSTONE.
	origCode, reason, valLen, found := parseTombstoneFromPayload(t, result)
	require.True(t, found, "ATTR_TOMBSTONE must be present for malformed AGGREGATOR")
	assert.Equal(t, byte(attribute.AttrAggregator), origCode)
	assert.Equal(t, TombstoneInvalidLength, reason)
	assert.Equal(t, 5, valLen)
}

// --- RFC 6793 Section 4.2.2 AS4_PATH on the EBGP prepend path ---

// TestRewriteASPath_AS4PathWireBytes verifies the exact wire bytes produced when
// the prepended local ASN is non-mappable and the destination is an OLD speaker.
//
// The expectation below is derived from RFC 6793 and RFC 4271 attribute encoding,
// not from ze's output:
//
//	AS_PATH  (RFC 4271 well-known transitive, type 2, 2-octet ASNs per RFC 6793 4.2.2):
//	  40 02 08 | 02 03 | 5BA0 FC00 FC01
//	  flags=0x40 (transitive), code=2, len=8
//	  seg type=2 (AS_SEQUENCE), count=3
//	  23456 (AS_TRANS, 0x5BA0) FC00=64512 FC01=64513
//
//	AS4_PATH (RFC 6793 Section 3: optional transitive, type 17, 4-octet ASNs):
//	  C0 11 0E | 02 03 | 00030D40 0000FC00 0000FC01
//	  flags=0xC0 (optional|transitive), code=17 (0x11), len=14 (0x0E)
//	  seg type=2 (AS_SEQUENCE), count=3
//	  200000 (0x00030D40) 64512 64513
//
// Attribute order (AS4_PATH appended last) is not RFC-constrained; it matches the
// order TranscodeASPath produces.
//
// VALIDATES: AC-1 — RFC 6793 Section 4.2.2 "The NEW BGP speaker MUST also send the
// AS path information in the AS4_PATH attribute (encoded with four-octet AS numbers)".
// PREVENTS: Irrecoverable loss of a >65535 ASN behind AS_TRANS on the eBGP forward path.
func TestRewriteASPath_AS4PathWireBytes(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 64513}},
	}, true)
	payload := buildPayload(nil, concatAttrs(origin, aspath), nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPath(dst, payload, 200000, true, false)
	require.NoError(t, err)

	want := []byte{
		0x00, 0x00, // withdrawn routes length = 0
		0x00, 0x20, // total path attribute length = 32
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x08, // AS_PATH, len 8
		0x02, 0x03, // AS_SEQUENCE, 3 ASNs
		0x5B, 0xA0, // 23456 = AS_TRANS
		0xFC, 0x00, // 64512
		0xFC, 0x01, // 64513
		0xC0, 0x11, 0x0E, // AS4_PATH, len 14
		0x02, 0x03, // AS_SEQUENCE, 3 ASNs
		0x00, 0x03, 0x0D, 0x40, // 200000
		0x00, 0x00, 0xFC, 0x00, // 64512
		0x00, 0x00, 0xFC, 0x01, // 64513
	}
	assert.Equal(t, want, dst[:n])
}

// TestRewriteASPath_AS4PathFromNonMappablePathASN verifies AS4_PATH is emitted when
// the non-mappable ASN comes from the received AS_PATH rather than the local ASN.
//
// VALIDATES: AC-2 — RFC 6793 Section 4.2.2 AS4_PATH carries the full AS path
// information, including path ASNs replaced by AS_TRANS in AS_PATH.
// PREVENTS: A transit 4-byte ASN being flattened to AS_TRANS with no recovery.
func TestRewriteASPath_AS4PathFromNonMappablePathASN(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{200000, 64512}},
	}, true)
	payload := buildPayload(nil, concatAttrs(origin, aspath), nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPath(dst, payload, 65000, true, false)
	require.NoError(t, err)
	result := dst[:n]

	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{65000, attribute.ASTrans, 64512}, path.Segments[0].ASNs)

	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4, "AS4_PATH must be present when the path holds a non-mappable ASN")
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, attribute.ASSequence, as4.Segments[0].Type)
	assert.Equal(t, []uint32{65000, 200000, 64512}, as4.Segments[0].ASNs)
}

// TestRewriteASPath_AS4PathSameEncodingASN2 verifies AS4_PATH is emitted when the
// source already used 2-octet encoding (OLD peer in, OLD peer out) and the local
// ASN is non-mappable. This is the direct-prepend fast path.
//
// VALIDATES: AC-3 — RFC 6793 Section 4.2.2 applies whenever the destination is an
// OLD speaker, regardless of the source encoding.
// PREVENTS: The zero-copy fast path silently skipping the AS4_PATH MUST.
func TestRewriteASPath_AS4PathSameEncodingASN2(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 64513}},
	}, false)
	payload := buildPayload(nil, concatAttrs(origin, aspath), nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPath(dst, payload, 200000, false, false)
	require.NoError(t, err)
	result := dst[:n]

	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{attribute.ASTrans, 64512, 64513}, path.Segments[0].ASNs)

	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4, "AS4_PATH must be present for a non-mappable local ASN")
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, []uint32{200000, 64512, 64513}, as4.Segments[0].ASNs)
}

// TestRewriteASPath_AS4PathPrependedToReceivedAS4Path verifies that a received
// AS4_PATH is extended with the local ASN rather than replaced or left stale.
//
// RFC 6793 Section 4.2.3 reconstruction prepends (count(AS_PATH) - count(AS4_PATH))
// leading AS_PATH ASNs to AS4_PATH. Prepending to AS_PATH only would leave the
// receiver prepending AS_TRANS, losing the real local ASN.
//
// VALIDATES: AC-4 — local ASN appears in AS4_PATH ahead of the received AS4_PATH,
// and exactly one AS4_PATH attribute is emitted.
// PREVENTS: Stale AS4_PATH making the receiver reconstruct AS_TRANS for our hop.
func TestRewriteASPath_AS4PathPrependedToReceivedAS4Path(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{attribute.ASTrans, 64512}},
	}, false)
	as4in := buildAS4PathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{131072, 64512}},
	})
	payload := buildPayload(nil, concatAttrs(origin, aspath, as4in), nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPath(dst, payload, 200000, false, false)
	require.NoError(t, err)
	result := dst[:n]

	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{attribute.ASTrans, attribute.ASTrans, 64512}, path.Segments[0].ASNs)

	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4)
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, []uint32{200000, 131072, 64512}, as4.Segments[0].ASNs)
	assert.Equal(t, 1, countAttrOccurrences(t, result, attribute.AttrAS4Path),
		"exactly one AS4_PATH attribute must be emitted")
}

// TestRewriteASPath_NoAS4PathWhenAllMappable verifies the MUST NOT half of the rule.
//
// VALIDATES: AC-5 — RFC 6793 Section 4.2.2 "except for the case where all of the AS
// path information is composed of mappable four-octet AS numbers only ... the NEW BGP
// speaker MUST NOT send the AS4_PATH attribute".
// PREVENTS: Emitting AS4_PATH where the RFC forbids it.
func TestRewriteASPath_NoAS4PathWhenAllMappable(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512, 64513}},
	}, true)
	payload := buildPayload(nil, concatAttrs(origin, aspath), nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPath(dst, payload, 65000, true, false)
	require.NoError(t, err)

	assert.Nil(t, parseAS4PathFromPayload(t, dst[:n]),
		"AS4_PATH MUST NOT be sent when every ASN is mappable")
}

// TestRewriteASPath_NoAS4PathToNewSpeaker verifies AS4_PATH is never sent to a peer
// that negotiated four-octet AS support, even with a non-mappable ASN in the path.
//
// VALIDATES: AC-6 — RFC 6793 Section 4.1 "The new attributes, AS4_PATH and
// AS4_AGGREGATOR, MUST NOT be carried in an UPDATE message between NEW BGP speakers."
// PREVENTS: Over-applying the AS4_PATH rule to ASN4 peers.
func TestRewriteASPath_NoAS4PathToNewSpeaker(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{200000, 64512}},
	}, true)
	payload := buildPayload(nil, concatAttrs(origin, aspath), nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPath(dst, payload, 200001, true, true)
	require.NoError(t, err)

	assert.Nil(t, parseAS4PathFromPayload(t, dst[:n]),
		"AS4_PATH MUST NOT be carried between NEW BGP speakers")
}

// TestRewriteASPath_AS4PathExcludesConfedSegments verifies confederation segments are
// excluded from the constructed AS4_PATH while remaining in AS_PATH.
//
// VALIDATES: AC-7 — RFC 6793 Section 4.2.2 "the NEW BGP speaker MUST exclude such path
// segments from the AS4_PATH attribute being constructed".
// PREVENTS: Leaking AS_CONFED_* segments outside the confederation via AS4_PATH.
func TestRewriteASPath_AS4PathExcludesConfedSegments(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASConfedSequence, ASNs: []uint32{65001}},
		{Type: attribute.ASSequence, ASNs: []uint32{200000}},
	}, true)
	payload := buildPayload(nil, concatAttrs(origin, aspath), nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPath(dst, payload, 65000, true, false)
	require.NoError(t, err)
	result := dst[:n]

	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 3)
	assert.Equal(t, attribute.ASSequence, path.Segments[0].Type)
	assert.Equal(t, []uint32{65000}, path.Segments[0].ASNs)
	assert.Equal(t, attribute.ASConfedSequence, path.Segments[1].Type)

	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4)
	for _, seg := range as4.Segments {
		assert.NotEqual(t, attribute.ASConfedSequence, seg.Type, "AS4_PATH MUST NOT carry AS_CONFED_SEQUENCE")
		assert.NotEqual(t, attribute.ASConfedSet, seg.Type, "AS4_PATH MUST NOT carry AS_CONFED_SET")
	}
}

// TestRewriteASPathDual_AS4Path verifies the dual-AS prepend also emits AS4_PATH.
//
// VALIDATES: AC-8 — RFC 6793 Section 4.2.2 applies to the local-as dual prepend.
// PREVENTS: The dual-AS override path losing a 4-byte local AS behind AS_TRANS.
func TestRewriteASPathDual_AS4Path(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	payload := buildPayload(nil, concatAttrs(origin, aspath), nil)

	dst := make([]byte, len(payload)+128)
	n, err := RewriteASPathDual(dst, payload, 200000, 65000, true, false)
	require.NoError(t, err)
	result := dst[:n]

	path := parseASPathFromPayload(t, result, false)
	require.Len(t, path.Segments, 1)
	assert.Equal(t, []uint32{attribute.ASTrans, 65000, 64512}, path.Segments[0].ASNs)

	as4 := parseAS4PathFromPayload(t, result)
	require.NotNil(t, as4)
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, []uint32{200000, 65000, 64512}, as4.Segments[0].ASNs)
}

// countAttrOccurrences counts how many times an attribute code appears in a payload.
func countAttrOccurrences(t *testing.T, payload []byte, want attribute.AttributeCode) int {
	t.Helper()
	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + wdLen
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrsStart := attrLenOff + 2

	count := 0
	off := attrsStart
	for off < attrsStart+attrLen {
		_, code, length, hl, err := attribute.ParseHeader(payload[off:])
		require.NoError(t, err)
		if code == want {
			count++
		}
		off += hl + int(length)
	}
	return count
}

// TestRewriteASPath_AS4PathMappabilityBoundary pins the mappable/non-mappable
// boundary that decides whether AS4_PATH is emitted.
//
// RFC 6793 Terminology: "Mappable AS -- Four-octet AS where high two octets are
// zero (fits in two octets)". The boundary is therefore 65535 (last mappable) /
// 65536 (first non-mappable).
//
// VALIDATES: AC-9 — AS4_PATH emitted iff the local ASN exceeds 65535.
// PREVENTS: An off-by-one at the boundary emitting or omitting AS4_PATH wrongly.
func TestRewriteASPath_AS4PathMappabilityBoundary(t *testing.T) {
	tests := []struct {
		name       string
		localASN   uint32
		wantAS4    bool
		wantInPath uint32
	}{
		{"last mappable", 65535, false, 65535},
		{"first non-mappable", 65536, true, attribute.ASTrans},
		{"max 4-byte ASN", 4294967295, true, attribute.ASTrans},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origin := buildOriginAttr()
			aspath := buildASPathAttr([]attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{64512}},
			}, true)
			payload := buildPayload(nil, concatAttrs(origin, aspath), nil)

			dst := make([]byte, len(payload)+128)
			n, err := RewriteASPath(dst, payload, tt.localASN, true, false)
			require.NoError(t, err)
			result := dst[:n]

			path := parseASPathFromPayload(t, result, false)
			require.Len(t, path.Segments, 1)
			assert.Equal(t, tt.wantInPath, path.Segments[0].ASNs[0])

			as4 := parseAS4PathFromPayload(t, result)
			if !tt.wantAS4 {
				assert.Nil(t, as4, "AS4_PATH MUST NOT be sent for a mappable ASN")
				return
			}
			require.NotNil(t, as4, "AS4_PATH MUST be sent for a non-mappable ASN")
			require.Len(t, as4.Segments, 1)
			assert.Equal(t, []uint32{tt.localASN, 64512}, as4.Segments[0].ASNs)
		})
	}
}
