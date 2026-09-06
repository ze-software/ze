// Design: update_text.go — the "update text" attribute grammar under test.
//
// Goal: prove that the five path attributes added to `send bgp <selector> update
// text` in 2026-09 reach the wire with the octets their RFCs specify, and that a
// value the grammar cannot represent is refused rather than dropped.
//
// Method: drive ParseUpdateText with the operator's own tokens, then assert the
// hexadecimal attribute section the parser hands the reactor. A decoded struct
// would pass against an encoder that writes the wrong flags or the wrong field
// width, which is the defect class these attributes have already produced
// (attribute.OriginatorID once wrote sixteen octets into a four-octet field).
//
// RFC: rfc/short/rfc4271.md — ATOMIC_AGGREGATE (Section 4.3 f), AGGREGATOR (4.3 g)
// RFC: rfc/short/rfc4456.md — ORIGINATOR_ID and CLUSTER_LIST (Sections 7, 8)
// RFC: rfc/short/rfc7607.md — AS 0 is not originated in the AGGREGATOR
// RFC: rfc/short/rfc7311.md — the AIGP metric TLV

package update

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testAttributeHex parses one update text command and returns the attribute
// section it produced, as lowercase hexadecimal.
func testAttributeHex(t *testing.T, args ...string) string {
	t.Helper()
	result, err := ParseUpdateText(args)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	wire := result.Groups[0].Wire
	require.NotNil(t, wire, "the command declared attributes, so the group must carry a wire section")
	return hex.EncodeToString(wire.Packed())
}

// testAnnounceIPv4 is the shortest NLRI section that makes a command complete.
var testAnnounceIPv4 = []string{"nlri", "ipv4/unicast", "add", "10.0.0.0/24"}

func testCommand(attrs ...string) []string {
	return append(attrs, testAnnounceIPv4...)
}

// TestParseUpdateText_AtomicAggregate checks the value-less attribute.
//
// VALIDATES: RFC 4271 Section 4.3 f): "ATOMIC_AGGREGATE is a well-known
// discretionary attribute of length 0." Flags 0x40 (well-known transitive), code
// 6, length 0, no value octets.
// PREVENTS: the keyword being read as a valued attribute, which would swallow the
// token after it.
func TestParseUpdateText_AtomicAggregate(t *testing.T) {
	got := testAttributeHex(t, testCommand("atomic-aggregate")...)
	assert.Equal(t, "400600", got)
}

// TestParseUpdateText_AtomicAggregateDoesNotEatNextToken proves the flag consumes
// exactly one token, so an attribute written after it still parses.
//
// VALIDATES: atomic-aggregate takes no value.
// PREVENTS: a regression that routes it through parseCommonAttributeText, whose
// contract reads a zero consumed-count as "keyword not handled".
func TestParseUpdateText_AtomicAggregateDoesNotEatNextToken(t *testing.T) {
	got := testAttributeHex(t, testCommand("atomic-aggregate", "med", "7")...)
	// MED (4) sorts below ATOMIC_AGGREGATE (6): RFC 4271 Section 5 emits
	// attributes in ascending type-code order.
	assert.Equal(t, "80040400000007"+"400600", got)
}

// TestParseUpdateText_Aggregator checks the "<asn>:<ip>" form.
//
// VALIDATES: RFC 4271 Section 4.3 g) AGGREGATOR carries "the last AS number that
// formed the aggregate route ... followed by the IP address of the BGP speaker
// that formed the aggregate route (encoded as 4 octets)", written in the RFC 6793
// four-octet AS form. Flags 0xC0, code 7, length 8.
// PREVENTS: the two-octet AS form, or a swapped ASN and address.
func TestParseUpdateText_Aggregator(t *testing.T) {
	got := testAttributeHex(t, testCommand("aggregator", "65000:10.0.0.1")...)
	assert.Equal(t, "c00708"+"0000fde8"+"0a000001", got)
}

// TestParseUpdateText_AggregatorFourOctetASN checks an ASN above 65535.
//
// VALIDATES: RFC 6793 Section 3 four-octet AS numbers survive the text form.
// PREVENTS: a 16-bit parse silently truncating the ASN.
func TestParseUpdateText_AggregatorFourOctetASN(t *testing.T) {
	got := testAttributeHex(t, testCommand("aggregator", "4200000000:192.0.2.1")...)
	assert.Equal(t, "c00708"+"fa56ea00"+"c0000201", got)
}

// TestParseUpdateText_AggregatorRefusals checks every value the field cannot hold.
//
// VALIDATES: RFC 7607 Section 2: "A BGP speaker MUST NOT originate or propagate a
// route with an AS number of zero in the AS_PATH, AS4_PATH, AGGREGATOR, or
// AS4_AGGREGATOR attributes." And RFC 4271 Section 4.3 g): the address is four
// octets, so an IPv6 value has nowhere to go.
// PREVENTS: a zero ASN or a truncated address reaching the encoder, where a
// silently wrong value is indistinguishable from an unset one.
func TestParseUpdateText_AggregatorRefusals(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"no separator", "65000", "expected <asn>:<ip>"},
		{"as zero", "0:10.0.0.1", "RFC 7607"},
		{"asn not a number", "sixty-five-thousand:10.0.0.1", "invalid aggregator ASN"},
		{"asn above four octets", "4294967296:10.0.0.1", "invalid aggregator ASN"},
		{"address is ipv6", "65000:2001:db8::1", "is not an IPv4 address"},
		{"address not an address", "65000:router1", "is not an address"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseUpdateText(testCommand("aggregator", tc.value))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestParseUpdateText_AggregatorMissingValue checks the end-of-args case.
//
// VALIDATES: a keyword with no value is an error, not a silent no-op.
// PREVENTS: "aggregator" alone building an UPDATE with no AGGREGATOR.
func TestParseUpdateText_AggregatorMissingValue(t *testing.T) {
	_, err := ParseUpdateText([]string{"aggregator"})
	require.ErrorIs(t, err, errMissingAggregatorValue)
}

// TestParseUpdateText_OriginatorID checks the reflector's originator identifier.
//
// VALIDATES: RFC 4456 Section 8: ORIGINATOR_ID "is a new optional, non-transitive
// BGP attribute of Type code 9.  This attribute is 4 bytes long". Flags 0x80,
// code 9, length 4.
// PREVENTS: the transitive bit being set, which would make a route reflector's
// loop-prevention identifier propagate outside the AS.
func TestParseUpdateText_OriginatorID(t *testing.T) {
	got := testAttributeHex(t, testCommand("originator-id", "10.0.99.12")...)
	assert.Equal(t, "800904"+"0a00630c", got)
}

// TestParseUpdateText_OriginatorIDRefusals checks values wider than four octets.
//
// VALIDATES: RFC 4456 Section 8 fixes the attribute at 4 bytes.
// PREVENTS: an IPv6 address being zero-filled into the four-octet field, which
// would advertise originator 0.0.0.0 and defeat the loop check the attribute
// exists for.
func TestParseUpdateText_OriginatorIDRefusals(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"ipv6", "2001:db8::1", "is not an IPv4 address"},
		{"not an address", "reflector-1", "is not an address"},
		{"bare integer", "3", "is not an address"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseUpdateText(testCommand("originator-id", tc.value))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid originator-id")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestParseUpdateText_OriginatorIDMissingValue checks the end-of-args case.
//
// VALIDATES: a keyword with no value is an error.
// PREVENTS: a silently absent ORIGINATOR_ID.
func TestParseUpdateText_OriginatorIDMissingValue(t *testing.T) {
	_, err := ParseUpdateText([]string{"originator-id"})
	require.ErrorIs(t, err, errMissingOriginatorIDValue)
}

// TestParseUpdateText_ClusterList checks the bracketed reflection path.
//
// VALIDATES: RFC 4456 Section 8: CLUSTER_LIST "is a sequence of CLUSTER_ID values
// representing the reflection path that the route has passed", each four octets
// (Section 7). Flags 0x80, code 10, length 8 for two ids, in the order written.
// PREVENTS: a reordered or deduplicated list, either of which would rewrite the
// reflection path the receiver checks itself against.
func TestParseUpdateText_ClusterList(t *testing.T) {
	got := testAttributeHex(t, testCommand("cluster-list", "[", "3.3.3.3", "192.168.201.1", "]")...)
	assert.Equal(t, "800a08"+"03030303"+"c0a8c901", got)
}

// TestParseUpdateText_ClusterListSingleValue checks the unbracketed single id,
// which parseBracketedListText accepts for every list attribute.
//
// VALIDATES: one id produces a four-octet CLUSTER_LIST.
// PREVENTS: the bracket-free form being read as an empty list.
func TestParseUpdateText_ClusterListSingleValue(t *testing.T) {
	got := testAttributeHex(t, testCommand("cluster-list", "3.3.3.3")...)
	assert.Equal(t, "800a04"+"03030303", got)
}

// TestParseUpdateText_ClusterListRefusals checks ids the field cannot hold.
//
// VALIDATES: a CLUSTER_ID is four octets (RFC 4456 Section 7), and an empty
// sequence names no reflector.
// PREVENTS: a bad id being skipped, which would shorten the reflection path and
// hide a loop from the receiver.
func TestParseUpdateText_ClusterListRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"ipv6 id", []string{"cluster-list", "[", "2001:db8::1", "]"}, "is not an IPv4 address"},
		{"second id invalid", []string{"cluster-list", "[", "3.3.3.3", "cluster2", "]"}, "is not an address"},
		{"empty list", []string{"cluster-list", "[", "]"}, "at least one cluster id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseUpdateText(testCommand(tc.args...))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestParseUpdateText_ClusterListMissingValue checks the end-of-args case.
//
// VALIDATES: a keyword with no value is an error.
// PREVENTS: a silently absent CLUSTER_LIST.
func TestParseUpdateText_ClusterListMissingValue(t *testing.T) {
	_, err := ParseUpdateText([]string{"cluster-list"})
	require.ErrorIs(t, err, errMissingClusterListValue)
}

// TestParseUpdateText_AIGP checks the accumulated IGP metric.
//
// VALIDATES: RFC 7311 Section 3 gives the AIGP TLV "Type: 1", "Length: 11",
// "Value: Accumulated IGP Metric" over eight octets, and states "The AIGP
// attribute is an optional, non-transitive BGP path attribute", so the flags are
// 0x80 and NOT 0xC0.
// PREVENTS: the transitive bit returning. RFC 7311 Section 3.2: "If a BGP path
// attribute is received that has the AIGP attribute codepoint but also has the
// transitive bit set, the attribute MUST be considered to be a malformed AIGP
// attribute and MUST be discarded", so 0xC0 makes every conformant peer drop it.
func TestParseUpdateText_AIGP(t *testing.T) {
	got := testAttributeHex(t, testCommand("aigp", "100")...)
	assert.Equal(t, "801a0b"+"01000b"+"0000000000000064", got)
}

// TestParseUpdateText_AIGPMaxMetric checks the full 64-bit range.
//
// VALIDATES: the metric is an unsigned 64-bit value.
// PREVENTS: a 32-bit parse truncating a large accumulated metric.
func TestParseUpdateText_AIGPMaxMetric(t *testing.T) {
	got := testAttributeHex(t, testCommand("aigp", "18446744073709551615")...)
	assert.Equal(t, "801a0b"+"01000b"+"ffffffffffffffff", got)
}

// TestParseUpdateText_AIGPRefusals checks values outside the metric range.
//
// VALIDATES: a metric the TLV cannot carry is refused.
// PREVENTS: a wrapped or zeroed metric, which would silently reorder best-path
// selection on the receiver.
func TestParseUpdateText_AIGPRefusals(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"not a number", "high"},
		{"negative", "-1"},
		{"above 64 bits", "18446744073709551616"},
		{"hexadecimal", "0x64"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseUpdateText(testCommand("aigp", tc.value))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid aigp")
		})
	}
}

// TestParseUpdateText_AIGPMissingValue checks the end-of-args case.
//
// VALIDATES: a keyword with no value is an error.
// PREVENTS: a silently absent AIGP.
func TestParseUpdateText_AIGPMissingValue(t *testing.T) {
	_, err := ParseUpdateText([]string{"aigp"})
	require.ErrorIs(t, err, errMissingAIGPValue)
}

// TestParseUpdateText_ReflectionAttributesTogether is the whole command the
// grammar extension was written for.
//
// VALIDATES: all five attributes in one command, emitted in ascending type-code
// order (RFC 4271 Section 5), whatever order the operator wrote them in:
// ATOMIC_AGGREGATE 6, AGGREGATOR 7, ORIGINATOR_ID 9, CLUSTER_LIST 10, AIGP 26.
// PREVENTS: an emission order taken from the command line, which a receiver that
// checks attribute ordering rejects.
func TestParseUpdateText_ReflectionAttributesTogether(t *testing.T) {
	got := testAttributeHex(t,
		"originator-id", "10.0.99.12",
		"cluster-list", "[", "3.3.3.3", "192.168.201.1", "]",
		"atomic-aggregate",
		"aggregator", "65000:10.0.0.1",
		"aigp", "100",
		"nlri", "ipv4/unicast", "add", "10.0.0.0/24",
	)

	want := "400600" + // ATOMIC_AGGREGATE: flags 40, code 06, length 00
		"c00708" + "0000fde8" + "0a000001" + // AGGREGATOR: AS 65000, 10.0.0.1
		"800904" + "0a00630c" + // ORIGINATOR_ID: 10.0.99.12
		"800a08" + "03030303" + "c0a8c901" + // CLUSTER_LIST: 3.3.3.3, 192.168.201.1
		"801a0b" + "01000b" + "0000000000000064" // AIGP: metric 100
	assert.Equal(t, want, got)
}

// TestParseUpdateText_ReflectionAttributesAfterNLRIRefused checks the flat
// grammar's one ordering rule still holds for the new keywords.
//
// VALIDATES: attributes must precede every nlri section.
// PREVENTS: an attribute written after an nlri section being applied to the wrong
// group, or to none.
func TestParseUpdateText_ReflectionAttributesAfterNLRIRefused(t *testing.T) {
	for _, attr := range [][]string{
		{"atomic-aggregate"},
		{"aggregator", "65000:10.0.0.1"},
		{"originator-id", "10.0.99.12"},
		{"cluster-list", "3.3.3.3"},
		{"aigp", "100"},
	} {
		t.Run(attr[0], func(t *testing.T) {
			_, err := ParseUpdateText(append(append([]string{}, testAnnounceIPv4...), attr...))
			require.ErrorIs(t, err, errAttrsAfterNLRI)
		})
	}
}
