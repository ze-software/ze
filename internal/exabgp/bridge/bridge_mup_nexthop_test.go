// Design: docs/architecture/exabgp-bridge.md -- the commands a 4.2 or 5.0 script writes
// Overview: bridge_attribute.go -- parseFamilyRouteAttributes, bridgeNextHopLastFamily, mupNextHop

package bridge

import (
	"strings"
	"testing"
)

// TestRepeatedNextHopByFamily verifies which next-hop a route line that gives
// two is sent with, family by family.
//
// VALIDATES: a mup and an mcast-vpn route take the LAST next-hop, as ExaBGP's
// SelectLine sets the next hop each time it reads one (select.py at 685f772e)
// and 5.0 sent (qa/old/commands-5.0.json); every other family keeps the first
// (static.first_next_hop). An IPv4 next-hop on an IPv6 mup route is sent as
// `::ffff:<ipv4>`, as MupNextHop maps it, and 5.0 sent it.
// PREVENTS: `announce ipv4 mcast-vpn ... next-hop 10.10.6.4 ... next-hop
// 1.2.3.4` going out with 10.10.6.4, which 5.0 and 6.0 never sent.
func TestRepeatedNextHopByFamily(t *testing.T) {
	for _, tc := range []struct {
		line    string
		want    string
		dropped string
	}{
		{"announce ipv4 mcast-vpn source-ad source 10.99.12.4 group 239.251.255.228 rd 65000:99999 next-hop 10.10.6.4 extended-community [target:65000:99999] next-hop 1.2.3.4",
			"nhop 1.2.3.4 ", "10.10.6.4"},
		{"announce ipv6 mcast-vpn shared-join rp fd00::1 group ff0e::1 rd 65000:99999 source-as 65000 next-hop 10.10.6.3 next-hop 10.0.0.1",
			"nhop 10.0.0.1 ", "10.10.6.3"},
		{"announce ipv4 mup mup-isd 10.0.1.0/24 rd 100:100 next-hop 2001::1 next-hop 10.0.0.1",
			"nhop 10.0.0.1 ", "2001::1"},
		{"announce ipv6 mup mup-t2st 2001::1 rd 100:100 teid 12345/32 next-hop ::ffff:10.0.0.2 next-hop 1.2.3.4",
			"nhop ::ffff:1.2.3.4 ", "10.0.0.2"},
		{"announce ipv6 mup mup-dsd 2001::1 rd 100:100 next-hop 10.0.0.2",
			"nhop ::ffff:10.0.0.2 ", "nhop 10.0.0.2"},
		{"announce ipv6 unicast 2001:db8::/32 next-hop 2001::1 next-hop 2001::2",
			"nhop 2001::1 ", "2001::2"},
	} {
		translation, err := TranslateLine(tc.line)
		if err != nil {
			t.Errorf("%q: %v", tc.line, err)
			continue
		}
		if len(translation.Commands) == 0 {
			t.Errorf("%q: no command", tc.line)
			continue
		}
		text := translation.Commands[0].Text + " "
		if !strings.Contains(text, tc.want) {
			t.Errorf("%q became %q, want %q", tc.line, text, tc.want)
		}
		if strings.Contains(text, tc.dropped) {
			t.Errorf("%q became %q, which still carries %q", tc.line, text, tc.dropped)
		}
	}
}
