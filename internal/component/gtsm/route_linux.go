//go:build linux

// Design: rfc/short/rfc5082.md -- the transmit half of the related-message rule
// Overview: gtsm.go -- SetPeers, which calls into this file first
//
// RFC 5082 Section 3: "The TTL field in all IP packets used for transmission
// of messages associated with GTSM-enabled protocol sessions MUST be set to
// 255. This also applies to the related ICMP error handling messages."
//
// A protocol socket's IP_TTL does not reach an ICMP error the kernel generates
// on its own: that error is built by the per-namespace ICMP socket, whose
// unicast TTL is unset, so the kernel falls through to the route. Linux
// 7.2 ip_select_ttl (net/ipv4/ip_output.c) reads ip4_dst_hoplimit, which
// answers with the RTAX_HOPLIMIT metric of the route when the route carries
// one and with net.ipv4.ip_default_ttl when it does not. ip6_dst_hoplimit is
// the same answer for IPv6.
//
// So the metric is what ze installs, on a host route to the peer, and only to
// the peer. A sysctl would raise the TTL of every locally generated packet on
// the box, including those of sessions that asked for no GTSM.

package gtsm

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"sync"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/core/slogutil"
)

var logger = sync.OnceValue(func() *slog.Logger { return slogutil.Logger("gtsm") })

// routeAdd, routeDel and routeResolve are the three netlink calls this file
// makes. They are vars so a test can drive the reconcile without a kernel; the
// tagged proofs use the real ones.
var (
	routeAdd     = netlink.RouteReplace
	routeDel     = netlink.RouteDel
	routeResolve = netlink.RouteGet
	routeList    = func(family int) ([]netlink.Route, error) { return netlink.RouteList(nil, family) }
)

// applyHopLimitRoutes installs a host route for every peer in wanted that asks
// for an outgoing hop limit, and removes the route of every peer in previous
// that wanted no longer names.
//
// A peer whose route cannot be installed is reported and the reconcile
// continues. The peer's address is often unreachable at the moment the config
// is applied, because the interface carrying it has not come up yet, and a
// config apply that failed for that reason would take the whole BGP
// configuration down with it. The next peer reconcile installs the route, and
// the line below names the peer so an operator can see which half of GTSM is
// missing meanwhile.
func applyHopLimitRoutes(wanted, previous []Peer) error {
	for _, p := range previous {
		if slices.ContainsFunc(wanted, func(w Peer) bool { return w.Addr == p.Addr }) {
			continue
		}
		if err := withdrawHopLimitRoute(p); err != nil {
			return err
		}
	}

	for _, p := range wanted {
		if p.HopLimit == 0 {
			continue
		}
		if err := installHopLimitRoute(p); err != nil {
			logger().Warn("GTSM hop-limit route not installed, ICMP errors to this peer carry the system default TTL",
				"peer", p.Addr.String(), "hop-limit", p.HopLimit, "error", err)
		}
	}
	return nil
}

// installHopLimitRoute writes the peer's host route, carrying the hop limit as
// its RTAX_HOPLIMIT metric.
//
// The nexthop is the one the kernel already resolves for the peer address, so
// the route changes which TTL a packet to the peer leaves with and nothing
// else about how it gets there. Asking the kernel is what makes that true for
// a multi-hop peer as well as a directly connected one.
func installHopLimitRoute(p Peer) error {
	resolved, err := resolveNextHop(p.Addr)
	if err != nil {
		return err
	}

	route := hostRoute(p.Addr)
	route.LinkIndex = resolved.LinkIndex
	route.Gw = resolved.Gw
	route.Hoplimit = int(p.HopLimit)
	// A route with no gateway is reachable over the link alone. Saying so
	// keeps this route's scope the same as the one the kernel resolved, so it
	// replaces nothing about reachability.
	route.Scope = netlink.SCOPE_UNIVERSE
	if resolved.Gw == nil {
		route.Scope = netlink.SCOPE_LINK
	}

	if err := routeAdd(&route); err != nil {
		return fmt.Errorf("replace host route to %s with hop limit %d: %w", p.Addr, p.HopLimit, err)
	}
	return nil
}

// withdrawHopLimitRoute removes a peer's host route.
//
// It deletes the route the kernel HOLDS rather than a route rebuilt from the
// peer: an IPv4 delete is matched on the scope as well as the destination, and
// a rebuilt route knows the scope only by re-deriving how the peer is reached.
// A route ze installed link-scoped then survived a delete that named the
// default scope, and the peer kept a metric its configuration no longer asked
// for.
//
// Only a route carrying ze's GTSM protocol is a candidate, so a host route for
// the same address that another producer installed is never removed. A missing
// route is not an error: the reconcile is reached after a restart that never
// installed one, and after an operator removed it by hand.
func withdrawHopLimitRoute(p Peer) error {
	installed, found, err := findHopLimitRoute(p.Addr)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if err := routeDel(&installed); err != nil {
		if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.ENOENT) {
			return nil
		}
		return fmt.Errorf("delete host route to %s: %w", p.Addr, err)
	}
	return nil
}

// findHopLimitRoute answers with ze's own GTSM host route to the address.
func findHopLimitRoute(addr netip.Addr) (netlink.Route, bool, error) {
	family := unix.AF_INET
	if addr.Is6() {
		family = unix.AF_INET6
	}
	routes, err := routeList(family)
	if err != nil {
		return netlink.Route{}, false, fmt.Errorf("list routes for %s: %w", addr, err)
	}

	ip := net.IP(addr.AsSlice())
	for i := range routes {
		if routes[i].Protocol != netlink.RouteProtocol(rtproto.GTSM) {
			continue
		}
		if routes[i].Dst == nil || !routes[i].Dst.IP.Equal(ip) {
			continue
		}
		return routes[i], true, nil
	}
	return netlink.Route{}, false, nil
}

// hostRoute is the route both directions agree on: this address, the whole
// address, in the main table, under ze's GTSM protocol.
func hostRoute(addr netip.Addr) netlink.Route {
	ip := net.IP(addr.AsSlice())
	bits := addr.BitLen()
	return netlink.Route{
		Dst:      &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)},
		Table:    unix.RT_TABLE_MAIN,
		Protocol: netlink.RouteProtocol(rtproto.GTSM),
	}
}

// hopLimitRouteInstalled reports whether the kernel holds ze's host route to
// the peer carrying the hop limit the peer asks for. A route ze installed
// without the metric, or a route another producer installed, answers false:
// neither gives a related ICMP error the hop limit RFC 5082 requires.
func hopLimitRouteInstalled(p Peer) (bool, error) {
	installed, found, err := findHopLimitRoute(p.Addr)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	return installed.Hoplimit == int(p.HopLimit), nil
}

// peerRouteResolvable reports whether the kernel resolves a route to the peer
// today, which is what installHopLimitRoute needs before it can install one.
func peerRouteResolvable(addr netip.Addr) error {
	_, err := resolveNextHop(addr)
	return err
}

// resolveNextHop asks the kernel which route it would use for the peer today.
func resolveNextHop(addr netip.Addr) (netlink.Route, error) {
	routes, err := routeResolve(net.IP(addr.AsSlice()))
	if err != nil {
		return netlink.Route{}, fmt.Errorf("resolve route to %s: %w", addr, err)
	}
	if len(routes) == 0 {
		return netlink.Route{}, fmt.Errorf("resolve route to %s: %w", addr, errNoRouteToPeer)
	}
	if routes[0].LinkIndex == 0 {
		return netlink.Route{}, fmt.Errorf("resolve route to %s: %w", addr, errRouteNamesNoInterface)
	}
	return routes[0], nil
}

var (
	errNoRouteToPeer         = errors.New("the kernel answered with no route")
	errRouteNamesNoInterface = errors.New("the kernel's route names no interface")
)
