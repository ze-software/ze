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

// TestPackHeader verifies attribute header packing.
func TestPackHeader(t *testing.T) {
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
			got := PackHeader(tt.flags, tt.code, tt.length)
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

// TestPackAttributesOrdered verifies packing with ordering.
//
// RFC 4271 Appendix F.3: Order by type code for efficient comparison.
//
// VALIDATES: PackAttributesOrdered produces correctly ordered output.
func TestPackAttributesOrdered(t *testing.T) {
	// Create attributes out of order: COMMUNITY(8), ORIGIN(1)
	community := Communities{Community(0xFDE90064)}
	origin := OriginIGP

	attrs := []Attribute{community, origin}

	packed := PackAttributesOrdered(attrs)

	// Parse the packed data to verify order
	// First attribute should be ORIGIN (code 1)
	_, code1, _, _, err := ParseHeader(packed)
	require.NoError(t, err)
	assert.Equal(t, AttrOrigin, code1, "first attribute should be ORIGIN")
}
