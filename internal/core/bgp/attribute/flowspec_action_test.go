// VALIDATES: the colon-less FlowSpec action keywords resolve to the RFC 8955 /
//            RFC 5575bis wire values, from ONE table shared by both parsers.
// PREVENTS:  the two extended-community vocabularies drifting again. The config
//            path accepted `copy-to-nexthop` while the `update text` API path
//            answered "invalid extended community format", so a route an operator
//            could write in config could not be expressed through the API --
//            and test/plugin/flowspec-announce.ci only noticed intermittently, because its
//            wire assertions were satisfied by the static route in the same config.

package attribute

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlowSpecActionKeywordWireValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		want ExtendedCommunity
		why  string
	}{
		{
			"copy-to-nexthop",
			ExtendedCommunity{0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
			"RFC 5575bis: copy and redirect to next-hop, value 1",
		},
		{
			"redirect-to-nexthop-draft",
			ExtendedCommunity{0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			"pre-IETF draft: redirect to next-hop, value 0",
		},
		{
			"discard",
			ExtendedCommunity{0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			"RFC 8955 Section 7.3: traffic-rate 0 means discard",
		},
	} {
		got, ok := FlowSpecActionKeyword(tc.name)
		require.Truef(t, ok, "%s must be a recognized FlowSpec action keyword", tc.name)
		require.Equalf(t, tc.want, got, "%s wire value (%s)", tc.name, tc.why)
	}
}

// copy-to-nexthop and redirect-to-nexthop-draft differ in ONE bit of the last
// octet. Pinned separately because a copy/paste between the two entries would
// otherwise silently turn a copy action into a plain redirect, which drops the
// traffic the operator asked to keep.
func TestCopyAndRedirectToNexthopDifferOnlyInTheCopyBit(t *testing.T) {
	copyTo, ok := FlowSpecActionKeyword("copy-to-nexthop")
	require.True(t, ok)
	redirect, ok := FlowSpecActionKeyword("redirect-to-nexthop-draft")
	require.True(t, ok)

	require.Equal(t, copyTo[:7], redirect[:7], "type, subtype and reserved octets must match")
	require.Equal(t, byte(0x01), copyTo[7], "the copy semantic is the low bit")
	require.Equal(t, byte(0x00), redirect[7])
}

func TestFlowSpecActionKeywordRejectsEverythingElse(t *testing.T) {
	for _, name := range []string{"", "target:65000:1", "rate-limit:1000", "COPY-TO-NEXTHOP", "copy-to-nexthop "} {
		_, ok := FlowSpecActionKeyword(name)
		require.Falsef(t, ok, "%q must not resolve as a FlowSpec action keyword", name)
	}
}

// The error message a parser builds from this must name what IS accepted, and it
// must stay derived from the table rather than hand-listed.
func TestFlowSpecActionKeywordsIsSortedAndComplete(t *testing.T) {
	got := FlowSpecActionKeywords()
	require.Equal(t, []string{"copy-to-nexthop", "discard", "redirect-to-nexthop-draft"}, got)
	require.Len(t, got, len(flowSpecActionKeywords), "every table entry must appear in the diagnostic")
}

// TestParseFlowSpecRedirectToIP verifies the one reader of the two-word
// redirect-to-IP actions that both the config parser and `update text` call.
//
// VALIDATES: `redirect-to-nexthop <ip>` and `copy-to-nexthop <ip>` name the
// draft-ietf-idr-flowspec-redirect-ip-16 community, with the copy bit clear and
// set; an IPv4 address gives the 8-octet community (attribute 16) and an IPv6
// one the 20-octet community (attribute 25). `copy-to-nexthop` followed by
// something that is not an address is not this form, because the bare word is
// the 0x0800 copy action; `redirect-to-nexthop` has no one-word meaning, so it
// is refused with no address.
// PREVENTS: the config and `update text` reading the words differently, which
// is what they did: only the config knew the address form.
func TestParseFlowSpecRedirectToIP(t *testing.T) {
	for _, tc := range []struct {
		word, next string
		want4      string
		want6      string
	}{
		{"redirect-to-nexthop", "192.0.2.1", "010cc00002010000", ""},
		{"copy-to-nexthop", "192.0.2.1", "010cc00002010001", ""},
		{"redirect-to-nexthop", "2001:db8::1", "", "000c20010db80000000000000000000000010000"},
		{"copy-to-nexthop", "2001:db8::1", "", "000c20010db80000000000000000000000010001"},
	} {
		action, matched, err := ParseFlowSpecRedirectToIP(tc.word, tc.next)
		require.NoError(t, err, "%s %s", tc.word, tc.next)
		require.True(t, matched, "%s %s", tc.word, tc.next)
		v4, v6, err := action.AppendTo(nil, nil)
		require.NoError(t, err)
		if tc.want4 != "" {
			require.Len(t, v4, 1)
			require.Empty(t, v6)
			require.Equal(t, tc.want4, hex.EncodeToString(v4[0][:]))
			continue
		}
		require.Empty(t, v4)
		require.Len(t, v6, 1)
		require.Equal(t, tc.want6, hex.EncodeToString(v6[0][:]))
	}

	for _, tc := range []struct{ word, next string }{
		{"copy-to-nexthop", ""},
		{"copy-to-nexthop", "target:65000:1"},
		{"redirect-to-nexthop-draft", "192.0.2.1"},
		{"discard", "192.0.2.1"},
		{"target:65000:1", "192.0.2.1"},
	} {
		_, matched, err := ParseFlowSpecRedirectToIP(tc.word, tc.next)
		require.NoError(t, err, "%q %q", tc.word, tc.next)
		require.False(t, matched, "%q %q is not the two-word form", tc.word, tc.next)
	}

	for _, next := range []string{"", "target:65000:1", "not-an-address"} {
		_, matched, err := ParseFlowSpecRedirectToIP("redirect-to-nexthop", next)
		require.True(t, matched, "redirect-to-nexthop %q", next)
		require.Error(t, err, "redirect-to-nexthop %q", next)
		require.Contains(t, err.Error(), "redirect-to-nexthop")
	}
}
