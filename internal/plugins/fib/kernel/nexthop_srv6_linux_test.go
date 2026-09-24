//go:build linux

package fibkernel

import (
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
)

// VALIDATES: an SRv6 service route selects IPv6 encapsulation even when its
// NLRI label field carries transposition bits.
// PREVENTS: transmitting an MPLS packet with the SID's function as its label.
// RFC requirement: RFC9252-5-3 positive
func TestSRv6ServiceSIDSelectsIPv6Encapsulation(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8:1:2:abcd::")
	route, err := buildRichRoute(RichRoute{
		Prefix:  netip.MustParsePrefix("192.0.2.0/24"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Labels:  []uint32{0xabcd0},
		SRv6SID: sid,
	})
	if err != nil {
		t.Fatal(err)
	}
	encap, ok := route.Encap.(*netlink.SEG6Encap)
	if !ok {
		t.Fatalf("encapsulation = %T, want IPv6 SEG6", route.Encap)
	}
	if encap.Mode != 1 || len(encap.Segments) != 1 || !encap.Segments[0].Equal(sid.AsSlice()) {
		t.Fatalf("SEG6 encapsulation = %+v, want encap mode with SID %v", encap, sid)
	}
}

// VALIDATES: ordinary labeled routes retain MPLS encapsulation when no Service
// SID is present.
// PREVENTS: treating every NLRI label as transposed SRv6 information.
// RFC requirement: RFC9252-5-3 negative
func TestRouteWithoutServiceSIDKeepsMPLSEncapsulation(t *testing.T) {
	route, err := buildRichRoute(RichRoute{
		Prefix:  netip.MustParsePrefix("192.0.2.0/24"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Labels:  []uint32{16000},
	})
	if err != nil {
		t.Fatal(err)
	}
	encap, ok := route.Encap.(*netlink.MPLSEncap)
	if !ok {
		t.Fatalf("encapsulation = %T, want MPLS", route.Encap)
	}
	if len(encap.Labels) != 1 || encap.Labels[0] != 16000 {
		t.Fatalf("labels = %v, want [16000]", encap.Labels)
	}
}
