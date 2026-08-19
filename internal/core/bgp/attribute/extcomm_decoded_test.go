// VALIDATES: ExtendedCommunity.AppendDecoded renders each named extended
//            community type, in each of its three administrator forms, with the
//            words Ze's own parsers read on input, and ignores the reserved bits
//            RFC 8955 Sections 7.3 and 7.5 say to ignore on decode.
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
// "rate-limit:", "mark:" and "traffic-action:" for the type/subtype pairs RFC
// 4360 Sections 4 and 5 and RFC 8955 Section 7 define, in all three
// administrator forms (two-octet AS, IPv4 address, four-octet AS), and
// "0x<type><subtype>:<hex>" for anything else.
//
// PREVENTS: a field offset moving. Each expectation below carries a distinct
// value in the administrator half and the local-administrator half, so swapping
// the two reads changes the output. The three forms split the 6-octet value
// field in two different places (2+4 and 4+2), so a case for each is what pins
// the split.
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
			// RFC 4360 Section 3.2: a 4-octet IPv4 global administrator and a
			// 2-octet local administrator, the split RFC 4360 Section 4 names
			// for a type 0x01 Route Target.
			name: "route target IPv4 address",
			comm: ExtendedCommunity{0x01, 0x02, 0xc0, 0x00, 0x02, 0x01, 0x00, 0x64},
			want: "target:192.0.2.1:100",
		},
		{
			// RFC 5668 Section 2: a 4-octet AS global administrator and a
			// 2-octet local administrator, the split RFC 4360 Section 4 names
			// for a type 0x02 Route Target. 65536 needs all four octets.
			name: "route target four-octet AS",
			comm: ExtendedCommunity{0x02, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00, 0x64},
			want: "target:65536:100",
		},
		{
			name: "route origin two-octet AS",
			comm: ExtendedCommunity{0x00, 0x03, 0x00, 0x64, 0x00, 0x00, 0x00, 0x02},
			want: "origin:100:2",
		},
		{
			// RFC 4360 Section 5: a type 0x01 Route Origin, IPv4 address form.
			name: "route origin IPv4 address",
			comm: ExtendedCommunity{0x01, 0x03, 0xc0, 0x00, 0x02, 0x02, 0x00, 0xc8},
			want: "origin:192.0.2.2:200",
		},
		{
			// RFC 4360 Section 5: a type 0x02 Route Origin, four-octet AS form.
			name: "route origin four-octet AS",
			comm: ExtendedCommunity{0x02, 0x03, 0x00, 0x01, 0x00, 0x01, 0x00, 0xc8},
			want: "origin:65537:200",
		},
		{
			name: "rt-redirect two-octet AS",
			comm: ExtendedCommunity{0x80, 0x08, 0xfd, 0xe8, 0x00, 0x00, 0x03, 0xe7},
			want: "redirect:65000:999",
		},
		{
			// RFC 8955 Section 7.4: type 0x81 reuses the RFC 4360 Section 3.2
			// IPv4 address layout.
			name: "rt-redirect IPv4 address",
			comm: ExtendedCommunity{0x81, 0x08, 0xc0, 0x00, 0x02, 0x03, 0x03, 0xe7},
			want: "redirect:192.0.2.3:999",
		},
		{
			// RFC 8955 Section 7.4: type 0x82 reuses the RFC 5668 Section 2
			// four-octet AS layout.
			name: "rt-redirect four-octet AS",
			comm: ExtendedCommunity{0x82, 0x08, 0x00, 0x01, 0x00, 0x02, 0x03, 0xe7},
			want: "redirect:65538:999",
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
			// RFC 8955 Section 7.3 Figure 5: S (bit 46) and T (bit 47) are the
			// last two bits of the Traffic Action Field, so 0x03 sets both.
			name: "traffic-action sample and terminal",
			comm: ExtendedCommunity{0x80, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03},
			want: "traffic-action:sample-terminal",
		},
		{
			name: "traffic-action no bit set",
			comm: ExtendedCommunity{0x80, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			want: "traffic-action:none",
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

// TestExtendedCommunityNamedFormRoundTrip parses each named administrator form
// and decodes the community it built.
//
// VALIDATES: parseSingleExtCommunity (builder_parse.go) and AppendDecoded
// (extcomm_decoded.go) spell one community the same way, in all three
// administrator forms RFC 4360 Sections 4 and 5 name.
//
// PREVENTS: the encode and decode halves parting again. The IPv4 and
// four-octet-AS forms were parseable and not renderable, so a four-byte-ASN
// deployment wrote "target:65536:100" into its config and read hex back out of
// `ze bgp decode` and the event JSON.
func TestExtendedCommunityNamedFormRoundTrip(t *testing.T) {
	for _, spelling := range []string{
		"target:65000:1",
		"target:192.0.2.1:100",
		"target:65536:100",
		"origin:100:2",
		"origin:192.0.2.2:200",
		"origin:65537:200",
	} {
		t.Run(spelling, func(t *testing.T) {
			comm, err := parseSingleExtCommunity(spelling)
			require.NoError(t, err)
			assert.Equal(t, spelling, comm.String())
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

// TestRFC8955TrafficActionBitsDecoded drives the traffic-action decoder with
// each combination of the two defined bits, first with the reserved bits clear
// and then with every one of them set.
//
// VALIDATES: AppendDecoded (extcomm_decoded.go) reads S (Sample, bit 46) and T
// (Terminal Action, bit 47) out of the Traffic Action Field and spells each set
// bit, and reads nothing else out of the field.
//
// PREVENTS: the two bits an operator acts on never reaching a surface. A
// renderer that names the community and drops the value cannot tell "ignores
// the reserved bits" from "ignores every bit", so the pairing below is what
// makes the second assertion mean anything: the defined bits MUST change the
// render, and the reserved bits MUST NOT.
func TestRFC8955TrafficActionBitsDecoded(t *testing.T) {
	// RFC 8955 Section 7.3: "T Terminal Action (bit 47) ... S Sample (bit 46)",
	// drawn as |S|T| in Figure 5, so T is 0x01 and S is 0x02 of the last octet.
	// RFC requirement: RFC8955-7.3-1 positive -- the defined traffic-action bits decode to their names (§7.3)
	assert.Equal(t, "traffic-action:none",
		ExtendedCommunity{0x80, 0x07, 0, 0, 0, 0, 0, 0x00}.String())
	assert.Equal(t, "traffic-action:terminal",
		ExtendedCommunity{0x80, 0x07, 0, 0, 0, 0, 0, 0x01}.String())
	assert.Equal(t, "traffic-action:sample",
		ExtendedCommunity{0x80, 0x07, 0, 0, 0, 0, 0, 0x02}.String())
	assert.Equal(t, "traffic-action:sample-terminal",
		ExtendedCommunity{0x80, 0x07, 0, 0, 0, 0, 0, 0x03}.String())

	// RFC 8955 Section 7.3: the other Traffic Action Field bits "MUST be set to
	// 0 on encoding and MUST be ignored during decoding". Each community below
	// carries every reserved bit set above the same two defined bits, so its
	// render must equal the clean render above it.
	// RFC requirement: RFC8955-7.3-1 negative -- the reserved traffic-action bits do not change the decode (§7.3)
	reserved := []struct {
		comm ExtendedCommunity
		want string
	}{
		{ExtendedCommunity{0x80, 0x07, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfc}, "traffic-action:none"},
		{ExtendedCommunity{0x80, 0x07, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfd}, "traffic-action:terminal"},
		{ExtendedCommunity{0x80, 0x07, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe}, "traffic-action:sample"},
		{ExtendedCommunity{0x80, 0x07, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, "traffic-action:sample-terminal"},
	}
	for _, tt := range reserved {
		assert.Equal(t, tt.want, tt.comm.String(),
			"the reserved Traffic Action Field bits must not change the decode")
	}
}

// TestExtendedCommunityAppendDecodedAllocatesNothing runs every render arm
// through testing.AllocsPerRun with a buffer the caller owns.
//
// VALIDATES: the "It allocates nothing" contract on AppendDecoded
// (extcomm_decoded.go). This renderer runs on the receive path, once for each
// extended community of each UPDATE, so an allocation here is paid for each
// message rather than once.
//
// PREVENTS: a render arm reaching for fmt, for a string concatenation, or for a
// value that escapes to the heap. The IPv4 arms are the ones at risk: they
// build a netip.Addr to append a dotted quad.
func TestExtendedCommunityAppendDecodedAllocatesNothing(t *testing.T) {
	comms := []ExtendedCommunity{
		{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01}, // target, two-octet AS
		{0x01, 0x02, 0xc0, 0x00, 0x02, 0x01, 0x00, 0x64}, // target, IPv4 address
		{0x02, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00, 0x64}, // target, four-octet AS
		{0x01, 0x03, 0xc0, 0x00, 0x02, 0x02, 0x00, 0xc8}, // origin, IPv4 address
		{0x81, 0x08, 0xc0, 0x00, 0x02, 0x03, 0x03, 0xe7}, // redirect, IPv4 address
		{0x80, 0x06, 0x00, 0x00, 0x44, 0x7a, 0x00, 0x00}, // rate-limit
		{0x80, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03}, // traffic-action
		{0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x2e}, // mark
		{0x00, 0xff, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, // the generic hex arm
	}

	// 48 octets is the scratch size the CLI and the event JSON give this
	// renderer (cli/decode_extcomm.go, String below), so measure at that size.
	buf := make([]byte, 0, 48)
	for _, comm := range comms {
		allocs := testing.AllocsPerRun(100, func() {
			buf = comm.AppendDecoded(buf[:0])
		})
		assert.Zero(t, allocs, "AppendDecoded must not allocate for %x", comm)
	}
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
