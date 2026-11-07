//go:build linux

// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- authenticated endpoint mobility
// Related: mobike.go -- mobikeRouteSource, which finds the source MOBIKE moves to
// Related: cycle_addr.go -- resolveCyclePair, the route-chosen local address of a connect cycle
// Related: unmatched.go -- the SPD catch-all this lookup must not cross

package engine

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// routeSourceTo returns the source address the routing table picks for remote.
//
// It asks the routing table itself (RTM_GETROUTE), as strongSwan's kernel-netlink
// get_source_addr does, and never connects a socket. A UDP connect is an output
// route lookup the kernel runs through the XFRM SPD, and the socket's ephemeral
// port is named by no IKE bypass, so under Ze's SPD catch-all DISCARD (the default,
// RFC 4301 Section 4.4.1) the connect fails with EPERM and the new source of a
// moved address would never be found (RFC 4555 Section 3.5).
func routeSourceTo(remote net.IP) (net.IP, error) {
	routes, err := netlink.RouteGet(remote)
	if err != nil {
		return nil, fmt.Errorf("ike: route lookup toward %s failed: %w; check the routing table with: ze show route", remote, err)
	}
	for i := range routes {
		route := &routes[i]
		if route.Src != nil && !route.Src.IsUnspecified() {
			return route.Src, nil
		}
	}
	return nil, fmt.Errorf("ike: the route toward %s names no source address; give the route a preferred source, or the interface an address on it", remote)
}
