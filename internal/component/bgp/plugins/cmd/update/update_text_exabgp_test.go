// Design: docs/architecture/api/update-syntax.md -- the three spellings ExaBGP writes
// Overview: update_text.go -- the parser under test
//
// Every expected byte run in this file is copied from the `raw:` line of the
// ExaBGP compatibility fixture named above it. They are what ExaBGP puts on the
// wire for the same command, so they are the contract; none of them was derived
// from Ze's own output.

package update

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/mvpn" // blank import: registers the MVPN NLRI encoder
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// TestExtendedCommunityRawHexReachesTheWire verifies the raw 8-octet
// extended-community spelling an operator uses for a community Ze has no
// keyword for.
//
// VALIDATES: `extended-community 0x0002FDE900000001` encodes the 8 octets
// verbatim. The expectation is the EXTENDED_COMMUNITIES run of
// test/exabgp-compat/api/api-vpnv4.ci line 5, `C010080002FDE900000001`, whose
// value octets are `0002FDE900000001`. The profile that drives that fixture
// writes the raw form (internal/le/interoplab/bgp/exabgp_helpers.go, api-vpnv4).
// PREVENTS: `update text` refusing a community the config path accepts, which
// is what it did: "expected <type>:<value>, or one of [...]".
func TestExtendedCommunityRawHexReachesTheWire(t *testing.T) {
	result, err := ParseUpdateText([]string{
		"extended-community", "0x0002FDE900000001",
		"nlri", "ipv4/unicast", "add", "1.4.0.0/16",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	ecs := testExtractExtCommunities(t, result.Groups[0].Wire)
	require.Len(t, ecs, 1)
	assert.Equal(t, "0002fde900000001", hex.EncodeToString(ecs[0][:]))
}

// TestExtendedCommunityRawHexRefusesAWrongWidth verifies the raw form is held to
// its width.
//
// VALIDATES: RFC 4360 Section 2 -- "Each Extended Community is encoded as an
// 8-octet quantity" -- so 14 and 18 hex digits are each refused BY NAME rather
// than padded or truncated onto the wire.
// PREVENTS: a mistyped community reaching a peer as a different community.
func TestExtendedCommunityRawHexRefusesAWrongWidth(t *testing.T) {
	for _, value := range []string{"0x0002FDE9000000", "0x0002FDE90000000123", "0x"} {
		_, err := ParseUpdateText([]string{
			"extended-community", value,
			"nlri", "ipv4/unicast", "add", "1.4.0.0/16",
		})
		require.Error(t, err, "value %s", value)
		assert.Contains(t, err.Error(), value)
		assert.Contains(t, err.Error(), "16 hex digits")
	}
}

// TestMVPNSectionMatchesTheExaBGPWire verifies every MCAST-VPN route type Ze
// advertises is reachable from `update text`, in both address families.
//
// VALIDATES: the NLRI bytes of test/exabgp-compat/api/api-mvpn.ci. Each want
// value is the `raw` field of that file's `1:json:` line for the same route,
// which is the MCAST-VPN NLRI ExaBGP encodes for the same command.
// PREVENTS: "family not supported in text mode: ipv4/mvpn", which is what the
// parser answered while a registered plugin stood beside it with no encoder.
func TestMVPNSectionMatchesTheExaBGPWire(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "ipv4 shared-join",
			args: strings.Fields("nlri ipv4/mvpn add shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000"),
			want: "06160000FDE80001869F0000FDE8200A63C70120EFFBFFE4",
		},
		{
			name: "ipv4 source-join",
			args: strings.Fields("nlri ipv4/mvpn add source-join source 10.99.12.2 group 239.251.255.228 rd 65000:99999 source-as 65000"),
			want: "07160000FDE80001869F0000FDE8200A630C0220EFFBFFE4",
		},
		{
			name: "ipv4 source-ad",
			args: strings.Fields("nlri ipv4/mvpn add source-ad source 10.99.12.4 group 239.251.255.228 rd 65000:99999"),
			want: "05120000FDE80001869F200A630C0420EFFBFFE4",
		},
		{
			name: "ipv6 shared-join",
			args: strings.Fields("nlri ipv6/mvpn add shared-join rp fd00::1 group ff0e::1 rd 65000:99999 source-as 65000"),
			want: "062E0000FDE80001869F0000FDE880FD00000000000000000000000000000180FF0E0000000000000000000000000001",
		},
		{
			name: "ipv6 source-join",
			args: strings.Fields("nlri ipv6/mvpn add source-join source fd12::2 group ff0e::1 rd 65000:99999 source-as 65000"),
			want: "072E0000FDE80001869F0000FDE880FD12000000000000000000000000000280FF0E0000000000000000000000000001",
		},
		{
			name: "ipv6 source-ad",
			args: strings.Fields("nlri ipv6/mvpn add source-ad source fd12::4 group ff0e::1 rd 65000:99999"),
			want: "052A0000FDE80001869F80FD12000000000000000000000000000480FF0E0000000000000000000000000001",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseUpdateText(tc.args)
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 1)

			assert.Equal(t, strings.ToLower(tc.want), testNLRIHex(t, result.Groups[0].Announce[0]))
		})
	}
}

// TestMVPNSectionWithdrawsTheSameBytes verifies `del` reaches MP_UNREACH with
// the NLRI `add` would have announced.
//
// VALIDATES: the withdraw half of test/exabgp-compat/api/api-mvpn.ci line 28,
// whose MP_UNREACH carries the same `06160000FDE8...` NLRI as the announce.
// PREVENTS: a withdrawal that names a different route from the announcement.
func TestMVPNSectionWithdrawsTheSameBytes(t *testing.T) {
	result, err := ParseUpdateText(strings.Fields(
		"nlri ipv4/mvpn del shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000"))
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Empty(t, result.Groups[0].Announce)
	require.Len(t, result.Groups[0].Withdraw, 1)

	assert.Equal(t, "06160000fde80001869f0000fde8200a63c70120effbffe4", testNLRIHex(t, result.Groups[0].Withdraw[0]))
}

// TestMVPNSectionRefusesAnIncompleteRoute verifies each missing or mismatched
// field is named rather than defaulted.
//
// VALIDATES: RFC 6514 Section 4.6 gives route types 6 and 7 a "Source AS (4
// octets)" field, and Section 4 makes the AFI decide whether the source and
// group are IPv4 or IPv6. A missing source-as, a missing rd and an address of
// the wrong family are each refused BY NAME.
// PREVENTS: AS 0 or the wrong address family reaching a peer as if the operator
// had asked for it (ai/rules/principles.md).
func TestMVPNSectionRefusesAnIncompleteRoute(t *testing.T) {
	cases := map[string]struct {
		args []string
		want string
	}{
		"missing source-as": {
			args: strings.Fields("nlri ipv4/mvpn add shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999"),
			want: "source-as",
		},
		"missing rd": {
			args: strings.Fields("nlri ipv4/mvpn add source-ad source 10.99.12.4 group 239.251.255.228"),
			want: "rd",
		},
		"ipv6 address under the ipv4 afi": {
			args: strings.Fields("nlri ipv4/mvpn add source-ad source fd12::4 group 239.251.255.228 rd 65000:99999"),
			want: "IPv4",
		},
		"ipv4 address under the ipv6 afi": {
			args: strings.Fields("nlri ipv6/mvpn add source-ad source 10.99.12.4 group ff0e::1 rd 65000:99999"),
			want: "IPv6",
		},
		"unknown route type": {
			args: strings.Fields("nlri ipv4/mvpn add leaf-ad source 10.99.12.4 group 239.251.255.228 rd 65000:99999"),
			want: "leaf-ad",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseUpdateText(tc.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestPrefixSIDSRv6MatchesTheExaBGPAttribute verifies the BGP Prefix-SID
// attribute ExaBGP writes as `bgp-prefix-sid-srv6 ( l3-service ... )`.
//
// VALIDATES: the `C028 25 ...` attribute run of
// test/exabgp-compat/api/api-ipv4.ci line 10 and api-ipv6.ci line 10. The two
// differ only in the SRv6 Endpoint Behavior, 0x48 against 0x47.
// PREVENTS: `update text` dropping the attribute, which it did: no keyword
// reached the encoder, so the SRv6 L3 Service TLV never left the parser.
func TestPrefixSIDSRv6MatchesTheExaBGPAttribute(t *testing.T) {
	cases := []struct {
		name     string
		behavior string
		want     string // the whole attribute, flags and code included
	}{
		{
			name:     "api-ipv4.ci behavior 0x48",
			behavior: "0x48",
			want:     "C028250500220001001E0020010DB800010001000000000000000000004800010006401810000000",
		},
		{
			name:     "api-ipv6.ci behavior 0x47",
			behavior: "0x47",
			want:     "C028250500220001001E0020010DB800010001000000000000000000004700010006401810000000",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseUpdateText([]string{
				"bgp-prefix-sid-srv6", "(", "l3-service", "2001:db8:1:1::", tc.behavior, "[64,24,16,0,0,0]", ")",
				"nlri", "ipv4/unicast", "add", "10.0.1.0/24",
			})
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)

			wire := result.Groups[0].Wire
			require.NotNil(t, wire)
			assert.Contains(t, strings.ToUpper(hex.EncodeToString(wire.Packed())), tc.want)
		})
	}
}

// TestPrefixSIDSRv6RefusesAMalformedValue verifies each part of the value is
// checked rather than skipped.
//
// VALIDATES: a service type that is neither l3-service nor l2-service, a SID
// that is not an IPv6 address, and a SID structure that is not the six values
// RFC 9252 Section 3.2.1 lists are each refused BY NAME.
// PREVENTS: a mistyped Prefix-SID being encoded as a shorter TLV a peer reads
// as a different SID.
func TestPrefixSIDSRv6RefusesAMalformedValue(t *testing.T) {
	cases := map[string][]string{
		"unknown service": {"bgp-prefix-sid-srv6", "(", "l4-service", "2001:db8:1:1::", ")"},
		"ipv4 sid":        {"bgp-prefix-sid-srv6", "(", "l3-service", "10.0.0.1", ")"},
		"short structure": {"bgp-prefix-sid-srv6", "(", "l3-service", "2001:db8:1:1::", "0x48", "[64,24,16]", ")"},
		"unclosed":        {"bgp-prefix-sid-srv6", "(", "l3-service", "2001:db8:1:1::"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseUpdateText(append(args, "nlri", "ipv4/unicast", "add", "10.0.1.0/24"))
			require.Error(t, err)
		})
	}
}

// TestPrefixSIDSRv6MatchesTheConfigPath verifies the two entry points produce
// one answer.
//
// VALIDATES: `update text` and the config file reach the SAME encoder, so the
// attribute value bytes are identical for the same operator text.
// PREVENTS: the two spellings drifting, which is the defect this keyword was
// added to close (ai/rules/principles.md).
func TestPrefixSIDSRv6MatchesTheConfigPath(t *testing.T) {
	fromConfig, err := attribute.ParsePrefixSIDSRv6("l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0]")
	require.NoError(t, err)

	result, err := ParseUpdateText([]string{
		"bgp-prefix-sid-srv6", "(", "l3-service", "2001:db8:1:1::", "0x48", "[64,24,16,0,0,0]", ")",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24",
	})
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)

	assert.Contains(t,
		hex.EncodeToString(result.Groups[0].Wire.Packed()),
		hex.EncodeToString(fromConfig))
}

// TestPathInformationAtTheTopLevelReachesTheNLRI verifies the ADD-PATH path
// identifier is accepted where `rd` and `label` are accepted.
//
// VALIDATES: the NLRI bytes of test/exabgp-compat/api/api-attributes-path.ci
// line 7, `01020304 20 10111213` and `01020304 20 14151617`: a four-octet Path
// Identifier prepended to each <length, prefix>. The fixture writes the
// identifier in its dotted form, 1.2.3.4, which is 16909060.
// PREVENTS: "unexpected token 'path-information'", which is what the parser
// answered while accepting the same keyword inside an nlri section.
func TestPathInformationAtTheTopLevelReachesTheNLRI(t *testing.T) {
	cases := []struct {
		name  string
		id    string
		want0 string
		want1 string
	}{
		{
			name:  "api-attributes-path.ci 1.2.3.4",
			id:    "16909060",
			want0: "010203042010111213",
			want1: "010203042014151617",
		},
		{
			name:  "api-attributes-path.ci 4.3.2.1",
			id:    "67305985",
			want0: "040302012010111213",
			want1: "040302012014151617",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseUpdateText([]string{
				"path-information", tc.id, "next-hop", "10.11.12.13",
				"origin", "igp", "local-preference", "16", "community", "[14:15]",
				"nlri", "ipv4/unicast", "add", "16.17.18.19/32", "20.21.22.23/32",
			})
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 2)

			// The peer negotiated ADD-PATH, so the four-octet Path Identifier is
			// written: RFC 7911 Section 3 prepends it to the <length, prefix>.
			assert.Equal(t, tc.want0, testAddPathNLRIHex(t, result.Groups[0].Announce[0]))
			assert.Equal(t, tc.want1, testAddPathNLRIHex(t, result.Groups[0].Announce[1]))
		})
	}
}

// TestPathInformationRefusesAMalformedValue verifies the identifier is checked
// rather than defaulted.
//
// VALIDATES: RFC 7911 Section 3 gives the Path Identifier four octets, so a
// non-numeric token and a value above 4294967295 are each refused BY NAME.
// PREVENTS: a mistyped identifier silently becoming a different path.
func TestPathInformationRefusesAMalformedValue(t *testing.T) {
	for _, value := range []string{"1.2.3.4", "4294967296", "-1"} {
		_, err := ParseUpdateText([]string{
			"path-information", value,
			"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
		})
		require.Error(t, err, "value %s", value)
		assert.Contains(t, err.Error(), value)
	}
}

// TestLabelTakesAStack verifies `label` accepts the list an MPLS label stack is.
//
// VALIDATES: the MP_REACH NLRI of test/exabgp-compat/api/api-attributes-vpn.ci
// line 9, `6A 0006E1 0000F76500000064 800040`: 106 bits, one label 110 with the
// Bottom of Stack bit set (0x0006E1 = 110<<4 | 1), the RD 63333:100, then
// 128.0.64.0/18. The profile that drives that fixture writes `label [ 110 ]`
// (internal/le/interoplab/bgp/exabgp_helpers.go, api-attributes-vpn).
//
// The two-label expectation is NOT from a fixture: no shipped fixture sends a
// stack. It is the same RFC 8277 Section 2 rule applied twice -- 110 without the
// Bottom of Stack bit, then 200 with it -- so the bit placement stays asserted
// once a second label exists.
// PREVENTS: `invalid label: strconv.ParseUint: parsing "[": invalid syntax`,
// which is what the bracketed form answered while the accumulator behind it was
// already a slice.
func TestLabelTakesAStack(t *testing.T) {
	cases := []struct {
		name  string
		label []string
		want  string
	}{
		{
			name:  "bare label, as api-attributes-vpn.ci encodes it",
			label: []string{"label", "110"},
			want:  "6a0006e10000f76500000064800040",
		},
		{
			name:  "bracketed one-label list, as the ExaBGP profile writes it",
			label: []string{"label", "[", "110", "]"},
			want:  "6a0006e10000f76500000064800040",
		},
		{
			name:  "two-label stack: 110 then 200, bottom of stack on the last",
			label: []string{"label", "[", "110", "200", "]"},
			want:  "820006e0000c810000f76500000064800040",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"rd", "63333:100"}
			args = append(args, tc.label...)
			args = append(args, "next-hop", "10.0.99.12",
				"nlri", "ipv4/mpls-vpn", "add", "128.0.64.0/18")

			result, err := ParseUpdateText(args)
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			require.Len(t, result.Groups[0].Announce, 1)
			assert.Equal(t, tc.want, testNLRIHex(t, result.Groups[0].Announce[0]))
		})
	}
}

// TestLabelStackRefusesAMalformedValue verifies each value in the list is
// checked.
//
// VALIDATES: RFC 3032 Section 2.1 gives the Label field 20 bits, so 1048576 is
// refused, and so is a non-numeric member of an otherwise valid list.
// PREVENTS: a truncated label reaching the wire as a different label.
func TestLabelStackRefusesAMalformedValue(t *testing.T) {
	cases := map[string][]string{
		"over 20 bits":           {"label", "1048576"},
		"over 20 bits in a list": {"label", "[", "110", "1048576", "]"},
		"non-numeric in a list":  {"label", "[", "110", "green", "]"},
		"empty list":             {"label", "[", "]"},
	}
	for name, label := range cases {
		t.Run(name, func(t *testing.T) {
			args := append([]string{"rd", "63333:100"}, label...)
			args = append(args, "next-hop", "10.0.99.12",
				"nlri", "ipv4/mpls-vpn", "add", "128.0.64.0/18")
			_, err := ParseUpdateText(args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "label")
		})
	}
}

// testNLRIHex writes one NLRI to a scratch buffer and returns its lowercase hex.
func testNLRIHex(t *testing.T, n nlri.NLRI) string {
	t.Helper()
	buf := make([]byte, 256)
	return hex.EncodeToString(buf[:n.WriteTo(buf, 0)])
}

// testAddPathNLRIHex is testNLRIHex with the ADD-PATH path identifier written.
// NLRI.WriteTo omits it by contract (core/bgp/nlri/inet.go), and nlri.WriteNLRI
// is the encoder the reactor calls once a peer has negotiated RFC 7911.
func testAddPathNLRIHex(t *testing.T, n nlri.NLRI) string {
	t.Helper()
	buf := make([]byte, 256)
	return hex.EncodeToString(buf[:nlri.WriteNLRI(n, buf, 0, true)])
}

// TestPrefixSIDSRv6RefusesAnEmptyValue verifies an empty parenthesized value is
// refused rather than dropping the attribute.
//
// VALIDATES: `bgp-prefix-sid-srv6 ( )` is an error, not a Prefix-SID-free
// UPDATE that the operator believes carries one.
// PREVENTS: a nil TLV a caller cannot tell from the attribute it asked for
// (ai/rules/principles.md).
func TestPrefixSIDSRv6RefusesAnEmptyValue(t *testing.T) {
	_, err := ParseUpdateText([]string{
		"bgp-prefix-sid-srv6", "(", ")",
		"nlri", "ipv4/unicast", "add", "10.0.1.0/24",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "srv6 prefix-sid requires")
}
