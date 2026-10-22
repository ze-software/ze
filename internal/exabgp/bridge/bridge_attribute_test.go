// Design: docs/architecture/exabgp-bridge.md -- what the bridge refuses, and what it keeps
// Related: bridge_attribute.go -- parseRouteAttributes, keep-first and the value refusal
// Related: bridge_route_forms.go -- splitPrefix and the mask refusal
// Related: bridge_operational.go -- convertOperational
//
// ExaBGP 6 keeps 5.0's leniency on the API for a repeated attribute and a short
// split (keep the first, send the prefix whole, log it), and refuses a
// value-less attribute, a mask that is no number and `announce operational`.
// These tests drive one translated line each and read what reaches ze, what is
// refused, and what is logged.
package bridge

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureWarnings routes the default slog logger into a buffer for the test,
// and restores the previous logger after it. The bridge logs a route line's
// warnings through the default logger (warnRouteLine), so a test that reads
// them MUST NOT run in parallel with another that does.
func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &logged
}

// commandTexts is the ze command text of a translation, in order.
func commandTexts(translation Translation) []string {
	texts := make([]string, 0, len(translation.Commands))
	for _, command := range translation.Commands {
		texts = append(texts, command.Text)
	}
	return texts
}

// TestRepeatedAttributeKeepsFirstAndWarns is X-26 and X-27 (AC-32).
//
// VALIDATES: `next-hop 1.1.1.1 next-hop 2.2.2.2` sends next-hop 1.1.1.1 only,
// and one warning names the dropped 2.2.2.2; another single-value attribute
// repeated (med) is kept first the same way, in both the route and the
// attributes forms.
// PREVENTS: forwarding both values and letting ze's parser pick the last.
func TestRepeatedAttributeKeepsFirstAndWarns(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		want    string
		dropped string
	}{
		{
			name:    "next-hop twice keeps the first",
			line:    "announce route 10.0.0.0/24 next-hop 1.1.1.1 next-hop 2.2.2.2",
			want:    "send bgp * update text nhop 1.1.1.1 nlri add 10.0.0.0/24",
			dropped: "next-hop 2.2.2.2 is dropped",
		},
		{
			name:    "med twice keeps the first",
			line:    "announce route 10.0.0.0/24 next-hop 1.1.1.1 med 10 med 20",
			want:    "send bgp * update text nhop 1.1.1.1 med 10 nlri add 10.0.0.0/24",
			dropped: "med 20 is dropped",
		},
		{
			name:    "the attributes form keeps the first",
			line:    "announce attributes next-hop 1.1.1.1 next-hop 2.2.2.2 nlri 10.0.0.0/24",
			want:    "send bgp * update text nhop 1.1.1.1 nlri add 10.0.0.0/24",
			dropped: "next-hop 2.2.2.2 is dropped",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logged := captureWarnings(t)
			translation, err := TranslateLine(tc.line)
			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, commandTexts(translation))
			assert.Equal(t, 1, strings.Count(logged.String(), "exabgp-bridge route line"), logged.String())
			assert.Contains(t, logged.String(), tc.dropped)
		})
	}
}

// TestSplitShorterThanPrefixSendsPrefix is X-27 (AC-33).
//
// VALIDATES: `split /23` on a /24 sends the /24 whole and logs one warning
// naming the split; a split longer than an address is still refused.
// PREVENTS: refusing a line ExaBGP's API sends, and a silent whole send.
func TestSplitShorterThanPrefixSendsPrefix(t *testing.T) {
	logged := captureWarnings(t)
	translation, err := TranslateLine("announce route 10.0.0.0/24 next-hop 1.1.1.1 split /23")
	require.NoError(t, err)
	assert.Equal(t, []string{"send bgp * update text nhop 1.1.1.1 nlri add 10.0.0.0/24"}, commandTexts(translation))
	assert.Equal(t, 1, strings.Count(logged.String(), "exabgp-bridge route line"), logged.String())
	assert.Contains(t, logged.String(), "split /23 is shorter than the prefix 10.0.0.0/24")

	_, err = TranslateLine("announce route 10.0.0.0/24 next-hop 1.1.1.1 split /33")
	require.ErrorIs(t, err, errSplitLength)
}

// TestPathInformationWithoutValueRefused is X-28 (AC-34), and settles A-6.
//
// VALIDATES: `path-information` with no value is refused, and the reason names
// the missing path identifier and what to write.
// PREVENTS: 5.0's silent ignore, and a refusal that names no identifier.
func TestPathInformationWithoutValueRefused(t *testing.T) {
	translation, err := TranslateLine("announce route 10.0.0.0/24 next-hop 1.1.1.1 path-information")
	require.ErrorIs(t, err, errMissingAttrValue)
	require.ErrorIs(t, err, ErrLineNotTranslated)
	assert.Empty(t, translation.Commands)
	assert.Contains(t, err.Error(), "path-information is written with no value")
	assert.Contains(t, err.Error(), "write path-information <path identifier, a number or an IPv4 address>")
}

// TestExtCommunityInterfaceSetRefused proves an interface-set written inside
// extended-community is refused on every route command form, as ExaBGP refuses it.
//
// VALIDATES: the route, attributes, family flow and sr-policy announce and
// withdraw forms each refuse the item, and the reason names the interface-set
// keyword to write it with.
// PREVENTS: Ze announcing an interface-set ExaBGP's extended_community
// (configuration/grammar/types/bgp.py) has no type for and refuses.
func TestExtCommunityInterfaceSetRefused(t *testing.T) {
	for _, line := range []string{
		"announce route 10.0.0.0/24 next-hop 1.1.1.1 extended-community [ interface-set:transitive:input:1.234:10 ]",
		"announce route 10.0.0.0/24 next-hop 1.1.1.1 extended-community interface-set:input:1234:10",
		"announce attributes next-hop 1.1.1.1 extended-community [ target:65000:1 interface-set:input:1234:10 ] nlri 10.0.0.0/24",
		"announce ipv4 flow destination 1.2.3.4/32 extended-community [ interface-set:input:1234:10 ] discard",
		// ExaBGP's sr_policy.py `_values` parses extended-community through
		// ExtendedCommunitiesType on announce and withdraw alike.
		"announce ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 1.2.3.4 extended-community [ interface-set:input:1:1 ]",
		"withdraw ipv4 sr-policy distinguisher 0 color 100 endpoint 10.0.0.1 extended-community [ interface-set:input:1:1 ]",
	} {
		t.Run(line, func(t *testing.T) {
			translation, err := TranslateLine(line)
			require.Error(t, err)
			assert.Empty(t, translation.Commands)
			assert.Contains(t, err.Error(), "ExaBGP refuses an interface-set inside extended-community")
			assert.Contains(t, err.Error(), "interface-set [ ")
		})
	}
}

// TestPrefixMaskNotNumberRefused is X-29 (AC-35).
//
// VALIDATES: `10.0.0.0/x` is refused in both the route and the attributes
// forms, and the reason names the mask and the prefix length to write.
// PREVENTS: taking it as the host route 10.0.0.0/32, as 5.0 did.
func TestPrefixMaskNotNumberRefused(t *testing.T) {
	for _, line := range []string{
		"announce route 10.0.0.0/x next-hop 1.1.1.1",
		"announce attributes next-hop 1.1.1.1 nlri 10.0.0.0/x",
	} {
		t.Run(line, func(t *testing.T) {
			translation, err := TranslateLine(line)
			require.ErrorIs(t, err, errMaskNotNumber)
			assert.Empty(t, translation.Commands)
			assert.Contains(t, err.Error(), `has the mask "x"`)
			assert.Contains(t, err.Error(), "10.0.0.0/<length>")
		})
	}
}

// TestRefuseMaskNotNumberOnlyJudgesAddresses holds refuseMaskNotNumber to its
// one refusal: an address whose mask is no number.
//
// VALIDATES: a token whose part before the slash is no address, a numeric
// mask and a bare address all pass; only `<address>/<word>` is refused.
// PREVENTS: the X-29 refusal swallowing a token that is no prefix at all.
func TestRefuseMaskNotNumberOnlyJudgesAddresses(t *testing.T) {
	for _, token := range []string{"65000:1/x", "label/x", "10.0.0.0/24", "2001:db8::/64", "10.0.0.0", "10.0.0/x"} {
		assert.NoError(t, refuseMaskNotNumber(token), token)
	}
	for _, token := range []string{"10.0.0.0/x", "2001:db8::/len"} {
		assert.ErrorIs(t, refuseMaskNotNumber(token), errMaskNotNumber, token)
	}
}

// TestOperationalRefusedByKind is X-30 (AC-36) and OD-1.
//
// VALIDATES: an unknown kind and a bare `announce operational` are refused
// naming the command; a known kind is refused naming that Ze does not send
// OPERATIONAL; the API 6 spelling after a selector is refused the same way.
// PREVENTS: a known kind refused as unknown, and an operational line answered done.
func TestOperationalRefusedByKind(t *testing.T) {
	cases := []struct {
		line string
		want error
		name string
	}{
		{line: "announce operational foo afi ipv4 safi unicast", want: errOperationalUnknown, name: "foo"},
		{line: "announce operational", want: errOperationalUnknown, name: "announce operational"},
		{line: "announce operational asm afi ipv4 safi unicast advisory \"hello\"", want: errOperationalNotSent, name: "(asm)"},
		{line: "peer * operational rpcq afi ipv4 safi unicast sequence 1", want: errOperationalNotSent, name: "(rpcq)"},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			translation, err := TranslateLine(tc.line)
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.want), "err = %v, want %v", err, tc.want)
			assert.Empty(t, translation.Commands)
			assert.Contains(t, err.Error(), tc.name)
		})
	}
}

// TestGenericAttributeTranslated checks ExaBGP's generic attribute,
// `attribute [ 0x<code> 0x<flag> 0x<data> ]`, reaches ze's update text under
// the same spelling, a code ze writes from a keyword included, and that a type
// code given twice keeps the first.
//
// VALIDATES: an unknown code is forwarded as written, flags ze refuses
// included (ze's update text refuses them by name); two of different codes
// are both forwarded; a code ze writes from a keyword is forwarded as written,
// for ze's update text to decode into that keyword's typed form; a generic
// attribute after the keyword of the same code, the keyword after a generic
// one, and a second generic one of the same code are each dropped with one
// warning naming the code in hex, as the pin's add_attribute keeps the first
// ("attribute.GENERIC or existing.GENERIC",
// configuration/grammar/tree/static.py at 685f772e); a generic NEXT_HOP never
// drops the next-hop keyword.
// PREVENTS: refusing a line ExaBGP 4.2, 5.0 and 6.0 announce, and forwarding a
// repeat that ze's update text would refuse as an attribute given twice.
func TestGenericAttributeTranslated(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		want    string
		dropped string
	}{
		{
			name: "forwarded as written",
			line: "announce attributes next-hop 1.2.3.4 attribute [ 0x99 0xd0 0x00000064 ] nlri 10.0.0.0/24",
			want: "send bgp * update text nhop 1.2.3.4 attribute [ 0x99 0xd0 0x00000064 ] nlri add 10.0.0.0/24",
		},
		{
			name: "flags ze refuses forwarded for ze to name",
			line: "announce route 10.0.0.0/24 next-hop 1.2.3.4 attribute [ 0x99 0x70 0x00000064 ]",
			want: "send bgp * update text nhop 1.2.3.4 attribute [ 0x99 0x70 0x00000064 ] nlri add 10.0.0.0/24",
		},
		{
			name: "two codes",
			line: "announce route 10.0.0.0/24 next-hop 1.2.3.4 attribute [ 0x99 0xc0 0x01 ] attribute [0x98 0xc0 0x02]",
			want: "send bgp * update text nhop 1.2.3.4 attribute [ 0x99 0xc0 0x01 ] attribute [0x98 0xc0 0x02] nlri add 10.0.0.0/24",
		},
		{
			name: "med forwarded as written",
			line: "announce route 10.0.0.0/24 next-hop 1.2.3.4 attribute [ 0x04 0x80 0x00000064 ]",
			want: "send bgp * update text nhop 1.2.3.4 attribute [ 0x04 0x80 0x00000064 ] nlri add 10.0.0.0/24",
		},
		{
			name: "origin and communities forwarded as written",
			line: "announce route 10.0.0.0/24 next-hop 1.2.3.4 attribute [ 0x01 0x40 0x02 ] attribute [ 0x08 0xc0 0xfde80001fde80002 ]",
			want: "send bgp * update text nhop 1.2.3.4 attribute [ 0x01 0x40 0x02 ] attribute [ 0x08 0xc0 0xfde80001fde80002 ] nlri add 10.0.0.0/24",
		},
		{
			name: "generic next-hop keeps the keyword",
			line: "announce route 10.0.0.0/24 attribute [ 0x03 0x40 0x05060708 ] next-hop 1.2.3.4",
			want: "send bgp * update text attribute [ 0x03 0x40 0x05060708 ] nhop 1.2.3.4 nlri add 10.0.0.0/24",
		},
		{
			name:    "after its keyword",
			line:    "announce route 10.0.0.0/24 next-hop 1.2.3.4 med 7 attribute [ 0x04 0x80 0x00000064 ]",
			want:    "send bgp * update text nhop 1.2.3.4 med 7 nlri add 10.0.0.0/24",
			dropped: "attribute 0x04 twice, through med and through attribute [ 0x04 0x80 0x00000064 ]",
		},
		{
			name:    "before its keyword",
			line:    "announce route 10.0.0.0/24 next-hop 1.2.3.4 attribute [ 0x04 0x80 0x00000064 ] med 7",
			want:    "send bgp * update text nhop 1.2.3.4 attribute [ 0x04 0x80 0x00000064 ] nlri add 10.0.0.0/24",
			dropped: "attribute 0x04 twice, through attribute and through med 7",
		},
		{
			name:    "the same code twice",
			line:    "announce route 10.0.0.0/24 next-hop 1.2.3.4 attribute [ 0x99 0xc0 0x01 ] attribute [ 0x99 0xc0 0x02 ]",
			want:    "send bgp * update text nhop 1.2.3.4 attribute [ 0x99 0xc0 0x01 ] nlri add 10.0.0.0/24",
			dropped: "attribute 0x99 twice, through attribute and through attribute [ 0x99 0xc0 0x02 ]; the first is kept and the second dropped: remove one of the two",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logged := captureWarnings(t)
			translation, err := TranslateLine(tc.line)
			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, commandTexts(translation))
			if tc.dropped == "" {
				assert.NotContains(t, logged.String(), "exabgp-bridge route line")
				return
			}
			assert.Equal(t, 1, strings.Count(logged.String(), "exabgp-bridge route line"), logged.String())
			assert.Contains(t, logged.String(), tc.dropped)
		})
	}
}

// TestGenericAttributeTypedForwarded checks that a generic attribute of a code
// ze writes from its own keyword is forwarded as written whatever its value,
// so ze's update text, which decodes it, is the one place that refuses a
// malformed one and names why.
//
// Method: TranslateLine on an AS_PATH and on a MED two octets short, asserting
// the generic attribute reaches the update text unchanged.
func TestGenericAttributeTypedForwarded(t *testing.T) {
	for _, generic := range []string{"attribute [ 0x02 0x40 0x02010000fde8 ]", "attribute [ 0x04 0x80 0x0064 ]"} {
		translation, err := TranslateLine("announce route 10.0.0.0/24 next-hop 1.2.3.4 " + generic)
		require.NoError(t, err, generic)
		assert.Equal(t, []string{"send bgp * update text nhop 1.2.3.4 " + generic + " nlri add 10.0.0.0/24"}, commandTexts(translation))
	}
}

// TestIPv4AdministratorCommunity pins ExaBGP's `target4:` and `origin4:`.
//
// VALIDATES: each reaches ze as the IPv4 address specific form ExaBGP packs it
// as (type 0x01, a four octet administrator, a two octet local one), whether the
// administrator is written dotted, decimal, hexadecimal or with a trailing `L`,
// and up to the largest four octet value; a value past it is passed as written.
// PREVENTS: ze refusing `target4:71200:12000` as an unknown type, which 5.0
// sent as 01 02 00011620 2EE0 (qa/old/commands-5.0.json).
func TestIPv4AdministratorCommunity(t *testing.T) {
	cases := []struct {
		written string
		want    string
	}{
		{"target4:71200:12000", "target:0.1.22.32:12000"},
		{"origin4:71100:11000", "origin:0.1.21.188:11000"},
		{"TARGET4:1:2", "target:0.0.0.1:2"},
		{"target4:192.0.2.1:5", "target:192.0.2.1:5"},
		{"target4:0x10:5", "target:0.0.0.16:5"},
		{"target4:100L:5", "target:0.0.0.100:5"},
		{"target4:0:0", "target:0.0.0.0:0"},
		{"target4:4294967295:65535", "target:255.255.255.255:65535"},
		{"target4:4294967296:1", "target4:4294967296:1"},
		{"target4:x:1", "target4:x:1"},
		{"target4:1", "target4:1"},
		{"65000:1", "target:65000:1"},
		{"origin:65000:1", "origin:65000:1"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, qualifyExtCommunity(tc.written), tc.written)
	}

	translation, err := TranslateLine("announce route 10.0.0.12/32 next-hop 15.15.15.12 extended-community [target4:71200:12000 origin4:1.2.3.4:7]")
	if err != nil {
		t.Fatalf("TranslateLine: %v", err)
	}
	if len(translation.Commands) != 1 {
		t.Fatalf("commands = %v, want one", translation.Commands)
	}
	got := translation.Commands[0].Text
	if !strings.Contains(got, "[target:0.1.22.32:12000 origin:1.2.3.4:7]") {
		t.Errorf("command %q does not carry the IPv4 administrator forms", got)
	}
}

// TestGroupAddressRouteIsMulticast pins the SAFI of a family-less route.
//
// VALIDATES: `announce route` and `announce attributes ... nlri` of a prefix in
// 224.0.0.0/4, or of an IPv4-mapped IPv6 group written dotted, go out as
// multicast, and the prefixes either side of the range as unicast.
// PREVENTS: ze sending `announce route 239.1.1.0/24` as unicast, where 5.0
// sends SAFI 2 (IP.tosafi; qa/old/commands-5.0.json).
func TestGroupAddressRouteIsMulticast(t *testing.T) {
	cases := []struct {
		line   string
		family string
	}{
		{"announce route 239.1.1.0/24 next-hop 1.1.1.1", " ipv4/multicast "},
		{"announce route 224.0.0.0/24 next-hop 1.1.1.1", " ipv4/multicast "},
		{"announce route 239.255.255.255/32 next-hop 1.1.1.1", " ipv4/multicast "},
		{"announce route 223.255.255.0/24 next-hop 1.1.1.1", " nlri add "},
		{"announce route 240.0.0.0/24 next-hop 1.1.1.1", " nlri add "},
		{"announce attributes next-hop 1.1.1.1 nlri 230.0.0.0/24", " ipv4/multicast "},
		{"announce route ::ffff:239.1.1.1/128 next-hop 2001:db8::1", " ipv6/multicast "},
		{"announce route 2001:db8::/32 next-hop 2001:db8::1", " ipv6/unicast "},
	}
	for _, tc := range cases {
		translation, err := TranslateLine(tc.line)
		if err != nil {
			t.Fatalf("TranslateLine(%q): %v", tc.line, err)
		}
		if len(translation.Commands) != 1 {
			t.Fatalf("TranslateLine(%q) = %v, want one command", tc.line, translation.Commands)
		}
		if got := translation.Commands[0].Text; !strings.Contains(got, tc.family) {
			t.Errorf("TranslateLine(%q) = %q, want it to carry %q", tc.line, got, tc.family)
		}
	}
}
