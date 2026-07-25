// Design: plan/spec-fib-depth.md -- Linux netlink rich route programming
// Related: richroute.go -- RichRoute struct and richRouteBackend interface
// Related: backend_linux.go -- base netlinkBackend

//go:build linux

package fibkernel

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/component/sysrib/events"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func (n *netlinkBackend) addRichRoute(r RichRoute) error {
	route, err := buildRichRoute(r)
	if err != nil {
		return err
	}
	return n.handle.RouteAdd(route)
}

func (n *netlinkBackend) delRichRoute(prefix netip.Prefix, tableID uint32) error {
	_, cidr, err := net.ParseCIDR(prefix.String())
	if err != nil {
		return fmt.Errorf("parse prefix %v: %w", prefix, err)
	}
	route := &netlink.Route{
		Dst:      cidr,
		Protocol: rtprotZE,
	}
	if tableID != 0 {
		route.Table = int(tableID)
	}
	return n.handle.RouteDel(route)
}

func (n *netlinkBackend) replaceRichRoute(r RichRoute) error {
	route, err := buildRichRoute(r)
	if err != nil {
		return err
	}
	return n.handle.RouteReplace(route)
}

func buildRichRoute(r RichRoute) (*netlink.Route, error) {
	_, cidr, err := net.ParseCIDR(r.Prefix.String())
	if err != nil {
		return nil, fmt.Errorf("parse prefix %v: %w", r.Prefix, err)
	}

	route := &netlink.Route{
		Dst:      cidr,
		Protocol: rtprotZE,
		Priority: int(r.Metric),
	}

	if r.TableID != 0 {
		route.Table = int(r.TableID)
	}

	route.Type = routeTypeToLinux(r.RouteType)

	if len(r.ECMPPaths) > 0 {
		route.MultiPath = buildMultiPath(r.NextHop, r.ECMPPaths)
	} else if r.NextHop.IsValid() {
		route.Gw = r.NextHop.AsSlice()
	}

	if len(r.Labels) > 0 && route.Type == unix.RTN_UNICAST {
		route.Encap = buildMPLSEncap(r.Labels)
	} else if r.SRv6SID.IsValid() && r.SRv6SID.Is6() && route.Type == unix.RTN_UNICAST {
		route.Encap = buildSEG6Encap(r.SRv6SID)
	}

	// Fast-reroute backup (RFC 5286 / TI-LFA): program the backup next-hop(s) as
	// link-down-flagged multipath next-hops carrying the repair MPLS encap, so the
	// kernel forwards to a backup only when the primary link is down. A backup
	// requires a multipath route to hold both primary and backup, so a single-path
	// route is promoted to multipath first (its primary label stack moves onto the
	// primary next-hop).
	if len(r.Backup) > 0 {
		if route.MultiPath == nil {
			route.MultiPath = buildMultiPath(r.NextHop, nil)
			if route.Encap != nil && len(route.MultiPath) > 0 {
				route.MultiPath[0].Encap = route.Encap
				route.Encap = nil
			}
			route.Gw = nil
		}
		route.MultiPath = append(route.MultiPath, buildBackupNexthops(r.Backup)...)
	}

	return route, nil
}

// buildBackupNexthops builds the link-down/backup multipath next-hops for a
// fast-reroute backup: each carries the RTNH_F_LINKDOWN flag (used only when the
// primary link is down) and, for a TI-LFA repair, the SR repair MPLS encap.
func buildBackupNexthops(backup []events.ECMPPath) []*netlink.NexthopInfo {
	out := make([]*netlink.NexthopInfo, 0, len(backup))
	for _, b := range backup {
		if !b.NextHop.IsValid() {
			continue
		}
		nhi := &netlink.NexthopInfo{
			Gw:    b.NextHop.AsSlice(),
			Flags: int(unix.RTNH_F_LINKDOWN),
		}
		if len(b.Labels) > 0 {
			nhi.Encap = buildMPLSEncap(b.Labels)
		}
		out = append(out, nhi)
	}
	return out
}

func routeTypeToLinux(rt events.RouteType) int {
	switch rt {
	case events.RouteTypeBlackhole:
		return unix.RTN_BLACKHOLE
	case events.RouteTypeUnreachable:
		return unix.RTN_UNREACHABLE
	case events.RouteTypeProhibit:
		return unix.RTN_PROHIBIT
	default:
		return unix.RTN_UNICAST
	}
}

func buildMultiPath(primary netip.Addr, ecmpPaths []events.ECMPPath) []*netlink.NexthopInfo {
	paths := make([]*netlink.NexthopInfo, 0, len(ecmpPaths)+1)
	if primary.IsValid() {
		paths = append(paths, &netlink.NexthopInfo{
			Hops: 0,
			Gw:   primary.AsSlice(),
		})
	}
	for _, p := range ecmpPaths {
		if !p.NextHop.IsValid() {
			continue
		}
		nhi := &netlink.NexthopInfo{
			Gw: p.NextHop.AsSlice(),
		}
		if p.Weight > 1 {
			nhi.Hops = int(p.Weight) - 1
		}
		paths = append(paths, nhi)
	}
	return paths
}

func buildMPLSEncap(labels []uint32) *netlink.MPLSEncap {
	intLabels := make([]int, len(labels))
	for i, l := range labels {
		intLabels[i] = int(l)
	}
	return &netlink.MPLSEncap{Labels: intLabels}
}

const seg6IptunModeEncap = 1

func buildSEG6Encap(sid netip.Addr) *netlink.SEG6Encap {
	ip6 := sid.As16()
	return &netlink.SEG6Encap{
		Mode:     seg6IptunModeEncap,
		Segments: []net.IP{net.IP(ip6[:])},
	}
}
