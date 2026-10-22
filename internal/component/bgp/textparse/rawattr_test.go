// Design: docs/architecture/api/update-syntax.md -- the generic attribute keyword
// Related: rawattr.go -- ParseRawAttribute and CheckRawAttributeFlags, under test

package textparse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestCheckRawAttributeFlags walks the sender's rules of RFC 4271 Section 4.3
// over the flags octet of a known well-known, a known optional and an unknown
// code.
//
// Method: one CheckRawAttributeFlags call per case; an accepted octet answers
// nil, a refused one an error naming the rule. "For well-known attributes, the
// Transitive bit MUST be set to 1." "For well-known attributes and for optional
// non-transitive attributes, the Partial bit MUST be set to 0." The low four
// bits "MUST be zero when sent". A code one of Ze's keywords writes is sent from
// its decoded value, so its Partial bit is refused rather than dropped; an
// unknown optional transitive code keeps it, sent as written.
func TestCheckRawAttributeFlags(t *testing.T) {
	cases := []struct {
		name  string
		code  attribute.AttributeCode
		flags attribute.AttributeFlags
		want  string
	}{
		{"well-known", attribute.AttrOrigin, 0x40, ""},
		{"well-known extended length", attribute.AttrOrigin, 0x50, ""},
		{"well-known not transitive", attribute.AttrOrigin, 0x00, "write 0x40"},
		{"well-known partial", attribute.AttrOrigin, 0x60, "Partial bit"},
		{"well-known sent optional", attribute.AttrOrigin, 0xc0, "write 0x40"},
		{"optional transitive", attribute.AttrCommunity, 0xc0, ""},
		{"optional transitive partial on a decoded code", attribute.AttrCommunity, 0xe0, "write 0xc0"},
		{"aggregator partial", attribute.AttrAggregator, 0xe0, "Partial bit"},
		{"extended community partial", attribute.AttrExtCommunity, 0xf0, "write 0xd0"},
		{"ipv6 extended community partial", attribute.AttrIPv6ExtCommunity, 0xe0, "Partial bit"},
		{"large community partial", attribute.AttrLargeCommunity, 0xe0, "Partial bit"},
		{"prefix sid partial", attribute.AttrPrefixSID, 0xe0, "Partial bit"},
		{"optional transitive sent non-transitive", attribute.AttrCommunity, 0x80, "write 0xc0"},
		{"optional non-transitive", attribute.AttrOriginatorID, 0x80, ""},
		{"optional non-transitive partial", attribute.AttrOriginatorID, 0xa0, "Partial bit"},
		{"unknown optional transitive partial", 0x99, 0xf0, ""},
		{"unknown optional non-transitive", 0x99, 0x90, ""},
		{"unknown optional non-transitive partial", 0x99, 0xa0, "Partial bit"},
		{"unknown sent well-known", 0x99, 0x40, "clear the Optional bit"},
		{"unknown sent well-known partial", 0x99, 0x70, "clear the Optional bit"},
		{"unused bits", 0x99, 0xc4, "unused bits 0x04"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckRawAttributeFlags(tc.code, tc.flags)
			if tc.want == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestParseRawAttributeForms checks the spellings both callers write: the
// update text's bracketed value and the config file's leaf, bracketed or not.
//
// Method: each form must answer the same code, flags and value.
func TestParseRawAttributeForms(t *testing.T) {
	for _, text := range []string{"[ 0x99 0xc0 0x0102 ]", "[0x99 0xc0 0x0102]", "0x99 0xc0 0x0102", "0X99 0XC0 0X0102"} {
		raw, err := ParseRawAttribute(text)
		require.NoError(t, err, text)
		assert.Equal(t, RawAttribute{Code: 0x99, Flags: 0xc0, Value: []byte{1, 2}}, raw, text)
	}
	raw, err := ParseRawAttribute("[ 0x99 0xc0 0x ]")
	require.NoError(t, err)
	assert.Empty(t, raw.Value)
	_, err = ParseRawAttribute("[ 0x99 0xc0 0x01")
	require.ErrorIs(t, err, ErrRawAttributeShape)
}

// TestParseRawAttributeLargeCommunityRepeat checks that a LARGE_COMMUNITY value
// holding one large community twice is refused as that repeat, naming the value
// to write, rather than as malformed. RFC 8092 Section 3: "Duplicate BGP Large
// Community values MUST NOT be transmitted."
//
// Method: one value with the community 1:2:3 twice; the refusal must name the
// repeated twelve octets and the value carrying it once.
func TestParseRawAttributeLargeCommunityRepeat(t *testing.T) {
	entry := "000000010000000200000003"
	_, err := ParseRawAttribute("[ 0x20 0xc0 0x" + entry + entry + " ]")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repeats the large community 0x"+entry)
	assert.Contains(t, err.Error(), "write 0x"+entry+",")
	assert.NotContains(t, err.Error(), "malformed")
}

// TestAttributeKeywordPairing checks the keyword and type code lookups both
// ways, the short form of a keyword included.
//
// Method: a code with a keyword, one without, and the short form `nhop`.
func TestAttributeKeywordPairing(t *testing.T) {
	keyword, typed := AttributeKeyword(attribute.AttrMED)
	assert.True(t, typed)
	assert.Equal(t, KWMED, keyword)
	_, typed = AttributeKeyword(0x99)
	assert.False(t, typed)
	code, coded := KeywordAttributeCode(ShortNext)
	assert.True(t, coded)
	assert.Equal(t, attribute.AttrNextHop, code)
	_, coded = KeywordAttributeCode(KWLabel)
	assert.False(t, coded)
}
