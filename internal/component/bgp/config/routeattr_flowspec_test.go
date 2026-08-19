// Design: routeattr_community.go — the FlowSpec half of extended-community parsing
// RFC: rfc/short/rfc8955.md — traffic filtering actions (Section 7)

package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseExtendedCommunityMarkDSCPBound verifies that a `mark N` action encodes
// only into the six DSCP bits, and that a value which would reach the reserved
// bits is refused instead of truncated.
//
// The entry point is the one an operator's `extended-community` config line
// reaches. The helper below it had no test of any kind, which is how it kept a
// discarded strconv error for as long as it did.
//
// VALIDATES: `mark 63` and `mark 0` encode as 0x80,0x09 with five zero octets and
// the DSCP last; `mark 64`, `mark 200`, `mark 300` and `mark abc` are errors.
//
// PREVENTS: an out-of-range DSCP being written into the RFC 8955 Section 7.5
// reserved bits, and a discarded parse error remarking every matching packet to
// best-effort. `mark 300` used to encode 0xff, because strconv answers MaxUint64
// on a range error and the error was thrown away; `mark abc` used to encode 0x00.
func TestParseExtendedCommunityMarkDSCPBound(t *testing.T) {
	t.Parallel()

	// RFC 8955 Section 7.5: the DSCP is carried "in the 6 least significant bits
	// of the Extended Community value", and "reserved (r): MUST be set to 0 on
	// encoding and MUST be ignored during decoding".
	// RFC requirement: RFC8955-7.5-1 positive -- an in-range DSCP encodes with every reserved octet and both reserved bits zero (§7.5)
	for _, dscp := range []struct {
		spec string
		want byte
	}{
		{"0", 0x00}, // CS0 is a real value, not the absence of a marking.
		{"46", 0x2e},
		{"63", 0x3f}, // Last valid: every DSCP bit set, both reserved bits clear.
	} {
		ec, err := ParseExtendedCommunity("[ mark " + dscp.spec + " ]")
		require.NoError(t, err, "mark %s", dscp.spec)
		require.Len(t, ec.Bytes, 8, "mark %s", dscp.spec)

		assert.Equal(t, byte(0x80), ec.Bytes[0], "mark %s: type octet", dscp.spec)
		assert.Equal(t, byte(0x09), ec.Bytes[1], "mark %s: traffic-marking sub-type", dscp.spec)
		for i := 2; i <= 6; i++ {
			assert.Zero(t, ec.Bytes[i], "mark %s: reserved octet %d must be 0", dscp.spec, i)
		}
		assert.Equal(t, dscp.want, ec.Bytes[7], "mark %s: DSCP octet", dscp.spec)
		assert.Zero(t, ec.Bytes[7]&0xC0, "mark %s: the two reserved bits must be 0", dscp.spec)
	}

	// RFC requirement: RFC8955-7.5-1 negative -- a DSCP whose bits reach the reserved field is refused rather than encoded (§7.5)
	for _, spec := range []string{"64", "200", "300", "abc", "-1", ""} {
		_, err := ParseExtendedCommunity("[ mark " + spec + " ]")
		require.Error(t, err, "mark %q must be refused", spec)
	}
}

// TestParseExtendedCommunityRateLimitRefusesNonFinite verifies that a rate which
// is not a finite number is refused where the operator wrote it.
//
// VALIDATES: `rate-limit:NaN`, `rate-limit:Inf` and their packets forms are
// errors, while `rate-limit:0` still encodes the all-zero rate that RFC 8955
// Section 7.1 gives the meaning "discard".
//
// PREVENTS: a NaN reaching the wire as 0x7fc00000. RFC 8955 forbids only a
// negative rate, so a NaN passed the sign check; Ze's own decoder reads a NaN
// back as zero, so an accepted `rate-limit:NaN` turns a rate limit into a
// blackhole with nothing said. There is no RFC requirement tag here on purpose:
// the RFC states no obligation about a non-finite rate, so this is Ze's decision
// and it is recorded as one rather than dressed as conformance.
func TestParseExtendedCommunityRateLimitRefusesNonFinite(t *testing.T) {
	t.Parallel()

	for _, spec := range []string{"NaN", "Inf", "-Inf", "+Inf", "nan"} {
		_, err := ParseExtendedCommunity("[ rate-limit:" + spec + " ]")
		require.Error(t, err, "rate-limit:%s must be refused", spec)

		_, err = ParseExtendedCommunity("[ rate-limit:" + spec + ":packets ]")
		require.Error(t, err, "rate-limit:%s:packets must be refused", spec)
	}

	// A zero rate stays legal: RFC 8955 Section 7.1 gives it the discard meaning.
	ec, err := ParseExtendedCommunity("[ rate-limit:0 ]")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, ec.Bytes)
}

// TestParseExtendedCommunityActionRejectsUnknownWord verifies that an action word
// outside the RFC 8955 Section 7.3 vocabulary is refused.
//
// VALIDATES: `action sample`, `action terminal`, `action sample-terminal` and
// `action none` encode their bits; `action bogus` and `action termnal` are errors.
//
// PREVENTS: a typo silently producing a traffic-action community with both bits
// clear, which is an action the operator never asked for. The old parser looked
// for the substrings "sample" and "terminal" anywhere in the word and emitted the
// community whatever it found.
func TestParseExtendedCommunityActionRejectsUnknownWord(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		spec string
		want byte
	}{
		{"none", 0x00},
		{"terminal", 0x01},
		{"sample", 0x02},
		{"sample-terminal", 0x03},
		{"terminal-sample", 0x03},
	} {
		ec, err := ParseExtendedCommunity("[ action " + tc.spec + " ]")
		require.NoError(t, err, "action %s", tc.spec)
		require.Len(t, ec.Bytes, 8, "action %s", tc.spec)
		assert.Equal(t, tc.want, ec.Bytes[7], "action %s: flag octet", tc.spec)
	}

	for _, spec := range []string{"bogus", "termnal", "sample-bogus", "resample"} {
		_, err := ParseExtendedCommunity("[ action " + spec + " ]")
		require.Error(t, err, "action %q must be refused", spec)
	}
}

// TestParseExtendedCommunityRedirectEncodings verifies that the administrator of
// an rt-redirect selects which of the three RFC 8955 Section 7.4 encodings the
// community takes.
//
// VALIDATES: a 2-octet AS gives type 0x80 with a 4-octet value, an IPv4 address
// gives type 0x81 with a 2-octet value, and a 4-octet AS gives type 0x82 with a
// 2-octet value; a value too wide for its form is refused.
//
// PREVENTS: the type 0x81 form staying unimplemented, and a value being truncated
// into a form that cannot hold it, which would name a different VRF.
func TestParseExtendedCommunityRedirectEncodings(t *testing.T) {
	t.Parallel()

	// RFC 8955 Section 7.4: "It uses the same encoding as the Route Target
	// Extended Community in Sections 3.1 (type 0x80: 2-octet AS, 4-octet value),
	// 3.2 (type 0x81: 4-octet IPv4 address, 2-octet value), and 4 of [RFC4360]
	// and Section 2 of [RFC5668] (type 0x82: 4-octet AS, 2-octet value)".
	for _, tc := range []struct {
		spec string
		want []byte
	}{
		{"redirect:65000:33756718", []byte{0x80, 0x08, 0xfd, 0xe8, 0x02, 0x03, 0x16, 0x2e}},
		{"redirect:192.0.2.1:100", []byte{0x81, 0x08, 0xc0, 0x00, 0x02, 0x01, 0x00, 0x64}},
		{"redirect:4200000000:100", []byte{0x82, 0x08, 0xfa, 0x56, 0xea, 0x00, 0x00, 0x64}},
	} {
		ec, err := ParseExtendedCommunity("[ " + tc.spec + " ]")
		require.NoError(t, err, tc.spec)
		assert.Equal(t, tc.want, ec.Bytes, tc.spec)
	}

	for _, spec := range []string{
		"redirect:192.0.2.1:65536",   // IPv4 form holds 2 octets.
		"redirect:4200000000:65536",  // 4-octet AS form holds 2 octets.
		"redirect:65000:4294967296",  // 2-octet AS form holds 4 octets.
		"redirect:2001:db8::1:100",   // No IPv6 rt-redirect exists in Section 7.4.
		"redirect:not-an-address:10", // Neither an AS nor an address.
	} {
		_, err := ParseExtendedCommunity("[ " + spec + " ]")
		require.Error(t, err, "%s must be refused", spec)
	}
}

// TestParseExtendedCommunityRedirectToNextHopIPv6 verifies that an IPv6
// redirect-to-nexthop reaches the 20-octet IPv6-address-specific community, and
// that it lands in the field the caller puts in attribute 25.
//
// VALIDATES: `redirect-to-nexthop <IPv6>` fills IPv6Bytes with the RFC 5701
// Section 2 layout and leaves Bytes empty; `redirect-to-nexthop <IPv4>` does the
// reverse.
//
// PREVENTS: the 20-octet encoder staying unreachable. Two places used to decide
// which form the string named, and this one refused the route before the other
// ran, so attribute 25 could be configured by nobody.
func TestParseExtendedCommunityRedirectToNextHopIPv6(t *testing.T) {
	t.Parallel()

	ec, err := ParseExtendedCommunity("[ redirect-to-nexthop 2001:db8::1 ]")
	require.NoError(t, err)
	assert.Empty(t, ec.Bytes, "an IPv6 next hop does not fit the 8-octet form")
	require.Len(t, ec.IPv6Bytes, 20, "RFC 5701 Section 2: each community is 20 octets")

	// RFC 5701 Section 2: a transitive sub-type has 0x00 as its first octet, then
	// the sub-type, then a 16-octet global administrator, then a 2-octet local
	// administrator.
	assert.Equal(t, []byte{
		0x00, 0x0c,
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x00,
	}, ec.IPv6Bytes)

	ec, err = ParseExtendedCommunity("[ redirect-to-nexthop 1.2.3.4 ]")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x0c, 0x01, 0x02, 0x03, 0x04, 0x00, 0x00}, ec.Bytes)
	assert.Empty(t, ec.IPv6Bytes, "an IPv4 next hop does not use attribute 25")

	_, err = ParseExtendedCommunity("[ redirect-to-nexthop not-an-address ]")
	require.Error(t, err, "an unparseable next hop must be refused, not ignored")
}
