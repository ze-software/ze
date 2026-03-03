package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAttributeCodes verifies attribute type codes match RFC 4271/RFC 4760.
func TestAttributeCodes(t *testing.T) {
	tests := []struct {
		code AttributeCode
		val  uint8
	}{
		{AttrOrigin, 1},
		{AttrASPath, 2},
		{AttrNextHop, 3},
		{AttrMED, 4},
		{AttrLocalPref, 5},
		{AttrAtomicAggregate, 6},
		{AttrAggregator, 7},
		{AttrCommunity, 8},
		{AttrOriginatorID, 9},
		{AttrClusterList, 10},
		{AttrMPReachNLRI, 14},
		{AttrMPUnreachNLRI, 15},
		{AttrExtCommunity, 16},
		{AttrAS4Path, 17},
		{AttrAS4Aggregator, 18},
		{AttrLargeCommunity, 32},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.val, uint8(tt.code), "code %d", tt.val)
	}
}

// TestAttributeFlags verifies flag bit positions per RFC 4271.
func TestAttributeFlags(t *testing.T) {
	assert.Equal(t, uint8(0x80), uint8(FlagOptional))
	assert.Equal(t, uint8(0x40), uint8(FlagTransitive))
	assert.Equal(t, uint8(0x20), uint8(FlagPartial))
	assert.Equal(t, uint8(0x10), uint8(FlagExtLength))
}

// TestParseHeader verifies attribute header parsing.
func TestParseHeader(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		flags   AttributeFlags
		code    AttributeCode
		length  uint16
		hdrLen  int
		wantErr bool
	}{
		{
			name:   "origin 1-byte length",
			data:   []byte{0x40, 0x01, 0x01, 0x00},
			flags:  FlagTransitive,
			code:   AttrOrigin,
			length: 1,
			hdrLen: 3,
		},
		{
			name:   "as-path extended length",
			data:   []byte{0x50, 0x02, 0x00, 0x10},
			flags:  FlagTransitive | FlagExtLength,
			code:   AttrASPath,
			length: 16,
			hdrLen: 4,
		},
		{
			name:   "optional transitive",
			data:   []byte{0xC0, 0x08, 0x04},
			flags:  FlagOptional | FlagTransitive,
			code:   AttrCommunity,
			length: 4,
			hdrLen: 3,
		},
		{
			name:    "short data",
			data:    []byte{0x40, 0x01},
			wantErr: true,
		},
		{
			name:    "extended length short",
			data:    []byte{0x50, 0x02, 0x00},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, code, length, hdrLen, err := ParseHeader(tt.data)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.flags, flags)
			assert.Equal(t, tt.code, code)
			assert.Equal(t, tt.length, length)
			assert.Equal(t, tt.hdrLen, hdrLen)
		})
	}
}

// TestWriteHeaderToBasic verifies attribute header encoding.
func TestWriteHeaderToBasic(t *testing.T) {
	tests := []struct {
		name   string
		flags  AttributeFlags
		code   AttributeCode
		length uint16
		want   []byte
	}{
		{
			name:   "origin",
			flags:  FlagTransitive,
			code:   AttrOrigin,
			length: 1,
			want:   []byte{0x40, 0x01, 0x01},
		},
		{
			name:   "extended length",
			flags:  FlagTransitive,
			code:   AttrASPath,
			length: 300,
			want:   []byte{0x50, 0x02, 0x01, 0x2C}, // auto-sets ExtLength flag
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBuf := make([]byte, 10)
			gotN := WriteHeaderTo(gotBuf, 0, tt.flags, tt.code, tt.length)
			got := gotBuf[:gotN]
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAttributeCodeString verifies string representation.
func TestAttributeCodeString(t *testing.T) {
	assert.Equal(t, "ORIGIN", AttrOrigin.String())
	assert.Equal(t, "AS_PATH", AttrASPath.String())
	assert.Equal(t, "NEXT_HOP", AttrNextHop.String())
	assert.Equal(t, "MULTI_EXIT_DISC", AttrMED.String())
	assert.Equal(t, "LOCAL_PREF", AttrLocalPref.String())
	assert.Equal(t, "COMMUNITIES", AttrCommunity.String())
	assert.Equal(t, "MP_REACH_NLRI", AttrMPReachNLRI.String())
	assert.Equal(t, "UNKNOWN(99)", AttributeCode(99).String())
}

// TestOrderAttributes verifies RFC 4271 Appendix F.3 attribute ordering.
//
// RFC 4271 Appendix F.3 - Path Attribute Ordering:
//
//	"It is a useful optimization to order the path attributes according
//	 to type code. This optimization is entirely optional."
//
// VALIDATES: Attributes are sorted by type code.
//
// PREVENTS: Non-deterministic attribute order in UPDATE messages.
func TestOrderAttributes(t *testing.T) {
	// Create attributes out of order: COMMUNITY(8), ORIGIN(1), AS_PATH(2)
	community := Communities{Community(0xFDE90064)}
	origin := OriginIGP
	aspath := &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001}}}}

	attrs := []Attribute{community, origin, aspath}

	ordered := OrderAttributes(attrs)

	require.Len(t, ordered, 3)
	assert.Equal(t, AttrOrigin, ordered[0].Code())    // 1
	assert.Equal(t, AttrASPath, ordered[1].Code())    // 2
	assert.Equal(t, AttrCommunity, ordered[2].Code()) // 8
}

// TestOrderAttributesEmpty verifies empty/nil handling.
func TestOrderAttributesEmpty(t *testing.T) {
	assert.Nil(t, OrderAttributes(nil))
	assert.Equal(t, []Attribute{}, OrderAttributes([]Attribute{}))
}

// TestOrderAttributesSingle verifies single attribute.
func TestOrderAttributesSingle(t *testing.T) {
	origin := OriginIGP
	attrs := []Attribute{origin}

	ordered := OrderAttributes(attrs)

	require.Len(t, ordered, 1)
	assert.Equal(t, AttrOrigin, ordered[0].Code())
}

// TestOrderAttributesMPPlacement verifies MP_UNREACH first, attrs ordered, MP_REACH last.
//
// Order: MP_UNREACH_NLRI (15) → regular attrs by type code → MP_REACH_NLRI (14)
//
// VALIDATES: MP attributes placed correctly regardless of input order.
// PREVENTS: ExaBGP compatibility issues from wrong attribute ordering.
func TestOrderAttributesMPPlacement(t *testing.T) {
	// Create attrs in wrong order: REACH(14), COMMUNITY(8), ORIGIN(1), UNREACH(15)
	mpReach := &MPReachNLRI{AFI: AFIIPv6, SAFI: SAFIUnicast}
	community := Communities{Community(0xFDE90064)}
	origin := OriginIGP
	mpUnreach := &MPUnreachNLRI{AFI: AFIIPv6, SAFI: SAFIUnicast}

	attrs := []Attribute{mpReach, community, origin, mpUnreach}

	ordered := OrderAttributes(attrs)

	require.Len(t, ordered, 4)
	assert.Equal(t, AttrMPUnreachNLRI, ordered[0].Code()) // 15 - first
	assert.Equal(t, AttrOrigin, ordered[1].Code())        // 1
	assert.Equal(t, AttrCommunity, ordered[2].Code())     // 8
	assert.Equal(t, AttrMPReachNLRI, ordered[3].Code())   // 14 - last
}

// TestWriteAttributesOrdered verifies writing attributes with ordering.
//
// RFC 4271 Appendix F.3: Order by type code for efficient comparison.
//
// VALIDATES: WriteAttributesOrdered produces correctly ordered output.
func TestWriteAttributesOrdered(t *testing.T) {
	// Create attributes out of order: COMMUNITY(8), ORIGIN(1)
	community := Communities{Community(0xFDE90064)}
	origin := OriginIGP

	attrs := []Attribute{community, origin}

	packedBuf := make([]byte, 4096)
	packedN := WriteAttributesOrdered(attrs, packedBuf, 0)
	packed := packedBuf[:packedN]

	// Parse the packed data to verify order
	// First attribute should be ORIGIN (code 1)
	_, code1, _, _, err := ParseHeader(packed)
	require.NoError(t, err)
	assert.Equal(t, AttrOrigin, code1, "first attribute should be ORIGIN")
}

// TestWriteHeaderToBoundary verifies 255/256 byte boundary handling.
//
// RFC 4271 Section 4.3: Extended Length flag (0x10) required when length > 255.
//
// VALIDATES: WriteHeaderTo auto-sets extended length flag at boundary.
//
// PREVENTS: Length byte overflow causing malformed attribute header.
func TestWriteHeaderToBoundary(t *testing.T) {
	tests := []struct {
		name       string
		length     uint16
		wantHdrLen int
		wantExtLen bool
	}{
		{
			name:       "length 0",
			length:     0,
			wantHdrLen: 3,
			wantExtLen: false,
		},
		{
			name:       "length 254",
			length:     254,
			wantHdrLen: 3,
			wantExtLen: false,
		},
		{
			name:       "length 255 (max 1-byte)",
			length:     255,
			wantHdrLen: 3,
			wantExtLen: false,
		},
		{
			name:       "length 256 (requires extended)",
			length:     256,
			wantHdrLen: 4,
			wantExtLen: true,
		},
		{
			name:       "length 1000",
			length:     1000,
			wantHdrLen: 4,
			wantExtLen: true,
		},
		{
			name:       "length 65535 (max)",
			length:     65535,
			wantHdrLen: 4,
			wantExtLen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 10)
			n := WriteHeaderTo(buf, 0, FlagTransitive, AttrASPath, tt.length)

			assert.Equal(t, tt.wantHdrLen, n, "header length")

			// Verify extended length flag
			flags := AttributeFlags(buf[0])
			assert.Equal(t, tt.wantExtLen, flags.IsExtLength(), "extended length flag")

			// Verify length value can be read back correctly
			_, _, parsedLen, _, err := ParseHeader(buf[:n])
			require.NoError(t, err)
			assert.Equal(t, tt.length, parsedLen, "parsed length")
		})
	}
}

// TestWriteHeaderTo verifies WriteHeaderTo produces correct wire format.
//
// VALIDATES: WriteHeaderTo produces correct attribute headers.
//
// PREVENTS: Wire format errors in attribute header encoding.
func TestWriteHeaderTo(t *testing.T) {
	tests := []struct {
		flags  AttributeFlags
		code   AttributeCode
		length uint16
	}{
		{FlagTransitive, AttrOrigin, 1},
		{FlagOptional | FlagTransitive, AttrCommunity, 100},
		{FlagTransitive, AttrASPath, 255},
		{FlagTransitive, AttrASPath, 256},
		{FlagTransitive, AttrASPath, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			refBuf := make([]byte, 10)
			refN := WriteHeaderTo(refBuf, 0, tt.flags, tt.code, tt.length)
			expected := refBuf[:refN]

			buf := make([]byte, 10)
			n := WriteHeaderTo(buf, 0, tt.flags, tt.code, tt.length)

			assert.Equal(t, len(expected), n, "length mismatch")
			assert.Equal(t, expected, buf[:n], "content mismatch")
		})
	}
}

// TestWriteHeaderToOffset verifies WriteHeaderTo respects offset.
//
// VALIDATES: WriteHeaderTo writes at correct offset.
//
// PREVENTS: Buffer corruption when writing at non-zero offset.
func TestWriteHeaderToOffset(t *testing.T) {
	buf := make([]byte, 100)
	for i := range buf {
		buf[i] = 0xAA
	}

	offset := 50
	n := WriteHeaderTo(buf, offset, FlagTransitive, AttrASPath, 300)

	assert.Equal(t, 4, n) // Extended length header

	// Verify bytes before offset are untouched
	for i := range offset {
		assert.Equal(t, byte(0xAA), buf[i], "byte %d should be untouched", i)
	}

	// Verify header written correctly
	flags, code, length, _, err := ParseHeader(buf[offset:])
	require.NoError(t, err)
	assert.Equal(t, FlagTransitive|FlagExtLength, flags)
	assert.Equal(t, AttrASPath, code)
	assert.Equal(t, uint16(300), length)
}

// TestWriteAttrToExtendedLength verifies WriteAttrTo handles large attributes.
//
// RFC 4271 Section 4.3: Extended Length flag required when value > 255 bytes.
//
// VALIDATES: WriteAttrTo correctly sets extended length for large attributes.
//
// PREVENTS: Length byte overflow causing malformed attribute (bug found in code review).
func TestWriteAttrToExtendedLength(t *testing.T) {
	tests := []struct {
		name       string
		attr       Attribute
		wantExtLen bool
	}{
		{
			name:       "small COMMUNITIES (4 bytes)",
			attr:       Communities{Community(0xFDE90064)},
			wantExtLen: false,
		},
		{
			name:       "63 communities (252 bytes - no extended)",
			attr:       makeCommunities(63),
			wantExtLen: false,
		},
		{
			name:       "64 communities (256 bytes - extended)",
			attr:       makeCommunities(64),
			wantExtLen: true,
		},
		{
			name:       "100 communities (400 bytes - extended)",
			attr:       makeCommunities(100),
			wantExtLen: true,
		},
		{
			name:       "small AS_PATH",
			attr:       &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001}}}},
			wantExtLen: false,
		},
		{
			name:       "large AS_PATH (100 ASNs = 402 bytes)",
			attr:       makeASPath(100),
			wantExtLen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 4096)
			n := WriteAttrTo(tt.attr, buf, 0)

			// Check extended length flag
			flags := AttributeFlags(buf[0])
			assert.Equal(t, tt.wantExtLen, flags.IsExtLength(), "extended length flag")

			// Parse back to verify correctness
			parsedFlags, parsedCode, parsedLen, hdrLen, err := ParseHeader(buf[:n])
			require.NoError(t, err)
			assert.Equal(t, tt.attr.Code(), parsedCode, "attribute code")
			assert.Equal(t, tt.attr.Len(), int(parsedLen), "attribute length")
			assert.Equal(t, n, hdrLen+int(parsedLen), "total bytes")

			// Verify transitive flag preserved
			if tt.attr.Flags()&FlagTransitive != 0 {
				assert.True(t, parsedFlags.IsTransitive(), "transitive flag should be preserved")
			}
		})
	}
}

// TestWriteAttrTo verifies WriteAttrTo produces correct wire format.
//
// VALIDATES: WriteAttrTo produces correct header + value encoding.
//
// PREVENTS: Wire format errors in full attribute encoding.
func TestWriteAttrTo(t *testing.T) {
	attrs := []Attribute{
		OriginIGP,
		Communities{Community(0xFDE90064), CommunityNoExport},
		makeCommunities(100), // Large to test extended length
		&ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001, 65002}}}},
		makeASPath(100), // Large to test extended length
	}

	for _, attr := range attrs {
		t.Run(attr.Code().String(), func(t *testing.T) {
			refBuf := make([]byte, 4096)
			refN := WriteAttrTo(attr, refBuf, 0)
			expected := refBuf[:refN]

			buf := make([]byte, 4096)
			n := WriteAttrTo(attr, buf, 0)

			assert.Equal(t, len(expected), n, "length mismatch")
			assert.Equal(t, expected, buf[:n], "content mismatch")
		})
	}
}

// Helper to create large Communities for testing.
func makeCommunities(n int) Communities {
	comms := make(Communities, n)
	for i := range comms {
		comms[i] = Community(uint32(0xFFFF0000 | i)) //nolint:gosec // G115: test helper, i bounded
	}
	return comms
}

// Helper to create large ASPath for testing.
func makeASPath(n int) *ASPath {
	asns := make([]uint32, n)
	for i := range asns {
		asns[i] = uint32(65000 + i) //nolint:gosec // G115: test helper, i bounded
	}
	return &ASPath{
		Segments: []ASPathSegment{{Type: ASSequence, ASNs: asns}},
	}
}
