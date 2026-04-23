//go:build linux

// Design: plan/spec-diag-5-active-probes.md -- LPM route lookup via netlink

package iface

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
)

// RouteLookup performs a longest-prefix-match lookup for the given
// destination IP using the kernel's netlink RouteGet. Returns the
// matching route as a map suitable for JSON serialization.
func RouteLookup(dest netip.Addr) (map[string]any, error) {
	routes, err := netlink.RouteGet(net.IP(dest.AsSlice()))
	if err != nil {
		return nil, fmt.Errorf("route lookup: %w", err)
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("route lookup: no route to %s", dest)
	}
	r := routes[0]

	result := map[string]any{
		"destination": dest.String(),
	}
	if r.Dst != nil {
		prefix, ok := netip.AddrFromSlice(r.Dst.IP)
		if ok {
			ones, _ := r.Dst.Mask.Size()
			result["prefix"] = netip.PrefixFrom(prefix, ones).String()
		}
	} else {
		if dest.Is4() {
			result["prefix"] = "0.0.0.0/0"
		} else {
			result["prefix"] = "::/0"
		}
	}
	if r.Gw != nil {
		gw, ok := netip.AddrFromSlice(r.Gw)
		if ok {
			result["next-hop"] = gw.String()
		}
	}
	if r.LinkIndex > 0 {
		link, linkErr := netlink.LinkByIndex(r.LinkIndex)
		if linkErr == nil {
			result["interface"] = link.Attrs().Name
		}
	}
	result["protocol"] = int(r.Protocol)
	result["metric"] = r.Priority
	result["table"] = r.Table

	return result, nil
}
