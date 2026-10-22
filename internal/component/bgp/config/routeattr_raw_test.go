// Design: docs/architecture/config/syntax.md -- the generic attribute of a static route
// Related: routeattr.go -- ParseRouteAttributes, which reads it through textparse.ParseRawAttribute

package bgpconfig

import (
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteAttributeGeneric checks that a static route's `attribute` is read by
// textparse.ParseRawAttribute, the update text's reader, so the config file
// refuses the same statements, the RFC 4271 Section 4.3 flag checks included.
//
// Method: one ParseRouteAttributes call per case. An unknown optional code is
// kept raw with its flags; flags a sender must not write, a malformed
// statement, a code Ze builds itself and a value above 65535 octets are refused.
func TestRouteAttributeGeneric(t *testing.T) {
	parse := func(attr string) (*ParsedRouteAttributes, error) {
		return ParseRouteAttributes(&StaticRouteConfig{
			Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:   "192.0.2.1",
			Origin:    "igp",
			Attribute: attr,
		})
	}

	attrs, err := parse("[ 0x99 0xd0 0x01 ]")
	require.NoError(t, err)
	require.Len(t, attrs.RawAttributes, 1)
	assert.Equal(t, uint8(0xd0), attrs.RawAttributes[0].Flags)
	assert.Equal(t, []byte{0x01}, attrs.RawAttributes[0].Value)

	refused := map[string]string{
		"0x99 0x60 0x00000064": "clear the Optional bit",
		"0x99 0xa0 0x01":       "Partial bit",
		"0x08 0xe0 0x00000001": "write 0xc0",
		"0x99 0xc8 0x01":       "unused bits",
		"0x99 0xc0":            "needs three",
		"99 c0 01":             "not hexadecimal",
		"0x0e 0x80 0x00":       "MP_REACH_NLRI",
		"0x01 0x40 0x00":       "given twice",
		"0x99 0xc0 0x" + strings.Repeat("00", 65536): "above the 65535",
	}
	for attr, want := range refused {
		_, err := parse(attr)
		require.Error(t, err, attr)
		assert.Contains(t, err.Error(), want, attr)
	}
}

// TestRouteAttributeGenericTyped checks that a generic attribute of a code a
// keyword writes is decoded into the field that keyword fills, replacing its
// default and never kept raw beside it, and that a malformed value is refused,
// as the update text does (update.TestUpdateTextGenericAttributeTyped).
//
// Method: one ParseRouteAttributes call per code and value. The good value must
// set the typed field and leave RawAttributes empty; the short and long values
// must be refused as malformed, an empty list for having no value.
func TestRouteAttributeGenericTyped(t *testing.T) {
	parse := func(code, flags, value string) (*ParsedRouteAttributes, error) {
		return ParseRouteAttributes(&StaticRouteConfig{
			Prefix:    netip.MustParsePrefix("10.0.0.0/24"),
			NextHop:   "192.0.2.1",
			Attribute: "[ 0x" + code + " 0x" + flags + " 0x" + value + " ]",
		})
	}
	metric := uint64(100)
	cases := []struct {
		name, code, flags, good, short, long string
		check                                func(*ParsedRouteAttributes) bool
	}{
		{"origin", "01", "40", "02", "", "0000", func(a *ParsedRouteAttributes) bool { return a.Origin == OriginIncomplete }},
		{"as-path", "02", "40", "02010000fde8", "0201", "02010000fde800", func(a *ParsedRouteAttributes) bool { return slices.Equal(a.ASPath.Values, []uint32{65000}) }},
		{"med", "04", "80", "00000064", "0064", "0000006400", func(a *ParsedRouteAttributes) bool { return a.HasMED && a.MED == 100 }},
		{"local-preference", "05", "40", "00000064", "0064", "0000006400", func(a *ParsedRouteAttributes) bool { return a.HasLocalPreference && a.LocalPreference == 100 }},
		{"atomic-aggregate", "06", "40", "", "", "01", func(a *ParsedRouteAttributes) bool { return a.AtomicAggregate }},
		{"aggregator", "07", "c0", "0000fde8c0000201", "0000fde8", "0000fde8c000020100", func(a *ParsedRouteAttributes) bool {
			return a.Aggregator.Valid && a.Aggregator.ASN == 65000 && a.Aggregator.IP == [4]byte{192, 0, 2, 1}
		}},
		{"community", "08", "c0", "fde80001", "", "fde8000100", func(a *ParsedRouteAttributes) bool { return slices.Equal(a.Community.Values, []uint32{0xfde80001}) }},
		{"originator-id", "09", "80", "01020304", "0102", "0102030405", func(a *ParsedRouteAttributes) bool { return a.OriginatorID == 0x01020304 }},
		{"cluster-list", "0a", "80", "01020304", "", "0102030405", func(a *ParsedRouteAttributes) bool { return slices.Equal(a.ClusterList, []uint32{0x01020304}) }},
		{"extended-community", "10", "c0", "0002fde800000064", "", "0002fde80000006400", func(a *ParsedRouteAttributes) bool { return len(a.ExtendedCommunity.Bytes) == 8 }},
		{"ipv6-extended-community", "19", "c0", "000220010db80000000000000000000000010064", "", "000220010db8000000000000000000000001006400", func(a *ParsedRouteAttributes) bool {
			return len(a.ExtendedCommunity.IPv6Bytes) == 20
		}},
		{"aigp", "1a", "80", "01000b0000000000000064", "01000b00", "01000b000000000000006400", func(a *ParsedRouteAttributes) bool { return a.AIGPMetric != nil && *a.AIGPMetric == metric }},
		{"large-community", "20", "c0", "0000fde80000000100000002", "", "0000fde8000000010000000200", func(a *ParsedRouteAttributes) bool {
			return slices.Equal(a.LargeCommunity.Values, [][3]uint32{{65000, 1, 2}})
		}},
		{"prefix-sid", "28", "c0", "01000700000000000064", "0100", "0100070000000000006400", func(a *ParsedRouteAttributes) bool { return len(a.PrefixSID) == 10 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attrs, err := parse(tc.code, tc.flags, tc.good)
			require.NoError(t, err)
			assert.Empty(t, attrs.RawAttributes, "a typed code is never kept raw beside its field")
			assert.True(t, tc.check(attrs), "the typed field holds the decoded value")
			for _, bad := range []string{tc.short, tc.long} {
				if bad == "" {
					continue
				}
				_, err := parse(tc.code, tc.flags, bad)
				require.Error(t, err, bad)
				assert.Contains(t, err.Error(), "malformed", bad)
			}
		})
	}

	// An empty community list is refused for having no value (RFC 7606
	// Section 7.8), not stored as no communities.
	_, err := parse("08", "c0", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no value")

	// An AS_PATH holding an AS_SET has no typed form in a static route, so it
	// stays raw for the RFC 9774 send guard to withdraw the route.
	attrs, err := parse("02", "40", "01020000fde80000fdea")
	require.NoError(t, err)
	require.Len(t, attrs.RawAttributes, 1)
	assert.Empty(t, attrs.ASPath.Values)

	// An AIGP whose one TLV is not the metric has no aigp field to fill.
	_, err = parse("1a", "80", "02000400")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aigp <metric>")
}
