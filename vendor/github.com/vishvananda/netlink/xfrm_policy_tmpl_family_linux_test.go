package netlink

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink/nl"
)

// TestXfrmPolicyTmplFamilyNoDst checks the family written into a policy
// template: the selector's family when the template has no destination, as in
// transport mode, and the destination's family otherwise.
func TestXfrmPolicyTmplFamilyNoDst(t *testing.T) {
	for _, tc := range []struct {
		name string
		dst  net.IP
		sel  uint16
		want uint16
	}{
		{name: "nil destination, ipv6 selector", dst: nil, sel: nl.FAMILY_V6, want: nl.FAMILY_V6},
		{name: "empty destination, ipv6 selector", dst: net.IP{}, sel: nl.FAMILY_V6, want: nl.FAMILY_V6},
		{name: "nil destination, ipv4 selector", dst: nil, sel: nl.FAMILY_V4, want: nl.FAMILY_V4},
		{name: "ipv4 destination, ipv6 selector", dst: net.ParseIP("192.0.2.1"), sel: nl.FAMILY_V6, want: nl.FAMILY_V4},
		{name: "ipv6 destination, ipv4 selector", dst: net.ParseIP("2001:db8::1"), sel: nl.FAMILY_V4, want: nl.FAMILY_V6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := XfrmPolicyTmpl{Dst: tc.dst, Proto: XFRM_PROTO_ESP, Mode: XFRM_MODE_TRANSPORT}
			if got := xfrmUserTmplFamily(tmpl, tc.sel); got != tc.want {
				t.Errorf("template family = %d, want %d", got, tc.want)
			}
		})
	}
}
