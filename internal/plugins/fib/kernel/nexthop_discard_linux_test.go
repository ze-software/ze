// VALIDATES: spec-bcp194-6-blackhole AC-2 -- a discarding route reaches the
// kernel. Linux rejects RTN_BLACKHOLE, RTN_UNREACHABLE and RTN_PROHIBIT that
// carry a gateway, a device or multipath. Every BGP path resolves a next-hop.
// So buildRichRoute must drop the forwarding attributes for those three types,
// or the kernel programs nothing at all.
// PREVENTS: an RFC 7999 BLACKHOLE route that is honored, stamped, carried down
// both rails, and then refused by netlink with EINVAL where no test looks.

//go:build linux

package fibkernel

import (
	"net/netip"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"

	"golang.org/x/sys/unix"
)

func TestBuildRichRouteDiscardCarriesNoNextHop(t *testing.T) {
	cases := []struct {
		name     string
		routeT   sysribevents.RouteType
		wantType int
	}{
		{"blackhole", sysribevents.RouteTypeBlackhole, unix.RTN_BLACKHOLE},
		{"unreachable", sysribevents.RouteTypeUnreachable, unix.RTN_UNREACHABLE},
		{"prohibit", sysribevents.RouteTypeProhibit, unix.RTN_PROHIBIT},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Everything a BGP-learned discard route actually carries: a resolved
			// next-hop, and the label stack and ECMP set of the path it displaced.
			r := RichRoute{
				Prefix:    netip.MustParsePrefix("192.0.2.1/32"),
				NextHop:   netip.MustParseAddr("10.0.0.2"),
				RouteType: tc.routeT,
				Metric:    200,
				Labels:    []uint32{16010},
				ECMPPaths: []sysribevents.ECMPPath{{
					NextHop: netip.MustParseAddr("10.0.0.3"),
				}},
			}
			route, err := buildRichRoute(r)
			if err != nil {
				t.Fatalf("buildRichRoute: %v", err)
			}
			if route.Type != tc.wantType {
				t.Fatalf("route.Type = %d, want %d", route.Type, tc.wantType)
			}
			if route.Gw != nil {
				t.Errorf("route.Gw = %v, want nil: the kernel refuses a gateway on this type", route.Gw)
			}
			if route.MultiPath != nil {
				t.Errorf("route.MultiPath = %v, want nil: the kernel refuses multipath on this type", route.MultiPath)
			}
			if route.LinkIndex != 0 {
				t.Errorf("route.LinkIndex = %d, want 0: the kernel refuses a device on this type", route.LinkIndex)
			}
			if route.Encap != nil {
				t.Error("route.Encap set: an encap describes how to reach a next-hop this route does not have")
			}
			// The fields that identify the route are still owed.
			if route.Dst == nil || route.Dst.String() != "192.0.2.1/32" {
				t.Errorf("route.Dst = %v, want 192.0.2.1/32", route.Dst)
			}
			if route.Priority != 200 {
				t.Errorf("route.Priority = %d, want 200", route.Priority)
			}
			if route.Protocol != rtprotZE {
				t.Errorf("route.Protocol = %v, want %v", route.Protocol, rtprotZE)
			}
		})
	}
}

// A unicast route must be unaffected: the suppression is scoped to the types
// the kernel refuses, not applied to every rich route.
func TestBuildRichRouteUnicastKeepsNextHop(t *testing.T) {
	for _, tc := range []struct {
		name   string
		routeT sysribevents.RouteType
	}{
		{"explicit unicast", sysribevents.RouteTypeUnicast},
		{"unset", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			route, err := buildRichRoute(RichRoute{
				Prefix:    netip.MustParsePrefix("192.0.2.0/24"),
				NextHop:   netip.MustParseAddr("10.0.0.2"),
				RouteType: tc.routeT,
				Metric:    100,
				Labels:    []uint32{16010},
			})
			if err != nil {
				t.Fatalf("buildRichRoute: %v", err)
			}
			if route.Type != unix.RTN_UNICAST {
				t.Fatalf("route.Type = %d, want RTN_UNICAST", route.Type)
			}
			if route.Gw == nil || route.Gw.String() != "10.0.0.2" {
				t.Fatalf("route.Gw = %v, want 10.0.0.2", route.Gw)
			}
			if route.Encap == nil {
				t.Error("route.Encap = nil, want the MPLS label stack")
			}
		})
	}
}
