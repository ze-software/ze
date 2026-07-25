package wireu

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"

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

// buildAggregatorAttr constructs an AGGREGATOR attribute with the given ASN and IP.
// When asn4=true, uses 8-byte format (4-byte ASN + 4-byte IP).
// When asn4=false, uses 6-byte format (2-byte ASN + 4-byte IP).
func buildAggregatorAttr(asn uint32, addr netip.Addr, asn4 bool) []byte { //nolint:unparam // asn4 is always true in current tests but parameter needed for correctness
	if asn4 {
		buf := make([]byte, 3+8)
		attribute.WriteHeaderTo(buf, 0,
			attribute.FlagOptional|attribute.FlagTransitive,
			attribute.AttrAggregator, 8)
		binary.BigEndian.PutUint32(buf[3:], asn)
		copy(buf[7:], addr.AsSlice())
		return buf
	}
	buf := make([]byte, 3+6)
	attribute.WriteHeaderTo(buf, 0,
		attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAggregator, 6)
	binary.BigEndian.PutUint16(buf[3:], uint16(asn)) //nolint:gosec // test data
	copy(buf[5:], addr.AsSlice())
	return buf
}

// buildAS4AggregatorAttr constructs an AS4_AGGREGATOR attribute (always 8 bytes).
func buildAS4AggregatorAttr(asn uint32, addr netip.Addr) []byte {
	buf := make([]byte, 3+8)
	attribute.WriteHeaderTo(buf, 0,
		attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Aggregator, 8)
	binary.BigEndian.PutUint32(buf[3:], asn)
	copy(buf[7:], addr.AsSlice())
	return buf
}

// parseAggregatorFromPayload extracts AGGREGATOR from a payload.
// Returns (asn, addr, valueLen, found).
func parseAggregatorFromPayload(t *testing.T, payload []byte) (uint32, int, bool) {
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
		if code == attribute.AttrAggregator {
			value := payload[off+hl : off+hl+int(length)]
			valLen := int(length)
			var asn uint32
			switch valLen {
			case 8:
				asn = binary.BigEndian.Uint32(value[0:4])
			case 6:
				asn = uint32(binary.BigEndian.Uint16(value[0:2]))
			default:
				t.Fatalf("unexpected AGGREGATOR value length: %d", valLen)
			}
			return asn, valLen, true
		}
		off += hl + int(length)
	}
	return 0, 0, false
}

// parseAS4AggregatorFromPayload extracts AS4_AGGREGATOR from a payload.
func parseAS4AggregatorFromPayload(t *testing.T, payload []byte) (uint32, netip.Addr, bool) {
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
		if code == attribute.AttrAS4Aggregator {
			value := payload[off+hl : off+hl+int(length)]
			require.Equal(t, 8, int(length), "AS4_AGGREGATOR must be 8 bytes")
			asn := binary.BigEndian.Uint32(value[0:4])
			addr, ok := netip.AddrFromSlice(value[4:8])
			require.True(t, ok, "AS4_AGGREGATOR IP parse")
			return asn, addr, true
		}
		off += hl + int(length)
	}
	return 0, netip.Addr{}, false
}

// TestTranscodeASPath_4to2_AggregatorMappable verifies that an AGGREGATOR
// with a mappable ASN (<=65535) is re-encoded from 8 to 6 bytes without
// inserting AS4_AGGREGATOR.
//
// VALIDATES: RFC 6793 Section 4.2.2: mappable AGGREGATOR re-encoded to 2-byte; no AS4_AGGREGATOR.
// PREVENTS: Sending 8-byte AGGREGATOR to a 2-byte-only peer.
func TestTranscodeASPath_4to2_AggregatorMappable(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	aggAddr := netip.MustParseAddr("192.0.2.1")
	agg := buildAggregatorAttr(65001, aggAddr, true)
	attrs := concatAttrs(origin, aspath, agg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AGGREGATOR should be 6 bytes (2-byte ASN + 4-byte IP), ASN preserved.
	asn, valLen, found := parseAggregatorFromPayload(t, result)
	require.True(t, found, "AGGREGATOR must be present")
	assert.Equal(t, 6, valLen, "AGGREGATOR value should be 6 bytes for 2-byte encoding")
	assert.Equal(t, uint32(65001), asn)

	// No AS4_AGGREGATOR needed.
	_, _, found = parseAS4AggregatorFromPayload(t, result)
	assert.False(t, found, "AS4_AGGREGATOR should not be present for mappable ASN")
}

// TestTranscodeASPath_4to2_AggregatorNonMappable verifies that an AGGREGATOR
// with a non-mappable ASN (>65535) is re-encoded with AS_TRANS, and
// AS4_AGGREGATOR carries the original 4-byte ASN + IP.
//
// VALIDATES: RFC 6793 Section 4.2.2: non-mappable AGGREGATOR gets AS_TRANS; AS4_AGGREGATOR inserted.
// PREVENTS: Sending non-mappable 4-byte AGGREGATOR ASN to a 2-byte-only peer.
func TestTranscodeASPath_4to2_AggregatorNonMappable(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{200000}},
	}, true)
	aggAddr := netip.MustParseAddr("10.0.0.1")
	agg := buildAggregatorAttr(4200000000, aggAddr, true)
	attrs := concatAttrs(origin, aspath, agg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AGGREGATOR: 6 bytes, ASN = AS_TRANS (23456).
	asn, valLen, found := parseAggregatorFromPayload(t, result)
	require.True(t, found, "AGGREGATOR must be present")
	assert.Equal(t, 6, valLen, "AGGREGATOR value should be 6 bytes")
	assert.Equal(t, uint32(23456), asn, "AGGREGATOR ASN should be AS_TRANS")

	// AS4_AGGREGATOR: original 4-byte ASN + IP.
	as4ASN, as4Addr, found := parseAS4AggregatorFromPayload(t, result)
	require.True(t, found, "AS4_AGGREGATOR must be present for non-mappable ASN")
	assert.Equal(t, uint32(4200000000), as4ASN)
	assert.Equal(t, aggAddr, as4Addr)
}

// TestTranscodeASPath_4to2_StaleAS4Aggregator verifies that an existing
// AS4_AGGREGATOR from the source is stripped when transcoding produces a new one.
//
// VALIDATES: Stale AS4_AGGREGATOR replaced by fresh one with correct values.
// PREVENTS: Duplicate or stale AS4_AGGREGATOR in forwarded UPDATE.
func TestTranscodeASPath_4to2_StaleAS4Aggregator(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{200000}},
	}, true)
	aggAddr := netip.MustParseAddr("10.0.0.1")
	agg := buildAggregatorAttr(4200000000, aggAddr, true)
	staleAS4Agg := buildAS4AggregatorAttr(99999, netip.MustParseAddr("1.1.1.1"))
	attrs := concatAttrs(origin, aspath, agg, staleAS4Agg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AS4_AGGREGATOR should have the NEW values, not stale.
	as4ASN, as4Addr, found := parseAS4AggregatorFromPayload(t, result)
	require.True(t, found, "AS4_AGGREGATOR must be present")
	assert.Equal(t, uint32(4200000000), as4ASN, "should be the new ASN, not stale")
	assert.Equal(t, aggAddr, as4Addr, "should be the new addr, not stale")
}

// TestTranscodeASPath_4to2_AggregatorNoASPath verifies AGGREGATOR transcoding
// when no AS_PATH is present (e.g. End-of-RIB with optional attributes).
//
// VALIDATES: AGGREGATOR transcoded even without AS_PATH.
// PREVENTS: Skipping AGGREGATOR transcode when AS_PATH is absent.
func TestTranscodeASPath_4to2_AggregatorNoASPath(t *testing.T) {
	origin := buildOriginAttr()
	aggAddr := netip.MustParseAddr("10.0.0.1")
	agg := buildAggregatorAttr(4200000000, aggAddr, true)
	attrs := concatAttrs(origin, agg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AGGREGATOR should be transcoded to 6 bytes with AS_TRANS.
	asn, valLen, found := parseAggregatorFromPayload(t, result)
	require.True(t, found, "AGGREGATOR must be present")
	assert.Equal(t, 6, valLen)
	assert.Equal(t, uint32(23456), asn)

	// AS4_AGGREGATOR should carry the original.
	as4ASN, as4Addr, found := parseAS4AggregatorFromPayload(t, result)
	require.True(t, found, "AS4_AGGREGATOR must be present")
	assert.Equal(t, uint32(4200000000), as4ASN)
	assert.Equal(t, aggAddr, as4Addr)
}

// TestTranscodeASPath_4to2_AggregatorBoundary verifies the 65535/65536 boundary
// for AGGREGATOR transcoding (same boundary as AS_PATH).
//
// VALIDATES: AGGREGATOR ASN 65535 maps to itself; 65536 maps to AS_TRANS.
// PREVENTS: Off-by-one in the >65535 threshold for AGGREGATOR.
func TestTranscodeASPath_4to2_AggregatorBoundary(t *testing.T) {
	aggAddr := netip.MustParseAddr("10.0.0.1")
	tests := []struct {
		name       string
		aggASN     uint32
		wantASN    uint32
		wantAS4Agg bool
	}{
		{"last_mappable_65535", 65535, 65535, false},
		{"first_nonmappable_65536", 65536, 23456, true},
		{"large_4byte", 4200000000, 23456, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origin := buildOriginAttr()
			aspath := buildASPathAttr([]attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{64512}},
			}, true)
			agg := buildAggregatorAttr(tt.aggASN, aggAddr, true)
			attrs := concatAttrs(origin, aspath, agg)
			payload := buildPayload(nil, attrs, nil)

			dst := make([]byte, len(payload)+128)
			n, err := TranscodeASPath(dst, payload, true, false)
			require.NoError(t, err)
			require.Positive(t, n)
			result := dst[:n]

			asn, valLen, found := parseAggregatorFromPayload(t, result)
			require.True(t, found)
			assert.Equal(t, 6, valLen)
			assert.Equal(t, tt.wantASN, asn)

			_, _, found = parseAS4AggregatorFromPayload(t, result)
			if tt.wantAS4Agg {
				assert.True(t, found, "AS4_AGGREGATOR expected")
			} else {
				assert.False(t, found, "AS4_AGGREGATOR not expected")
			}
		})
	}
}

// TestTranscodeASPath_4to2_OrphanedAS4Aggregator verifies that an existing
// AS4_AGGREGATOR is preserved when AGGREGATOR was not transcoded (e.g.
// malformed AGGREGATOR size or no AGGREGATOR present).
//
// VALIDATES: AS4_AGGREGATOR preserved when AGGREGATOR not transcoded.
// PREVENTS: Silent loss of AS4_AGGREGATOR when AGGREGATOR size is unexpected.
func TestTranscodeASPath_4to2_OrphanedAS4Aggregator(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{200000}},
	}, true)
	// AS4_AGGREGATOR without a matching AGGREGATOR.
	as4Agg := buildAS4AggregatorAttr(4200000000, netip.MustParseAddr("10.0.0.1"))
	attrs := concatAttrs(origin, aspath, as4Agg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AS4_AGGREGATOR must be preserved (no AGGREGATOR to transcode).
	as4ASN, as4Addr, found := parseAS4AggregatorFromPayload(t, result)
	require.True(t, found, "orphaned AS4_AGGREGATOR must be preserved")
	assert.Equal(t, uint32(4200000000), as4ASN)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), as4Addr)
}

// parseTombstoneFromPayload extracts the first ATTR_TOMBSTONE from a payload.
// Returns (origCode, reason, valueLen, found).
func parseTombstoneFromPayload(t *testing.T, payload []byte) (byte, byte, int, bool) {
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
		if code == attribute.AttrTombstone {
			value := payload[off+hl : off+hl+int(length)]
			require.True(t, int(length) >= 2, "tombstone value too short")
			return value[0], value[1], int(length), true
		}
		off += hl + int(length)
	}
	return 0, 0, 0, false
}

// TestTranscodeASPath_4to2_MalformedAggregator verifies that an AGGREGATOR
// with wrong value length (not 8 for 4-byte encoding) is replaced with
// an ATTR_TOMBSTONE marker instead of being copied verbatim.
//
// VALIDATES: draft-mangin-idr-attr-tombstone-00: malformed attribute overwritten in-place.
// PREVENTS: Forwarding unparseable AGGREGATOR bytes to downstream peers.
func TestTranscodeASPath_4to2_MalformedAggregator(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	// Malformed AGGREGATOR: 5 bytes instead of 8 for 4-byte encoding.
	malformedAgg := []byte{0xC0, byte(attribute.AttrAggregator), 5, 0x01, 0x02, 0x03, 0x04, 0x05}
	attrs := concatAttrs(origin, aspath, malformedAgg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AGGREGATOR should be replaced by ATTR_TOMBSTONE.
	origCode, reason, valLen, found := parseTombstoneFromPayload(t, result)
	require.True(t, found, "ATTR_TOMBSTONE must be present for malformed AGGREGATOR")
	assert.Equal(t, byte(attribute.AttrAggregator), origCode, "original code preserved")
	assert.Equal(t, TombstoneInvalidLength, reason, "reason: invalid length")
	assert.Equal(t, 5, valLen, "value length inherited from original")
}

// TestTranscodeASPath_4to2_MalformedAggregatorTinyValue verifies that a
// malformed AGGREGATOR with value length < 2 is copied verbatim (tombstone
// cannot fit the code+reason pair).
//
// VALIDATES: WriteTombstone fallback when valueLen < 2.
// PREVENTS: Panic or corrupt output on degenerate malformed AGGREGATOR.
func TestTranscodeASPath_4to2_MalformedAggregatorTinyValue(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, true)
	// Malformed AGGREGATOR: 1-byte value (too small for tombstone pair).
	malformedAgg := []byte{0xC0, byte(attribute.AttrAggregator), 1, 0xFF}
	attrs := concatAttrs(origin, aspath, malformedAgg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, true, false)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// No tombstone (value too short). AGGREGATOR copied verbatim.
	_, _, _, found := parseTombstoneFromPayload(t, result)
	assert.False(t, found, "tombstone should not be generated for 1-byte value")

	// Verify output contains the original malformed AGGREGATOR bytes.
	// The attribute header (C0 07 01) + value (FF) should appear in the output.
	assert.Contains(t, string(result), string(malformedAgg), "malformed AGGREGATOR bytes preserved")
}

// TestTranscodeASPath_2to4_Aggregator verifies the 2→4 AGGREGATOR transcoding
// direction (6-byte to 8-byte).
//
// VALIDATES: AGGREGATOR re-encoded from 2-byte ASN to 4-byte ASN.
// PREVENTS: Untested 2→4 code path at TranscodeASPath line 173.
func TestTranscodeASPath_2to4_Aggregator(t *testing.T) {
	origin := buildOriginAttr()
	aspath := buildASPathAttr([]attribute.ASPathSegment{
		{Type: attribute.ASSequence, ASNs: []uint32{64512}},
	}, false) // 2-byte AS_PATH
	aggAddr := netip.MustParseAddr("10.0.0.1")
	agg := buildAggregatorAttr(65001, aggAddr, false) // 6-byte AGGREGATOR
	attrs := concatAttrs(origin, aspath, agg)
	payload := buildPayload(nil, attrs, nil)

	dst := make([]byte, len(payload)+128)
	n, err := TranscodeASPath(dst, payload, false, true)
	require.NoError(t, err)
	require.Positive(t, n)
	result := dst[:n]

	// AGGREGATOR should be 8 bytes (4-byte ASN + 4-byte IP).
	asn, valLen, found := parseAggregatorFromPayload(t, result)
	require.True(t, found, "AGGREGATOR must be present")
	assert.Equal(t, 8, valLen, "AGGREGATOR value should be 8 bytes for 4-byte encoding")
	assert.Equal(t, uint32(65001), asn)

	// No AS4_AGGREGATOR needed (2→4 direction).
	_, _, found = parseAS4AggregatorFromPayload(t, result)
	assert.False(t, found, "AS4_AGGREGATOR should not be present in 2→4 direction")
}
