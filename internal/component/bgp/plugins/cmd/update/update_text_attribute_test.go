// Design: docs/architecture/api/update-syntax.md -- the generic attribute keyword
// Related: update_text_attribute.go -- parseRawAttributeText and withRawAttrs, under test

package update

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/textparse"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestUpdateTextGenericAttribute reads `attribute [ 0x<code> 0x<flags>
// 0x<value> ]` through the update text entry point and compares the attribute
// block it builds with the octets RFC 4271 Section 4.3 lays out for it.
//
// Method: one ParseUpdateText call per case; the want column is the attribute
// block, flags octet, type code, length, value. For a code no keyword writes,
// flags RFC 4271 Section 4.3 lets a sender write go to the wire as written: the Partial bit of an
// optional transitive attribute, and the Extended Length bit (0xd0 writes a
// two-octet length for a four-octet value). The attribute takes its place in
// ascending type-code order among the ones the keywords wrote.
func TestUpdateTextGenericAttribute(t *testing.T) {
	cases := []struct {
		name string
		args string
		want string
	}{
		{"optional transitive partial", "med 7 attribute [ 0x99 0xe0 0x00000064 ]", "80040400000007" + "E0990400000064"},
		{"extended length as written", "attribute [ 0x99 0xd0 0x00000064 ]", "D099000400000064"},
		{"optional non-transitive", "attribute [ 0x99 0x80 0x01 ]", "80990101"},
		{"upper case prefix", "attribute [ 0X99 0XC0 0X01 ]", "C0990101"},
		{"unspaced brackets", "attribute [0x99 0xc0 0x01020304]", "C0990401020304"},
		{"empty value", "attribute [ 0x99 0xc0 0x ]", "C09900"},
		{"two codes", "attribute [ 0x99 0xc0 0x01 ] attribute [ 0x98 0xc0 0x02 ]", "C0980102C0990101"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append(strings.Fields(tc.args), "nhop", "1.2.3.4", "nlri", "add", "10.0.0.0/24")
			result, err := ParseUpdateText(args)
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			require.NotNil(t, result.Groups[0].Wire)
			assert.Equal(t, strings.ToLower(tc.want), hex.EncodeToString(result.Groups[0].Wire.Packed()))
		})
	}
}

// TestUpdateTextGenericAttributeRefused checks each statement the keyword
// refuses, and that the refusal names what is wrong.
//
// Method: one ParseUpdateText call per case, asserting an error containing the
// fragment. Ze builds NEXT_HOP, MP_REACH_NLRI, MP_UNREACH_NLRI, AS4_PATH and
// AS4_AGGREGATOR from other statements, a code a keyword writes is refused when
// its value is malformed or beside that keyword, RFC 4271 Section 4.3 fixes the
// flags a sender may write, a value is at most 65535 octets, and
// RFC 4271 Section 6.3 makes an attribute given twice a Malformed Attribute
// List.
func TestUpdateTextGenericAttributeRefused(t *testing.T) {
	cases := []struct {
		name string
		args string
		want string
	}{
		{"no bracket", "attribute 0x99 0xc0 0x01", "attribute [ 0x<code> 0x<flags> 0x<value> ]"},
		{"unclosed", "attribute [ 0x99 0xc0 0x01", "attribute [ 0x<code> 0x<flags> 0x<value> ]"},
		{"two words", "attribute [ 0x99 0xc0 ]", "attribute [ 0x<code> 0x<flags> 0x<value> ]"},
		{"decimal code", "attribute [ 153 0xc0 0x01 ]", "code"},
		{"code above a byte", "attribute [ 0x199 0xc0 0x01 ]", "code"},
		{"decimal flags", "attribute [ 0x99 192 0x01 ]", "flags"},
		{"odd value digits", "attribute [ 0x99 0xc0 0x123 ]", "odd number"},
		{"value not hex", "attribute [ 0x99 0xc0 0xzz ]", "value"},
		{"mp reach", "attribute [ 0x0e 0x80 0x00 ]", "MP_REACH_NLRI"},
		{"mp unreach", "attribute [ 0x0f 0x80 0x00 ]", "MP_UNREACH_NLRI"},
		{"as4 path", "attribute [ 0x11 0xc0 0x00 ]", "AS4_PATH"},
		{"as4 aggregator", "attribute [ 0x12 0xc0 0x00 ]", "AS4_AGGREGATOR"},
		{"next hop", "attribute [ 0x03 0x40 0x01020304 ]", "NEXT_HOP"},
		{"twice", "attribute [ 0x99 0xc0 0x01 ] attribute [ 0x99 0xc0 0x02 ]", "given twice"},
		{"beside its keyword", "med 7 attribute [ 0x04 0x80 0x00000064 ]", "given twice"},
		{"keyword after it", "attribute [ 0x01 0x40 0x00 ] origin igp", "given twice"},
		{"prefix sid malformed", "attribute [ 0x28 0xc0 0x01 ]", "malformed"},
		{"ipv6 extended community malformed", "attribute [ 0x19 0xc0 0x00 ]", "malformed"},
		{"value above 65535 octets", "attribute [ 0x99 0xc0 0x" + strings.Repeat("00", 65536) + " ]", "above the 65535"},
		{"unused flag bits", "attribute [ 0x99 0xc1 0x01 ]", "unused bits 0x01"},
		{"unknown code sent well-known", "attribute [ 0x99 0x40 0x01 ]", "clear the Optional bit"},
		{"unknown code well-known partial", "attribute [ 0x99 0x60 0x00000064 ]", "clear the Optional bit"},
		{"unknown code well-known extended", "attribute [ 0x99 0x70 0x00000064 ]", "clear the Optional bit"},
		{"well-known partial", "attribute [ 0x01 0x60 0x00 ]", "Partial bit"},
		{"well-known not transitive", "attribute [ 0x01 0x00 0x00 ]", "write 0x40"},
		{"optional non-transitive partial", "attribute [ 0x99 0xa0 0x01 ]", "Partial bit"},
		{"decoded code partial", "attribute [ 0x08 0xe0 0x00000001 ]", "write 0xc0"},
		{"large community repeated", "attribute [ 0x20 0xc0 0x000000010000000200000003000000010000000200000003 ]", "repeats the large community"},
		{"known code optional bit wrong", "attribute [ 0x08 0x40 0x00000001 ]", "write 0xc0"},
		{"known code transitive bit wrong", "attribute [ 0x09 0xc0 0x01020304 ]", "write 0x80"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append(strings.Fields(tc.args), "nhop", "1.2.3.4", "nlri", "add", "10.0.0.0/24")
			_, err := ParseUpdateText(args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// typedRawCase is a generic attribute of a code an update text keyword writes:
// a well-formed value, and a value too short and too long for its decoder.
// An empty short means the code has no shorter value than the good one.
type typedRawCase struct {
	name, code, flags, good, short, long string
}

// typedRawCases holds one row for each type code textparse pairs with a
// keyword, NEXT_HOP aside: Ze builds that one from next-hop.
var typedRawCases = []typedRawCase{
	{"origin", "01", "40", "02", "", "0000"},
	{"as-path", "02", "40", "02010000fde8", "0201", "02010000fde800"},
	{"med", "04", "80", "00000064", "0064", "0000006400"},
	{"local-preference", "05", "40", "00000064", "0064", "0000006400"},
	{"atomic-aggregate", "06", "40", "", "", "01"},
	{"aggregator", "07", "c0", "0000fde8c0000201", "0000fde8", "0000fde8c000020100"},
	{"community", "08", "c0", "fde80001", "", "fde8000100"},
	{"originator-id", "09", "80", "01020304", "0102", "0102030405"},
	{"cluster-list", "0a", "80", "01020304", "", "0102030405"},
	{"extended-community", "10", "c0", "0002fde800000064", "", "0002fde80000006400"},
	{"ipv6-extended-community", "19", "c0", "000220010db8000000000000000000000001" + "0064", "", "000220010db8000000000000000000000001" + "006400"},
	{"aigp", "1a", "80", "01000b0000000000000064", "01000b00", "01000b000000000000006400"},
	{"large-community", "20", "c0", "0000fde80000000100000002", "", "0000fde8000000010000000200"},
	{"prefix-sid", "28", "c0", "01000700000000000064", "0100", "0100070000000000006400"},
}

// TestUpdateTextGenericAttributeTyped checks that a generic attribute of a code
// a keyword writes is decoded by that attribute's wire decoder and goes into
// the block as its typed form writes it, and that a value the decoder refuses
// or leaves octets of unread is refused as malformed.
//
// Method: one ParseUpdateText call per row and value. The good value must come
// out as flags, code, length and the same octets; the short and long values
// must be refused naming the code. An empty list is refused for having no
// value (RFC 7606 Sections 7.8, 7.10, 7.14 and 7.15, RFC 8092 Section 6).
func TestUpdateTextGenericAttributeTyped(t *testing.T) {
	for _, tc := range typedRawCases {
		t.Run(tc.name, func(t *testing.T) {
			parse := func(value string) (string, error) {
				args := append(strings.Fields("attribute [ 0x"+tc.code+" 0x"+tc.flags+" 0x"+value+" ]"), "nhop", "1.2.3.4", "nlri", "add", "10.0.0.0/24")
				result, err := ParseUpdateText(args)
				if err != nil {
					return "", err
				}
				require.Len(t, result.Groups, 1)
				return hex.EncodeToString(result.Groups[0].Wire.Packed()), nil
			}
			if tc.good != "" || tc.code == "06" {
				block, err := parse(tc.good)
				require.NoError(t, err)
				assert.Equal(t, tc.flags+tc.code+hex.EncodeToString([]byte{byte(len(tc.good) / 2)})+tc.good, block)
			}
			for _, bad := range []string{tc.short, tc.long} {
				if bad == "" {
					continue
				}
				_, err := parse(bad)
				require.Error(t, err, bad)
				assert.Contains(t, err.Error(), "attribute 0x"+tc.code, bad)
				assert.Contains(t, err.Error(), "malformed", bad)
			}
		})
	}
}

// TestAttributeKeywordsPaired walks every per-attribute keyword the update text
// accepts and checks that textparse pairs it with the type code it writes, so a
// keyword added without its pairing fails here rather than letting a generic
// attribute of its code slip past the keyword's typed form.
//
// Method: each keyword is parsed with a sample value; the code
// textparse.KeywordAttributeCode names must be in the block the keyword built.
// origin-as writes no attribute (the reactor builds AS_PATH from it), and
// attribute is the generic form itself.
func TestAttributeKeywordsPaired(t *testing.T) {
	samples := map[string][]string{
		kwOrigin:                    {"igp"},
		kwMED:                       {"7"},
		kwLocalPref:                 {"7"},
		kwASPath:                    {"[", "65000", "]"},
		kwCommunity:                 {"[", "65000:1", "]"},
		kwLargeCommunity:            {"[", "65000:1:2", "]"},
		kwExtendedCommunity:         {"[", "target:65000:1", "]"},
		textparse.KWAtomicAggregate: nil,
		textparse.KWAggregator:      {"65000:10.0.0.1"},
		textparse.KWOriginatorID:    {"1.2.3.4"},
		textparse.KWClusterList:     {"[", "1.2.3.4", "]"},
		textparse.KWAIGP:            {"100"},
		textparse.KWPrefixSID:       {"[", "777", "]"},
		textparse.KWPrefixSIDSRv6:   {"(", "l3-service", "2001:db8::1", ")"},
	}
	for _, keyword := range attributeKeywords {
		if keyword == kwOriginAS || keyword == textparse.KWAttribute {
			continue
		}
		t.Run(keyword, func(t *testing.T) {
			sample, known := samples[keyword]
			require.True(t, known, "add a sample value for %s", keyword)
			code, paired := textparse.KeywordAttributeCode(keyword)
			require.True(t, paired, "textparse.attributeKeywordCodes has no row for %s", keyword)
			args := append(append([]string{keyword}, sample...), "nhop", "1.2.3.4", "nlri", "add", "10.0.0.0/24")
			result, err := ParseUpdateText(args)
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			_, _, _, found := attribute.AttrFind(result.Groups[0].Wire.Packed(), code)
			assert.True(t, found, "%s does not write the code %d textparse pairs it with", keyword, code)
		})
	}
}

// TestUpdateTextPrefixSIDGivenTwiceRefused proves a second BGP Prefix-SID
// keyword is refused instead of silently replacing the first.
//
// Method: bgp-prefix-sid and bgp-prefix-sid-srv6 both write attribute 40, so
// each pair of the two, in either order, must fail with the error that names
// both keywords and says to keep one; one of them alone must still parse.
// RFC 4271 Section 6.3 makes an attribute given twice a Malformed Attribute
// List.
func TestUpdateTextPrefixSIDGivenTwiceRefused(t *testing.T) {
	index := []string{"bgp-prefix-sid", "[", "100", "]"}
	srv6 := []string{"bgp-prefix-sid-srv6", "(", "l3-service", "2001:db8:1:1::", "0x48", "[64,24,16,0,0,0]", ")"}
	tail := []string{"nhop", "1.2.3.4", "nlri", "add", "10.0.0.0/24"}
	join := func(parts ...[]string) []string {
		var out []string
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}

	for _, alone := range [][]string{index, srv6} {
		_, err := ParseUpdateText(join(alone, tail))
		require.NoError(t, err, "%v alone parses", alone[0])
	}

	cases := []struct {
		name string
		args []string
	}{
		{"index twice", join(index, index, tail)},
		{"srv6 twice", join(srv6, srv6, tail)},
		{"index then srv6", join(index, srv6, tail)},
		{"srv6 then index", join(srv6, index, tail)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseUpdateText(tc.args)
			require.ErrorIs(t, err, errPrefixSIDTwice)
			assert.Contains(t, err.Error(), "keep one")
		})
	}
}
