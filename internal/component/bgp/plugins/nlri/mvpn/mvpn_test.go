package mvpn

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sharedJoinHex and sourceJoinHex are the two NLRIs
// test/exabgp-compat/encoding/conf-mvpn.ci packs into one MP_REACH_NLRI:
// a Shared Tree Join and a Source Tree Join, RD 65000:99999, Source AS 65000,
// group 239.251.255.228.
const (
	sharedJoinHex = "06160000FDE80001869F0000FDE8200A63C70120EFFBFFE4"
	sourceJoinHex = "07160000FDE80001869F0000FDE8200A630C0220EFFBFFE4"
)

// TestPackedSectionDecodesEveryRoute drives the two NLRIs the exabgp-compat
// fixture packs into one MP_REACH_NLRI.
//
// VALIDATES: DecodeNLRIHex walks an MCAST-VPN section to exhaustion and
// publishes every route in it, with the members ExaBGP names.
// PREVENTS: the regression recorded in
// plan/journal/validated-value-discarded-by-its-caller.md -- parseMVPN answers
// the unread remainder of the section, and a caller that drops it publishes the
// first route and loses every route behind it in silence.
func TestPackedSectionDecodesEveryRoute(t *testing.T) {
	t.Parallel()

	decoded, err := DecodeNLRIHex("ipv4/mvpn", sharedJoinHex+sourceJoinHex, false)
	require.NoError(t, err)

	raw, err := json.Marshal(decoded)
	require.NoError(t, err)

	var got []map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Len(t, got, 2, "a section of two NLRIs must publish two routes")

	assert.Equal(t, map[string]any{
		"code":      float64(6),
		"parsed":    true,
		"raw":       sharedJoinHex,
		"name":      "C-Multicast Shared Tree Join route",
		"rd":        "0:65000:99999",
		"source-as": "65000",
		"source":    "10.99.199.1",
		"group":     "239.251.255.228",
	}, got[0])

	assert.Equal(t, map[string]any{
		"code":      float64(7),
		"parsed":    true,
		"raw":       sourceJoinHex,
		"name":      "C-Multicast Source Tree Join route",
		"rd":        "0:65000:99999",
		"source-as": "65000",
		"source":    "10.99.12.2",
		"group":     "239.251.255.228",
	}, got[1])
}

// TestSourceActiveCarriesNoSourceAS drives the Source Active A-D route the same
// fixture sends on its own.
//
// VALIDATES: RFC 6514 Section 4.5 gives the Source Active A-D route no Source AS
// field, so the body is read four octets earlier than a C-multicast route's and
// no "source-as" member is published.
// PREVENTS: one offset table for every route type, which would read the source
// address out of the Source AS field.
func TestSourceActiveCarriesNoSourceAS(t *testing.T) {
	t.Parallel()

	const sourceADHex = "05120000FDE80001869F200A630C0420EFFBFFE4"
	decoded, err := DecodeNLRIHex("ipv4/mvpn", sourceADHex, false)
	require.NoError(t, err)

	raw, err := json.Marshal(decoded)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, map[string]any{
		"code":   float64(5),
		"parsed": true,
		"raw":    sourceADHex,
		"name":   "Source Active A-D Route",
		"rd":     "0:65000:99999",
		"source": "10.99.12.4",
		"group":  "239.251.255.228",
	}, got)
}

// TestUnknownRouteTypePublishesItsOctets drives a route type ze does not parse.
//
// VALIDATES: an Intra-AS I-PMSI A-D route (RFC 6514 Section 4.1) is published as
// code, parsed:false and its raw octets, which is what ExaBGP's GenericMVPN
// answers for the same bytes.
// PREVENTS: a half-filled route for a body ze never read, and a reader losing
// the octets it would need to decode the type itself.
func TestUnknownRouteTypePublishesItsOctets(t *testing.T) {
	t.Parallel()

	const intraASHex = "010C0000FDE9000000640A000001"
	decoded, err := DecodeNLRIHex("ipv4/mvpn", intraASHex, false)
	require.NoError(t, err)

	raw, err := json.Marshal(decoded)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, map[string]any{
		"code":   float64(1),
		"parsed": false,
		"raw":    intraASHex,
	}, got)
}

// TestMalformedSectionIsRefused drives each way a section can disagree with
// itself.
//
// VALIDATES: a truncated body, a Length octet that overruns the bytes present, a
// multicast address length that names neither an IPv4 nor an IPv6 address, and a
// body with octets the route type does not define, are each an error.
// PREVENTS: a half-filled route, and a walk that ends quietly on the remainder
// and publishes the routes it read as if the section held only those
// (ai/rules/principles.md).
func TestMalformedSectionIsRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hex  string
	}{
		{"empty section", ""},
		{"header only", "06"},
		{"length overruns the section", "0616" + "0000FDE8"},
		{"shared join body truncated after the source", "060D0000FDE80001869F0000FDE820"},
		{"source length is neither 32 nor 128 bits", "06160000FDE80001869F0000FDE8180A63C70120EFFBFFE4"},
		{"body carries octets the route type does not define", "06170000FDE80001869F0000FDE8200A63C70120EFFBFFE400"},
		{"second route of the section is truncated", sharedJoinHex + "0616" + "0000FDE8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeNLRIHex("ipv4/mvpn", tt.hex, false)
			assert.Error(t, err, "a section ze cannot read must not decode")
		})
	}
}

// TestParsedRouteReEncodesToItsOctets holds the wire round trip.
//
// VALIDATES: Bytes() and WriteTo reproduce the exact octets parseMVPN read, for
// every route type the section carries.
// PREVENTS: a re-encode that drops a field ze parsed, which would send a peer
// something other than what it advertised.
func TestParsedRouteReEncodesToItsOctets(t *testing.T) {
	t.Parallel()

	for _, want := range []string{sharedJoinHex, sourceJoinHex, "05120000FDE80001869F200A630C0420EFFBFFE4", "010C0000FDE9000000640A000001"} {
		t.Run(want[:2], func(t *testing.T) {
			t.Parallel()
			data, err := hex.DecodeString(want)
			require.NoError(t, err)

			parsed, rest, err := parseMVPN(AFIIPv4, data)
			require.NoError(t, err)
			assert.Empty(t, rest)
			assert.Equal(t, want, strings.ToUpper(hex.EncodeToString(parsed.Bytes())))

			buf := make([]byte, parsed.Len())
			assert.Equal(t, parsed.Len(), parsed.WriteTo(buf, 0))
			assert.Equal(t, want, strings.ToUpper(hex.EncodeToString(buf)))
		})
	}
}

// TestBothJSONPathsAgree holds the two decoders against each other.
//
// VALIDATES: AppendJSON (the in-process fast path) and mvpnToJSON (the registry
// hex path) publish the same members for the same octets.
// PREVENTS: a reader that cannot tell which decoder served it meeting two
// different shapes for one route.
func TestBothJSONPathsAgree(t *testing.T) {
	t.Parallel()

	for _, want := range []string{sharedJoinHex, sourceJoinHex, "05120000FDE80001869F200A630C0420EFFBFFE4", "010C0000FDE9000000640A000001"} {
		t.Run(want[:2], func(t *testing.T) {
			t.Parallel()
			data, err := hex.DecodeString(want)
			require.NoError(t, err)

			parsed, _, err := parseMVPN(AFIIPv4, data)
			require.NoError(t, err)

			var fromAppender map[string]any
			require.NoError(t, json.Unmarshal(parsed.AppendJSON(nil), &fromAppender))

			viaMap, err := json.Marshal(mvpnToJSON(parsed))
			require.NoError(t, err)
			var fromMap map[string]any
			require.NoError(t, json.Unmarshal(viaMap, &fromMap))

			assert.Equal(t, fromMap, fromAppender)
		})
	}
}

// TestIPv6SectionReadsSixteenOctetAddresses drives the IPv6 half of the fixture.
//
// VALIDATES: a Multicast Source Length of 128 reads sixteen octets, so an IPv6
// C-multicast route decodes to its own addresses.
// PREVENTS: a decoder that assumes the IPv4 lengths and reads the group out of
// the middle of the source.
func TestIPv6SectionReadsSixteenOctetAddresses(t *testing.T) {
	t.Parallel()

	const sharedJoinV6 = "062E0000FDE80001869F0000FDE880FD00000000000000000000000000000180FF0E0000000000000000000000000001"
	decoded, err := DecodeNLRIHex("ipv6/mvpn", sharedJoinV6, false)
	require.NoError(t, err)

	raw, err := json.Marshal(decoded)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "fd00::1", got["source"])
	assert.Equal(t, "ff0e::1", got["group"])
	assert.Equal(t, sharedJoinV6, got["raw"])
}
