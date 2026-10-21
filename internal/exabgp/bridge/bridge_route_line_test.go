// Design: docs/architecture/exabgp-bridge.md -- the route attributes the bridge translates
// Related: bridge_attribute.go -- bridgeRouteAttrs, the attribute table
// Related: bridge_route_forms.go -- familyForAttributes, the family a line names

package bridge

import (
	"strings"
	"testing"
)

// TestPrefixSIDLabelIndexTranslated verifies that ExaBGP's label-index BGP
// Prefix-SID reaches ze's update text unchanged.
//
// VALIDATES: ExaBGP 4.2, 5.0 and the pin read `bgp-prefix-sid [ <index> ]` and
// `bgp-prefix-sid [ <index>, [ ( <base>,<range> ) ... ] ]` on an API route
// (prefix_sid, configuration/static/mpls.py in 4.2.25 and 5.0.1), and ze's
// update text reads the same spelling under the same name.
// PREVENTS: the bridge refusing the line by name (36 recorded commands) or
// passing the keyword where update text reads a prefix (32).
func TestPrefixSIDLabelIndexTranslated(t *testing.T) {
	for _, tc := range []struct {
		line string
		want string
	}{
		{
			"announce route 10.0.0.0/24 next-hop 1.2.3.4 bgp-prefix-sid [ 777 ]",
			" bgp-prefix-sid [ 777 ] ",
		},
		{
			"announce attributes next-hop 1.2.3.4 bgp-prefix-sid [ 300, [ ( 800000,4096 ) , ( 1000000,5000 ) ] ] nlri 10.0.0.0/24",
			" bgp-prefix-sid [ 300, [ ( 800000,4096 ) , ( 1000000,5000 ) ] ] ",
		},
	} {
		translation, err := TranslateLine(tc.line)
		if err != nil {
			t.Errorf("%q: %v", tc.line, err)
			continue
		}
		texts := commandTexts(translation)
		if len(texts) != 1 || !strings.Contains(texts[0]+" ", tc.want) {
			t.Errorf("%q became %q, want it to hold %q", tc.line, texts, tc.want)
		}
	}
}

// TestLabeledRouteNamesZeFamily verifies that a family-less ExaBGP route with
// a bare label is sent under ze's name for SAFI 4, and that a bare address in
// it is given its host length.
//
// VALIDATES: ExaBGP writes SAFI 4 as `nlri-mpls` (SAFI._names,
// protocol/family.py) and ze registers it as `mpls-label`; the bridge reads a
// bare label as a labeled route (familyOfRoute, familyForAttributes), so the
// family it builds must be ze's word.
// PREVENTS: `ipv4/nlri-mpls` reaching ze's update text, which knows no such
// family and refused every labeled `announce route` and `announce attributes`
// line (14 recorded commands).
func TestLabeledRouteNamesZeFamily(t *testing.T) {
	for _, tc := range []struct {
		line string
		want string
	}{
		{"announce route 198.51.100.100/32 next-hop 198.51.100.1 label 800001", "ipv4/mpls-label"},
		{"announce route 2001:db8::/32 next-hop 2001:db8::1 label [ 100 ]", "ipv6/mpls-label"},
		{"announce attributes next-hop 1.2.3.4 label 100 nlri 10.0.0.1", "ipv4/mpls-label"},
		{"announce attributes next-hop 1.2.3.4 label 100 nlri 10.0.0.1", "10.0.0.1/32"},
	} {
		translation, err := TranslateLine(tc.line)
		if err != nil {
			t.Errorf("%q: %v", tc.line, err)
			continue
		}
		texts := commandTexts(translation)
		if len(texts) != 1 || !strings.Contains(texts[0], tc.want) || strings.Contains(texts[0], "nlri-mpls") {
			t.Errorf("%q became %q, want it to hold %q and not nlri-mpls", tc.line, texts, tc.want)
		}
	}
}

// TestWatchdogAndNameWithNoValue verifies that a route line ending in
// `watchdog` or `name` with no value is translated, not refused.
//
// VALIDATES: the pin reads a missing word as the empty word (Words.take at
// the end answers the empty token, configuration/grammar/words.py) and lists
// ” among WATCHDOG's examples (configuration/grammar/types/bgp.py); 4.2 and
// 5.0 took the empty tokeniser() answer as the name (watchdog and name,
// configuration/static/parser.py) and sent the route. A route under the empty
// watchdog is one no `announce watchdog <name>` can name, so on the wire it is
// a route with no watchdog, and `name` never reaches the wire.
// PREVENTS: 48 recorded commands refused with "written with no value".
func TestWatchdogAndNameWithNoValue(t *testing.T) {
	for _, line := range []string{
		"withdraw route 10.0.0.0/24 watchdog",
		"withdraw route 10.0.0.0/24 name",
		"announce route 10.0.0.0/24 next-hop 1.2.3.4 watchdog",
		"announce route 10.0.0.0/24 next-hop 1.2.3.4 name",
	} {
		translation, err := TranslateLine(line)
		if err != nil {
			t.Errorf("%q: %v", line, err)
			continue
		}
		texts := commandTexts(translation)
		if len(texts) != 1 || strings.Contains(texts[0], "watchdog") || !strings.Contains(texts[0], "10.0.0.0/24") {
			t.Errorf("%q became %q, want one command for 10.0.0.0/24 with no watchdog", line, texts)
		}
	}
	// The warning for a watchdog with no name names the fix, as every message
	// an operator reads must.
	logged := captureWarnings(t)
	if _, err := TranslateLine("announce route 10.0.0.0/24 next-hop 1.2.3.4 watchdog"); err != nil {
		t.Fatalf("watchdog at end of line: %v", err)
	}
	if !strings.Contains(logged.String(), "write watchdog <name> to tie it to a watchdog") {
		t.Errorf("warning %q does not name the fix", logged.String())
	}
	// A value-less attribute that is not at the end still takes the next
	// word as its value, as ExaBGP does, so this is unchanged.
	if _, err := TranslateLine("announce route 10.0.0.0/24 next-hop 1.2.3.4 med"); err == nil {
		t.Errorf("med with no value was translated, want a refusal")
	}
}

// TestMUPLeadingFieldNamed verifies that the field ExaBGP writes bare after a
// MUP route type reaches ze under the name ze's encoder reads for that type.
//
// VALIDATES: ExaBGP reads `mup-isd <ip prefix>`, `mup-dsd <ip address>`,
// `mup-t1st <ip prefix>` and `mup-t2st <endpoint address>` (the grammar
// comments in configuration/static/mpls.py, 5.0.1), and ze's MUP encoder reads
// `prefix` for the first and third and `address` for the other two
// (nlri/mup/encode.go).
// PREVENTS: every DSD and T2ST line refused with "requires address".
func TestMUPLeadingFieldNamed(t *testing.T) {
	for _, tc := range []struct {
		line string
		want string
	}{
		{"announce ipv4 mup mup-isd 10.0.1.0/24 rd 100:100 next-hop 10.0.0.2", " route-type mup-isd prefix 10.0.1.0/24 "},
		{"announce ipv4 mup mup-dsd 10.0.0.1 rd 100:100 next-hop 10.0.0.2", " route-type mup-dsd address 10.0.0.1 "},
		{"announce ipv4 mup mup-t1st 192.168.0.1/32 rd 100:100 teid 12345 qfi 9 endpoint 10.0.0.1 next-hop 10.0.0.2", " route-type mup-t1st prefix 192.168.0.1/32 "},
		{"announce ipv6 mup mup-t2st 2001::1 rd 100:100 teid 12345/32 next-hop 2001::2", " route-type mup-t2st address 2001::1 "},
	} {
		translation, err := TranslateLine(tc.line)
		if err != nil {
			t.Errorf("%q: %v", tc.line, err)
			continue
		}
		texts := commandTexts(translation)
		if len(texts) != 1 || !strings.Contains(texts[0]+" ", tc.want) {
			t.Errorf("%q became %q, want it to hold %q", tc.line, texts, tc.want)
		}
	}
}
