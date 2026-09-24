//go:build linux

// Design: docs/architecture/rib/unified-locrib.md -- each ECMP member owns its label stack.
package fibkernel

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/sysrib/events"
)

// Different protocols can supply equal-cost paths with different label stacks.
// Netlink must encode each stack on its own member, leaving a plain member plain.
func TestKernelECMPRetainsMemberLabels(t *testing.T) {
	route, err := buildRichRoute(RichRoute{
		Prefix:  netip.MustParsePrefix("198.51.100.0/24"),
		NextHop: netip.MustParseAddr("192.0.2.1"), Labels: []uint32{16000},
		ECMPPaths: []events.ECMPPath{
			{NextHop: netip.MustParseAddr("192.0.2.2"), Labels: []uint32{17000, 18000}, Weight: 3, OnLink: true},
			{NextHop: netip.MustParseAddr("192.0.2.3")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.Encap != nil || len(route.MultiPath) != 3 {
		t.Fatalf("route-level encapsulation %v, members %+v", route.Encap, route.MultiPath)
	}
	for index, want := range [][]int{{16000}, {17000, 18000}} {
		encap, ok := route.MultiPath[index].Encap.(*netlink.MPLSEncap)
		if !ok || !slices.Equal(encap.Labels, want) {
			t.Fatalf("member %d encap = %+v, want %v", index, route.MultiPath[index].Encap, want)
		}
	}
	if route.MultiPath[2].Encap != nil {
		t.Fatalf("plain member acquired labels: %+v", route.MultiPath[2].Encap)
	}
}

// A Service SID overrides transposition label fields for every ECMP member.
func TestKernelSRv6ECMPOverridesLabelFields(t *testing.T) {
	sid := netip.MustParseAddr("2001:db8::1234")
	route, err := buildRichRoute(RichRoute{
		Prefix:  netip.MustParsePrefix("198.51.100.0/24"),
		NextHop: netip.MustParseAddr("2001:db8::1"), Labels: []uint32{0x12340}, SRv6SID: sid,
		ECMPPaths: []events.ECMPPath{{NextHop: netip.MustParseAddr("2001:db8::2"), Labels: []uint32{0x12340}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.Encap != nil || len(route.MultiPath) != 2 {
		t.Fatalf("route-level encapsulation %v, members %+v", route.Encap, route.MultiPath)
	}
	for index, member := range route.MultiPath {
		encap, ok := member.Encap.(*netlink.SEG6Encap)
		if !ok || len(encap.Segments) != 1 || !encap.Segments[0].Equal(sid.AsSlice()) {
			t.Fatalf("member encapsulation = %+v, want SID %v", member.Encap, sid)
		}
		gateway := netip.MustParseAddr("2001:db8::1")
		if index == 1 {
			gateway = netip.MustParseAddr("2001:db8::2")
		}
		via, ok := member.Via.(*netlink.Via)
		if !ok || via.AddrFamily != unix.AF_INET6 || !via.Addr.Equal(gateway.AsSlice()) || member.Gw != nil {
			t.Fatalf("service route member %d gateway = %+v", index, member)
		}
	}
}

func TestKernelGatewayFamilyEncoding(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		gateway string
		via     bool
	}{
		{name: "IPv4", prefix: "198.51.100.0/24", gateway: "192.0.2.1"},
		{name: "IPv6", prefix: "2001:db8:1::/64", gateway: "2001:db8::1"},
		{name: "IPv4 through IPv6", prefix: "198.51.100.0/24", gateway: "fe80::2", via: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := netip.MustParseAddr(tt.gateway)
			input := RichRoute{
				Prefix: netip.MustParsePrefix(tt.prefix), NextHop: gateway,
				Metric: 77, OnLink: true, Weight: 3, Labels: []uint32{16000},
			}
			route, err := buildRichRoute(input)
			if err != nil {
				t.Fatal(err)
			}
			checkGateway := func(gw []byte, via netlink.Destination) {
				t.Helper()
				if tt.via {
					encoded, ok := via.(*netlink.Via)
					if !ok || encoded.AddrFamily != unix.AF_INET6 || !encoded.Addr.Equal(gateway.AsSlice()) || len(gw) != 0 {
						t.Fatalf("gateway = %v, via = %v; want IPv6 RTA_VIA for %s", gw, via, gateway)
					}
					return
				}
				if !slices.Equal(gw, gateway.AsSlice()) || via != nil {
					t.Fatalf("gateway = %v, via = %v; want RTA_GATEWAY for %s", gw, via, gateway)
				}
			}
			plain, err := buildRoute(tt.prefix, tt.gateway)
			if err != nil {
				t.Fatal(err)
			}
			checkGateway(plain.Gw, plain.Via)
			checkGateway(route.Gw, route.Via)
			if route.Priority != 77 || route.Flags&unix.RTNH_F_ONLINK == 0 {
				t.Fatalf("single path lost priority or adjacency: %+v", route)
			}
			if encap, ok := route.Encap.(*netlink.MPLSEncap); !ok || !slices.Equal(encap.Labels, []int{16000}) {
				t.Fatalf("single path lost label stack: %v", route.Encap)
			}
			input.ECMPPaths = []events.ECMPPath{{NextHop: gateway, Weight: 2, OnLink: true}}
			route, err = buildRichRoute(input)
			if err != nil {
				t.Fatal(err)
			}
			if route.Priority != 77 || route.Gw != nil || route.Via != nil || len(route.MultiPath) != 2 {
				t.Fatalf("multipath route = %+v", route)
			}
			for index, member := range route.MultiPath {
				checkGateway(member.Gw, member.Via)
				if member.Hops != 2-index || member.Flags&unix.RTNH_F_ONLINK == 0 {
					t.Fatalf("member %d lost weight or adjacency: %+v", index, member)
				}
			}
		})
	}
}

func TestKernelGatewayFamilyBackupPromotion(t *testing.T) {
	route, err := buildRichRoute(RichRoute{
		Prefix:  netip.MustParsePrefix("198.51.100.0/24"),
		NextHop: netip.MustParseAddr("fe80::1"), OnLink: true, Weight: 3,
		Backup: []events.ECMPPath{{
			NextHop: netip.MustParseAddr("fe80::2"), OnLink: true, Weight: 2, Labels: []uint32{16000},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.Gw != nil || route.Via != nil || len(route.MultiPath) != 2 {
		t.Fatalf("backup promotion left a route-level gateway: %+v", route)
	}
	for index, member := range route.MultiPath {
		via, ok := member.Via.(*netlink.Via)
		want := netip.MustParseAddr("fe80::1")
		flags := unix.RTNH_F_ONLINK
		if index == 1 {
			want = netip.MustParseAddr("fe80::2")
			flags |= unix.RTNH_F_LINKDOWN
		}
		if !ok || via.AddrFamily != unix.AF_INET6 || !via.Addr.Equal(want.AsSlice()) || member.Gw != nil {
			t.Fatalf("member %d gateway encoding = %+v", index, member)
		}
		if member.Flags != flags || member.Hops != 2-index {
			t.Fatalf("member %d lost weight or flags: %+v", index, member)
		}
	}
	encap, ok := route.MultiPath[1].Encap.(*netlink.MPLSEncap)
	if !ok || !slices.Equal(encap.Labels, []int{16000}) {
		t.Fatalf("backup lost repair stack: %v", route.MultiPath[1].Encap)
	}
}

func TestKernelRejectsUnusableGatewayMembers(t *testing.T) {
	tests := []struct {
		name  string
		route RichRoute
	}{
		{name: "IPv6 through IPv4", route: RichRoute{
			Prefix: netip.MustParsePrefix("2001:db8::/64"), NextHop: netip.MustParseAddr("192.0.2.1"),
		}},
		{name: "reverse-family ECMP member", route: RichRoute{
			Prefix: netip.MustParsePrefix("2001:db8::/64"), NextHop: netip.MustParseAddr("2001:db8:1::1"),
			ECMPPaths: []events.ECMPPath{{NextHop: netip.MustParseAddr("192.0.2.1")}},
		}},
		{name: "empty ECMP member", route: RichRoute{
			Prefix: netip.MustParsePrefix("198.51.100.0/24"), NextHop: netip.MustParseAddr("fe80::1"),
			ECMPPaths: []events.ECMPPath{{}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, err := buildRichRoute(tt.route)
			if err == nil || route != nil {
				t.Fatalf("unusable member was accepted or dropped: route=%+v error=%v", route, err)
			}
		})
	}
}
