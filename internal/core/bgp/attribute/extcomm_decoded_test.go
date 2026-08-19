// VALIDATES: ExtendedCommunity.AppendDecoded renders each named extended
//            community type with the spelling Ze's own parsers accept, and
//            RFC 8955 Section 7.5's reserved bits are ignored on decode.
// PREVENTS: the encode and decode vocabularies drifting apart again. The
//           encode table lives in flowspec_action.go, the decode renderer in
//           extcomm_decoded.go, and only a test that walks the encode table
//           can see when a keyword gains no decode spelling.

package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtendedCommunityAppendDecoded pins every type the renderer names, and
// the generic form it falls back to.
//
// VALIDATES: AppendDecoded writes "target:", "origin:", "redirect:",
// "rate-limit:", "mark:" and "traffic-action" for the type/subtype pairs RFC
// 4360 Sections 4 and 5 and RFC 8955 Section 7 define, and
// "0x<type><subtype>:<hex>" for anything else.
//
// PREVENTS: a field offset moving. Each expectation below carries a distinct
// value in the AS half and the local-administrator half, so swapping the two
// reads changes the output.
func TestExtendedCommunityAppendDecoded(t *testing.T) {
	tests := []struct {
		name string
		comm ExtendedCommunity
		want string
	}{
		{
			name: "route target two-octet AS",
			comm: ExtendedCommunity{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01},
			want: "target:65000:1",
		},
		{
			name: "route origin two-octet AS",
			comm: ExtendedCommunity{0x00, 0x03, 0x00, 0x64, 0x00, 0x00, 0x00, 0x02},
			want: "origin:100:2",
		},
		{
			name: "rt-redirect two-octet AS",
			comm: ExtendedCommunity{0x80, 0x08, 0xfd, 0xe8, 0x00, 0x00, 0x03, 0xe7},
			want: "redirect:65000:999",
		},
		{
			// 1000.0 as an IEEE 754 single.
			name: "traffic-rate-bytes 1000",
			comm: ExtendedCommunity{0x80, 0x06, 0x00, 0x00, 0x44, 0x7a, 0x00, 0x00},
			want: "rate-limit:1000",
		},
		{
			name: "traffic-rate-packets 1000",
			comm: ExtendedCommunity{0x80, 0x0c, 0x00, 0x00, 0x44, 0x7a, 0x00, 0x00},
			want: "rate-limit:1000:packets",
		},
		{
			// RFC 8955 Section 7.1: a negative rate decodes as zero.
			name: "traffic-rate-bytes negative discards",
			comm: ExtendedCommunity{0x80, 0x06, 0x00, 0x00, 0xc4, 0x7a, 0x00, 0x00},
			want: "rate-limit:0",
		},
		{
			// 3.4e38, above the uint64 range, where a Go float-to-integer
			// conversion is undefined. Saturating keeps one answer per wire
			// value on every architecture.
			name: "traffic-rate-bytes beyond uint64 saturates",
			comm: ExtendedCommunity{0x80, 0x06, 0x00, 0x00, 0x7f, 0x7f, 0xff, 0xff},
			want: "rate-limit:18446744073709551615",
		},
		{
			name: "traffic-action",
			comm: ExtendedCommunity{0x80, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03},
			want: "traffic-action",
		},
		{
			name: "traffic-marking dscp 46",
			comm: ExtendedCommunity{0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x2e},
			want: "mark:46",
		},
		{
			name: "unnamed type keeps its octets",
			comm: ExtendedCommunity{0x00, 0xff, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
			want: "0x00ff:010203040506",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.comm.AppendDecoded(nil)), "AppendDecoded")
			assert.Equal(t, tt.want, tt.comm.String(), "String must agree with AppendDecoded")
		})
	}
}

// TestRFC8955TrafficMarkingReservedBitsIgnored drives the traffic-marking
// decoder with the two reserved bits above the DSCP field clear and then set.
//
// VALIDATES: AppendDecoded (extcomm_decoded.go) masks the final octet to its 6
// least significant bits, so the DSCP a peer encoded is the DSCP Ze reads.
//
// PREVENTS: the whole octet reaching a consumer. An unmasked 0xEE decodes to
// 238, which is no DSCP, and the FlowSpec firewall lowering drops any mark
// above 63 -- so a peer that leaves a reserved bit set loses its marking action
// entirely, with no error anywhere.
func TestRFC8955TrafficMarkingReservedBitsIgnored(t *testing.T) {
	// RFC 8955 Section 7.5: the DSCP is "encoded in the 6 least significant bits
	// of the Extended Community value".
	// RFC requirement: RFC8955-7.5-1 positive -- a traffic-marking community with the reserved bits clear decodes to its DSCP (§7.5)
	clean := ExtendedCommunity{0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x2e}
	require.Equal(t, "mark:46", clean.String())

	// RFC 8955 Section 7.5: "reserved (r): MUST be set to 0 on encoding and MUST
	// be ignored during decoding." 0xEE is 0b11_101110: both reserved bits set
	// above the same DSCP 46.
	// RFC requirement: RFC8955-7.5-1 negative -- a traffic-marking community with both reserved bits set decodes to the same DSCP (§7.5)
	reserved := ExtendedCommunity{0x80, 0x09, 0xff, 0xff, 0xff, 0xff, 0xff, 0xee}
	assert.Equal(t, clean.String(), reserved.String(),
		"the reserved bits above the 6-bit DSCP field must not change the decode")

	// The boundary: 63 is the last valid DSCP, and 0x40 is the first reserved
	// bit on its own, which must read as DSCP 0 rather than as 64.
	assert.Equal(t, "mark:63", ExtendedCommunity{0x80, 0x09, 0, 0, 0, 0, 0, 0x3f}.String())
	assert.Equal(t, "mark:0", ExtendedCommunity{0x80, 0x09, 0, 0, 0, 0, 0, 0x40}.String())
}

// TestFlowSpecActionKeywordsDecodeToTheirSpelling walks the encode keyword
// table and decodes every community it produces.
//
// VALIDATES: a community the encode side builds from a keyword renders through
// AppendDecoded as the spelling the FlowSpec firewall bridge matches. `discard`
// encodes to sub-type 0x06 with rate 0 and decodes to "rate-limit:0", not back
// to "discard": that asymmetry is intended, because "rate-limit:0" is the
// documented spelling and the one the bridge reads.
//
// PREVENTS: a keyword being added to flowSpecActionKeywords with no decoded
// spelling decided for it. The table below is checked against
// FlowSpecActionKeywords(), so a new keyword fails this test rather than
// reaching an operator as raw hex.
func TestFlowSpecActionKeywordsDecodeToTheirSpelling(t *testing.T) {
	decoded := map[string]string{
		// RFC 8955 Section 7.3: traffic-rate 0 is discard, and the rate form is
		// what the FlowSpec firewall bridge matches on the update event.
		"discard": "rate-limit:0",
		// Pre-IETF draft and RFC 5575bis redirect-to-nexthop: type 0x08,
		// sub-type 0x00, which no RFC names, so they keep their octets.
		"redirect-to-nexthop-draft": "0x0800:000000000000",
		"copy-to-nexthop":           "0x0800:000000000001",
	}

	keywords := FlowSpecActionKeywords()
	require.Len(t, decoded, len(keywords),
		"every encode keyword needs a decided decode spelling: %v", keywords)

	for _, keyword := range keywords {
		t.Run(keyword, func(t *testing.T) {
			want, ok := decoded[keyword]
			require.True(t, ok, "keyword %q has no decode spelling in this table", keyword)

			comm, ok := FlowSpecActionKeyword(keyword)
			require.True(t, ok, "FlowSpecActionKeywords listed a keyword the lookup rejects")
			assert.Equal(t, want, comm.String())
		})
	}

	// The bridge's discard case matches this exact string
	// (internal/plugins/flowspec-firewall/translate.go parseExtendedCommunities).
	comm, ok := FlowSpecActionKeyword("discard")
	require.True(t, ok)
	assert.Equal(t, "rate-limit:0", comm.String())
}
