// Design: docs/architecture/exabgp-bridge.md -- the route attributes the bridge translates
// Overview: bridge_attribute.go -- parseRouteAttributes, the parser under test

package bridge

import (
	"strings"
	"testing"
)

// TestRepeatedListAttributeMerged verifies that a community list given twice
// on one line is sent as one list holding both.
//
// VALIDATES: the pin merges a repeated list attribute, for an API command as
// for a file ("a list one given twice is merged, any other is refused",
// add_attribute, configuration/grammar/tree/static.py at f9bfb853b, whose
// LIST_ATTRIBUTES are COMMUNITY, LARGE_COMMUNITY and EXTENDED_COMMUNITY), and
// 5.0 merged a repeated extended-community (Attributes.add, 5.0.1): it sent
// both lists in 54 recorded commands (testdata/exabgp-old/commands-5.0.json.gz).
// PREVENTS: the bridge keeping the first list and dropping the second, which
// X-27's keep-first rule asks only of an attribute that is not a list.
func TestRepeatedListAttributeMerged(t *testing.T) {
	for _, tc := range []struct {
		line string
		want string
	}{
		{
			"announce route 10.0.0.0/24 next-hop 1.2.3.4 extended-community 0:0 med 5 extended-community [ origin:2.3.4.5:6789 redirect:65500:12345 ]",
			"send bgp * update text nhop 1.2.3.4 extended-community [target:0:0 origin:2.3.4.5:6789 redirect:65500:12345] med 5 nlri add 10.0.0.0/24",
		},
		{
			"announce route 10.0.0.0/24 next-hop 1.2.3.4 community 1:1 community [ 2:2 3:3 ]",
			"send bgp * update text nhop 1.2.3.4 community [1:1 2:2 3:3] nlri add 10.0.0.0/24",
		},
		{
			"announce route 10.0.0.0/24 next-hop 1.2.3.4 large-community [ 1:2:3 ] large-community 4:5:6 large-community 7:8:9",
			"send bgp * update text nhop 1.2.3.4 large-community [1:2:3 4:5:6 7:8:9] nlri add 10.0.0.0/24",
		},
	} {
		logged := captureWarnings(t)
		translation, err := TranslateLine(tc.line)
		if err != nil {
			t.Errorf("%q: %v", tc.line, err)
			continue
		}
		texts := commandTexts(translation)
		if len(texts) != 1 || texts[0] != tc.want {
			t.Errorf("%q became %q, want %q", tc.line, texts, tc.want)
		}
		if strings.Contains(logged.String(), "is dropped") {
			t.Errorf("%q: a merged list logged a dropped value: %s", tc.line, logged.String())
		}
	}
}

// TestSRv6BehaviorDefaultsOpaque verifies that an SRv6 service SID written
// without an endpoint behavior is sent with the opaque behavior.
//
// VALIDATES: ExaBGP 5.0 reads `bgp-prefix-sid-srv6 ( l3-service <sid> )` with
// `behavior = 0xFFFF` until a value is written (prefix_sid_srv6,
// configuration/static/mpls.py, 5.0.1), which RFC 9252 Section 3.1 names: "The
// opaque SRv6 Endpoint Behavior (i.e., value 0xFFFF) MAY be used when the
// advertising router wishes to abstract the actual behavior of its locally
// instantiated SRv6 SID." ze's update text reads an omitted behavior as 0, so
// the bridge writes the value ExaBGP sends.
// PREVENTS: a migrated script announcing endpoint behavior 0 where ExaBGP sent
// 0xFFFF (51 recorded commands across 5.0 and 4.2).
func TestSRv6BehaviorDefaultsOpaque(t *testing.T) {
	for _, tc := range []struct {
		line string
		want string
	}{
		{
			"announce route 10.0.0.0/24 next-hop 1.2.3.4 bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:: )",
			" bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:: 0xFFFF ) ",
		},
		{
			"announce route 10.0.0.0/24 next-hop 1.2.3.4 bgp-prefix-sid-srv6 (l2-service 2001:db8:1::)",
			" bgp-prefix-sid-srv6 ( l2-service 2001:db8:1:: 0xFFFF ) ",
		},
		{
			"announce route 10.0.0.0/24 next-hop 1.2.3.4 bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:: 0x13 )",
			" bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:: 0x13 ) ",
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

// TestRepeatedNLRIFieldLastWins verifies that a label, a route distinguisher
// or a path identifier given twice on one line is sent as the last one given.
//
// VALIDATES: these are fields of the NLRI rather than attributes, and ExaBGP
// sets each as it reads it (Collected.apply: `if spec.target == Target.NLRI:
// self.settings.set(spec.field, value)`, configuration/grammar/tree/static.py
// at f9bfb853b, where `label`, `rd`, `route-distinguisher` and
// `path-information` are Target.NLRI), so the last one read is the route's.
// 5.0 sent label 100 for `label [ 110 ] ... label 100` in the recorded
// commands (testdata/exabgp-old/commands-5.0.json.gz).
// PREVENTS: the bridge applying X-27's keep-first, which add_attribute applies
// to attributes only, to a field of the NLRI.
func TestRepeatedNLRIFieldLastWins(t *testing.T) {
	for _, tc := range []struct {
		line string
		want string
	}{
		{
			"announce route 10.0.0.0/24 next-hop 1.2.3.4 rd 1:1 label [ 110 ] label 100",
			"nhop 1.2.3.4 rd 1:1 label 100 mp ipv4/mpls-vpn add 10.0.0.0/24",
		},
		{
			"announce route 10.0.0.0/24 next-hop 1.2.3.4 route-distinguisher 1:1 label 5 rd 2:2",
			"nhop 1.2.3.4 rd 2:2 label 5 mp ipv4/mpls-vpn add 10.0.0.0/24",
		},
		{
			"announce route 10.0.0.0/24 next-hop 1.2.3.4 path-information 1 path-information 0.0.0.9",
			"nhop 1.2.3.4 path-information 9 nlri add 10.0.0.0/24",
		},
	} {
		logged := captureWarnings(t)
		translation, err := TranslateLine(tc.line)
		if err != nil {
			t.Errorf("%q: %v", tc.line, err)
			continue
		}
		texts := commandTexts(translation)
		if len(texts) != 1 || !strings.HasSuffix(texts[0], " "+tc.want) {
			t.Errorf("%q became %q, want %q", tc.line, texts, tc.want)
		}
		if strings.Contains(logged.String(), "is dropped") {
			t.Errorf("%q: a field of the NLRI logged a dropped value: %s", tc.line, logged.String())
		}
	}
}
