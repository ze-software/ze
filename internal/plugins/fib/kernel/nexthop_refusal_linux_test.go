//go:build linux

// Design: docs/architecture/core-design.md -- Linux netlink route programming
// Related: nexthop_linux.go -- buildRichRoute, buildMultiPath, buildGateway

package fibkernel

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/sysrib/events"
)

// Goal: a route that names an outgoing device this host does not have is
// refused, and the refusal names the device.
// Method: build a single-path route whose Interface no link carries. The
// resolver refuses it whether or not an interface backend is loaded
// (iface.ResolveIndex), so the case needs no kernel state and no privilege.
// Expected: no route, and an error that names the interface, because a route
// built without the index would reach the kernel with no next-hop device.
func TestBuildRichRouteRefusesAnUnknownInterface(t *testing.T) {
	route, err := buildRichRoute(RichRoute{
		Prefix:    netip.MustParsePrefix("192.0.2.0/24"),
		Interface: "ze-absent9",
	})
	if err == nil {
		t.Fatalf("route out of an absent device was built: %+v", route)
	}
	if route != nil {
		t.Errorf("refused route still returned %+v", route)
	}
	if !strings.Contains(err.Error(), `interface "ze-absent9"`) {
		t.Errorf("refusal %q does not name the interface", err)
	}
}

// Goal: an IPv6 route whose gateway is IPv4 is refused, on the single path
// and on an ECMP member alike, with the message buildGateway writes.
// Method: build each form through buildRichRoute.
// Expected: no route, and the error names the route's prefix and the IPv4
// gateway it cannot use. TestKernelRejectsUnusableGatewayMembers proves that
// some error is returned; this test pins which refusal it is.
func TestBuildRichRouteGatewayRefusalNamesTheGateway(t *testing.T) {
	const want = "IPv6 route 2001:db8::/64 cannot use IPv4 gateway 192.0.2.1"
	cases := []struct {
		name  string
		route RichRoute
	}{
		{name: "single path", route: RichRoute{
			Prefix:  netip.MustParsePrefix("2001:db8::/64"),
			NextHop: netip.MustParseAddr("192.0.2.1"),
		}},
		{name: "ECMP member", route: RichRoute{
			Prefix:    netip.MustParsePrefix("2001:db8::/64"),
			NextHop:   netip.MustParseAddr("2001:db8:1::1"),
			ECMPPaths: []events.ECMPPath{{NextHop: netip.MustParseAddr("192.0.2.1")}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			route, err := buildRichRoute(tc.route)
			if err == nil {
				t.Fatalf("IPv4 gateway for an IPv6 route was accepted: %+v", route)
			}
			if route != nil {
				t.Errorf("refused route still returned %+v", route)
			}
			if err.Error() != want {
				t.Errorf("refusal = %q, want %q", err, want)
			}
		})
	}
}
