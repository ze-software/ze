package mup

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wire fixtures, taken from test/exabgp-compat/encoding/conf-srv6-mup.ci and
// conf-srv6-mup-v3.ci, which hold the octets ExaBGP and ze exchange for each
// route type. Every one carries RD 0:100:100.
const (
	wireISDv4        = "0100010C0000006400000064180A0001"                                                                                                         // 10.0.1.0/24
	wireDSDv4        = "0100020C00000064000000640A000001"                                                                                                         // 10.0.0.1
	wireT1STv4       = "01000317000000640000006420C0A800010000303909200A000001"                                                                                   // no source address
	wireT1STv4Source = "0100031C000000640000006420C0A800020000303909200A000001200A000101"                                                                         // source 10.0.1.1
	wireT2STv4       = "010004110000006400000064400A00000100003039"                                                                                               // endpoint length 64
	wireT2STv4NoTEID = "0100040D0000006400000064200A000001"                                                                                                       // endpoint length 32
	wireISDv6        = "010001110000006400000064402001000000000000"                                                                                               // 2001::/64
	wireT1STv6       = "0100032F00000064000000648020010DB800010001000000000000000100003039098020010000000000000000000000000001"                                   // no source address
	wireT2STv6       = "0100041D0000006400000064A02001000000000000000000000000000100003039"                                                                       //nolint:lll // one NLRI per line reads better than a wrapped fixture
	wireUnknownType  = "010063040A0B0C0D"                                                                                                                         // route type 99, four octets of body
	wireT1STv6Source = "0100034000000064000000648020010DB8000100010000000000000002000030390980200100000000000000000000000000018020020000000000000000000000000002" //nolint:lll // same
)

// parseHex parses a hex NLRI and fails the test if it does not parse.
func parseHex(t *testing.T, afi AFI, wire string) *MUP {
	t.Helper()
	data, err := hex.DecodeString(wire)
	require.NoError(t, err, "fixture is not hex")
	mup, rest, err := ParseMUP(afi, data)
	require.NoError(t, err)
	require.Empty(t, rest, "one NLRI must consume the whole fixture")
	return mup
}

// TestMUPTypes verifies MUP route types.
func TestMUPTypes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, MUPRouteType(1), MUPISD)
	assert.Equal(t, MUPRouteType(2), MUPDSD)
	assert.Equal(t, MUPRouteType(3), MUPT1ST)
	assert.Equal(t, MUPRouteType(4), MUPT2ST)
}

// TestMUPFamily verifies MUP address family.
func TestMUPFamily(t *testing.T) {
	t.Parallel()
	assert.Equal(t, AFIIPv4, parseHex(t, AFIIPv4, wireISDv4).Family().AFI)
	assert.Equal(t, AFIIPv6, parseHex(t, AFIIPv6, wireISDv6).Family().AFI)
	assert.Equal(t, SAFIMUP, parseHex(t, AFIIPv4, wireISDv4).Family().SAFI)
}

// TestMUPHeaderFields verifies the three header fields every route type shares.
//
// VALIDATES: ParseMUP reads the architecture type, the route type and the RD of
// each of the four route types.
// PREVENTS: a route type being read under the wrong header offsets, which would
// give every MUP route the same identity.
func TestMUPHeaderFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		afi  AFI
		wire string
		rt   MUPRouteType
	}{
		{"isd ipv4", AFIIPv4, wireISDv4, MUPISD},
		{"dsd ipv4", AFIIPv4, wireDSDv4, MUPDSD},
		{"t1st ipv4", AFIIPv4, wireT1STv4, MUPT1ST},
		{"t2st ipv4", AFIIPv4, wireT2STv4, MUPT2ST},
		{"isd ipv6", AFIIPv6, wireISDv6, MUPISD},
		{"t1st ipv6", AFIIPv6, wireT1STv6, MUPT1ST},
		{"t2st ipv6", AFIIPv6, wireT2STv6, MUPT2ST},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mup := parseHex(t, tc.afi, tc.wire)
			assert.Equal(t, MUPArch3GPP5G, mup.ArchType())
			assert.Equal(t, tc.rt, mup.RouteType())
			assert.True(t, mup.parsed)
			assert.Equal(t, "0:100:100", mup.RD().String())
		})
	}
}

// TestMUPRoundTrip verifies that re-encoding a parsed NLRI reproduces the
// octets it was parsed from.
//
// VALIDATES: WriteTo and Bytes reproduce the received wire bytes for every
// route type, the unknown one included.
// PREVENTS: a relayed MUP route reaching the peer with different octets than
// the one ze received, which no receiver would match to the original.
func TestMUPRoundTrip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		afi  AFI
		wire string
	}{
		{"isd ipv4", AFIIPv4, wireISDv4},
		{"dsd ipv4", AFIIPv4, wireDSDv4},
		{"t1st ipv4", AFIIPv4, wireT1STv4},
		{"t1st ipv4 with source", AFIIPv4, wireT1STv4Source},
		{"t2st ipv4", AFIIPv4, wireT2STv4},
		{"t2st ipv4 without teid", AFIIPv4, wireT2STv4NoTEID},
		{"isd ipv6", AFIIPv6, wireISDv6},
		{"t1st ipv6", AFIIPv6, wireT1STv6},
		{"t1st ipv6 with source", AFIIPv6, wireT1STv6Source},
		{"t2st ipv6", AFIIPv6, wireT2STv6},
		{"unknown route type", AFIIPv4, wireUnknownType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want, err := hex.DecodeString(tc.wire)
			require.NoError(t, err)

			mup := parseHex(t, tc.afi, tc.wire)
			assert.Equal(t, len(want), mup.Len())
			assert.Equal(t, want, mup.Bytes())

			buf := make([]byte, mup.Len()+10)
			n := mup.WriteTo(buf, 0)
			assert.Equal(t, len(want), n, "WriteTo returned wrong length")
			assert.Equal(t, want, buf[:n], "WriteTo output differs from the received octets")
		})
	}
}

// TestMUPParseErrors verifies that a body which does not add up is refused.
//
// VALIDATES: ParseMUP returns an error, and no NLRI, for a truncated header, a
// truncated body, a prefix length past the AFI width, a length that disagrees
// with the octets present, an address length the AFI does not allow, an
// endpoint or source address length outside 32 and 128, a zero TEID and a TLV
// that does not fit.
// PREVENTS: a half-filled route reaching a reader, where a zero prefix length
// or an unset endpoint cannot be told from a field ze failed to read.
func TestMUPParseErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		afi  AFI
		wire string
	}{
		{"empty", AFIIPv4, ""},
		{"truncated header", AFIIPv4, "01"},
		{"truncated body", AFIIPv4, "01000110"},
		{"body shorter than the RD", AFIIPv4, "010001020000"},
		{"isd prefix length above 32 under ipv4", AFIIPv4, "0100010C0000006400000064280A000100"},
		{"isd prefix length above 128 under ipv6", AFIIPv6, "010001110000006400000064812001000000000000"},
		{"isd carries an octet past the prefix", AFIIPv4, "0100010D0000006400000064180A000100"},
		{"isd prefix shorter than its length", AFIIPv4, "0100010B0000006400000064180A00"},
		{"dsd address is not 4 octets under ipv4", AFIIPv4, "0100020B00000064000000640A0000"},
		{"dsd address is not 16 octets under ipv6", AFIIPv6, "0100020C00000064000000640A000001"},
		{"t1st endpoint length is neither 32 nor 128", AFIIPv4, "01000317000000640000006420C0A800010000303909400A000001"},
		{"t1st source length is neither 32 nor 128", AFIIPv4, "0100031C000000640000006420C0A800020000303909200A000001400A000101"},
		{"t1st teid is zero", AFIIPv4, "01000317000000640000006420C0A800010000000009200A000001"},
		{"t1st truncated endpoint address", AFIIPv4, "01000316000000640000006420C0A800010000303909200A0000"},
		{"t1st trailing octet cannot hold a tlv", AFIIPv4, "0100031D000000640000006420C0A800020000303909200A000001200A00010100"},
		{"t1st tlv longer than the octets present", AFIIPv4, "0100031E000000640000006420C0A800020000303909200A000001200A0001010104"},
		{"t2st endpoint length below the address width", AFIIPv4, "0100040D0000006400000064100A000001"},
		{"t2st endpoint length leaves more than a 4-octet teid", AFIIPv4, "010004120000006400000064610A0000010000303901"},
		{"t2st teid is zero", AFIIPv4, "010004110000006400000064400A00000100000000"},
		{"t2st truncated teid", AFIIPv4, "010004100000006400000064400A0000010000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data, err := hex.DecodeString(tc.wire)
			require.NoError(t, err, "fixture is not hex")
			mup, rest, parseErr := ParseMUP(tc.afi, data)
			assert.Error(t, parseErr)
			assert.Nil(t, mup, "a refused NLRI must not be returned half filled")
			assert.Nil(t, rest)
		})
	}
}

// TestMUPUnknownRouteTypeKeepsOctets verifies an unimplemented route type is
// carried rather than read.
//
// VALIDATES: ParseMUP reports Parsed false, keeps the octets for WriteTo, and
// leaves the RD at its zero value for a route type outside Implemented.
// PREVENTS: an unknown route type being reported with an RD read out of octets
// that mean something else.
func TestMUPUnknownRouteTypeKeepsOctets(t *testing.T) {
	t.Parallel()
	mup := parseHex(t, AFIIPv4, wireUnknownType)
	assert.False(t, mup.parsed)
	assert.Equal(t, MUPRouteType(99), mup.RouteType())
	assert.Equal(t, RouteDistinguisher{}, mup.RD())
	assert.Equal(t, "type(99)", mup.String())
}

// TestMUPStringCommandStyle verifies command-style string representation.
//
// VALIDATES: MUP String() outputs command-style format for API round-trip.
// PREVENTS: Output format not matching input parser, breaking round-trip.
func TestMUPStringCommandStyle(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "isd rd 0:100:100", parseHex(t, AFIIPv4, wireISDv4).String())
	assert.Equal(t, "t1st rd 0:100:100", parseHex(t, AFIIPv4, wireT1STv4).String())
	assert.Equal(t, "dsd rd 0:100:100", parseHex(t, AFIIPv4, wireDSDv4).String())
}
