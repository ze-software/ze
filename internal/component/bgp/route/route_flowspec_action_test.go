// Design: route_community.go — the FlowSpec half of `update text` attribute parsing
// RFC: rfc/short/rfc8955.md — traffic filtering actions (Section 7)

package route

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestParseExtendedCommunitiesRedirectEncodings verifies that the API parser
// reaches all three RFC 8955 Section 7.4 rt-redirect encodings, in both the
// function form and the list form.
//
// VALIDATES: `redirect <admin> <value>` and `redirect:<admin>:<value>` produce
// type 0x80 for a 2-octet AS, 0x81 for an IPv4 address, and 0x82 for a 4-octet
// AS; a value too wide for its form is refused.
//
// PREVENTS: the API refusing a redirect the config parser accepts. This parser
// read the administrator with a 16-bit strconv, so a 4-octet AS was rejected
// outright here and encoded as type 0x82 in config, and the type 0x81 form
// existed in neither.
func TestParseExtendedCommunitiesRedirectEncodings(t *testing.T) {
	t.Parallel()

	// RFC 8955 Section 7.4: "It uses the same encoding as the Route Target
	// Extended Community in Sections 3.1 (type 0x80: 2-octet AS, 4-octet value),
	// 3.2 (type 0x81: 4-octet IPv4 address, 2-octet value), and 4 of [RFC4360]
	// and Section 2 of [RFC5668] (type 0x82: 4-octet AS, 2-octet value)".
	for _, tc := range []struct {
		name string
		args []string
		want attribute.ExtendedCommunity
	}{
		{
			name: "function form, 2-octet AS",
			args: []string{"redirect", "65000", "33756718"},
			want: attribute.ExtendedCommunity{0x80, 0x08, 0xfd, 0xe8, 0x02, 0x03, 0x16, 0x2e},
		},
		{
			name: "function form, IPv4 address",
			args: []string{"redirect", "192.0.2.1", "100"},
			want: attribute.ExtendedCommunity{0x81, 0x08, 0xc0, 0x00, 0x02, 0x01, 0x00, 0x64},
		},
		{
			name: "function form, 4-octet AS",
			args: []string{"redirect", "4200000000", "100"},
			want: attribute.ExtendedCommunity{0x82, 0x08, 0xfa, 0x56, 0xea, 0x00, 0x00, 0x64},
		},
		{
			name: "list form, IPv4 address",
			args: []string{"[redirect:192.0.2.1:100]"},
			want: attribute.ExtendedCommunity{0x81, 0x08, 0xc0, 0x00, 0x02, 0x01, 0x00, 0x64},
		},
		{
			name: "list form, 4-octet AS",
			args: []string{"[redirect:4200000000:100]"},
			want: attribute.ExtendedCommunity{0x82, 0x08, 0xfa, 0x56, 0xea, 0x00, 0x00, 0x64},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			comms, _, err := ParseExtendedCommunities(tc.args)
			require.NoError(t, err)
			require.Len(t, comms, 1)
			assert.Equal(t, tc.want, comms[0])
		})
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"IPv4 form value above 65535", []string{"redirect", "192.0.2.1", "65536"}},
		{"4-octet AS form value above 65535", []string{"redirect", "4200000000", "65536"}},
		{"administrator is neither an AS nor an address", []string{"redirect", "wat", "1"}},
		{"IPv6 has no rt-redirect encoding", []string{"redirect", "2001:db8::1", "1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := ParseExtendedCommunities(tc.args)
			require.Error(t, err)
		})
	}
}

// TestParseExtendedCommunitiesTrafficRateRefusesNonFinite verifies that a rate
// which is not a finite number is refused by the API parser.
//
// VALIDATES: `traffic-rate 0 NaN` and the rate-limit list forms are errors, while
// a zero rate still encodes as the all-zero IEEE 754 value.
//
// PREVENTS: 0x7fc00000 reaching the wire as a rate. RFC 8955 forbids a negative
// rate, and a NaN is not negative, so the sign check passed it. No RFC
// requirement tag: the RFC states no obligation about a non-finite rate, and
// tagging this would claim a conformance the document does not ask for.
func TestParseExtendedCommunitiesTrafficRateRefusesNonFinite(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"traffic-rate", "0", "NaN"},
		{"traffic-rate", "65000", "NaN", "packets"},
		{"traffic-rate", "0", "Inf"},
		{"[rate-limit:NaN]"},
		{"[rate-limit:NaN:packets]"},
		{"[rate-limit-packets:Inf]"},
	} {
		_, _, err := ParseExtendedCommunities(args)
		require.Error(t, err, "%v must be refused", args)
	}

	// RFC 8955 Section 7.1 gives a traffic-rate of 0 the discard meaning, so it
	// stays legal and must not be caught by the guard above.
	comms, _, err := ParseExtendedCommunities([]string{"traffic-rate", "0", "0"})
	require.NoError(t, err)
	require.Len(t, comms, 1)
	assert.Equal(t, attribute.ExtendedCommunity{0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, comms[0])
}

// TestParseFlowSpecArgsRecordsZeroValuedActions verifies that a zero rate limit
// and a zero DSCP are recorded as requested rather than left indistinguishable
// from an action nobody asked for.
//
// VALIDATES: `then rate-limit 0`, `then rate-limit 0 packets` and `then mark 0`
// each set their companion flag, and an absent action leaves it clear.
//
// PREVENTS: the encoder reading a zero value as "no action". RFC 8955
// Section 7.1 gives a traffic-rate of 0 the meaning "discard all traffic", so
// dropping it turns a discard rule into the default accept.
func TestParseFlowSpecArgsRecordsZeroValuedActions(t *testing.T) {
	t.Parallel()

	r, err := ParseFlowSpecArgs([]string{"match", "destination", "10.0.0.0/8", "then", "rate-limit", "0"})
	require.NoError(t, err)
	assert.True(t, r.Actions.RateLimitSet, "a zero byte rate must be recorded as requested")
	assert.Zero(t, r.Actions.RateLimit)

	r, err = ParseFlowSpecArgs([]string{"match", "destination", "10.0.0.0/8", "then", "rate-limit", "0", "packets"})
	require.NoError(t, err)
	assert.True(t, r.Actions.RateLimitPacketsSet, "a zero packet rate must be recorded as requested")
	assert.False(t, r.Actions.RateLimitSet, "the packets unit must not also set the bytes action")

	r, err = ParseFlowSpecArgs([]string{"match", "destination", "10.0.0.0/8", "then", "mark", "0"})
	require.NoError(t, err)
	assert.True(t, r.Actions.MarkDSCPSet, "DSCP 0 is CS0, a value rather than an absence")
	assert.Zero(t, r.Actions.MarkDSCP)

	r, err = ParseFlowSpecArgs([]string{"match", "destination", "10.0.0.0/8", "then", "discard"})
	require.NoError(t, err)
	assert.False(t, r.Actions.RateLimitSet, "no rate limit was asked for")
	assert.False(t, r.Actions.MarkDSCPSet, "no marking was asked for")

	_, err = ParseFlowSpecArgs([]string{"match", "destination", "10.0.0.0/8", "then", "mark", "64"})
	require.Error(t, err, "a DSCP above the six-bit field must be refused at parse time")
}
