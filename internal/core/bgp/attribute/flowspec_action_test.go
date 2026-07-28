// VALIDATES: the colon-less FlowSpec action keywords resolve to the RFC 8955 /
//            RFC 5575bis wire values, from ONE table shared by both parsers.
// PREVENTS:  the two extended-community vocabularies drifting again. The config
//            path accepted `copy-to-nexthop` while the `update text` API path
//            answered "invalid extended community format", so a route an operator
//            could write in config could not be expressed through the API --
//            and test/plugin/flowspec.ci only noticed intermittently, because its
//            wire assertions were satisfied by the static route in the same config.

package attribute

import (
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
