// Design: plan/immediate/spec-fib-depth.md -- Linux netlink rich route programming
// Related: richroute.go -- RichRoute struct and richRouteBackend interface
// Related: backend_linux.go -- base netlinkBackend

//go:build linux

package fibkernel

import (
	"fmt"
	"math"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/sysrib/events"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// maxNetlinkInt is the largest value this build can carry in one of the netlink
// bindings' int-typed route fields (netlink.Route.Table, .Priority). On a
// 32-bit build a uint32 above MaxInt32 turns negative there and the encoder
// drops the attribute silently; the bound turns that into an error. On the
// 64-bit targets Ze ships it is above every uint32 and never bites.
const maxNetlinkInt = uint64(math.MaxInt)

func (n *netlinkBackend) addRichRoute(r RichRoute) error {
	route, err := buildRichRoute(r)
	if err != nil {
		return err
	}
	// The kernel refuses a label the table has no room for, and its table is
	// empty by default. Repair that before the push route goes in, not after
	// (labelspace_linux.go).
	if needsLabelSpace(r) {
		ensureLabelSpace()
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
	// The same repair as addRichRoute: a replace is the first programming of a
	// label just as often as an add is (labelspace_linux.go).
	if needsLabelSpace(r) {
		ensureLabelSpace()
	}
	return n.replaceOwnedRoute(route)
}

func needsLabelSpace(r RichRoute) bool {
	if len(r.Labels) > 0 || len(r.Backup) > 0 {
		return true
	}
	for i := range r.ECMPPaths {
		if len(r.ECMPPaths[i].Labels) > 0 {
			return true
		}
	}
	return false
}

// buildRichRoute translates a RichRoute into the netlink form.
//
// Metric and TableID are bounded before conversion because netlink.Route
// carries both in a Go int and the encoder emits RTA_PRIORITY / RTA_TABLE only
// for positive values (vendor/github.com/vishvananda/netlink/
// route_linux.go:1058,1069). A value that does not survive the conversion would
// otherwise be dropped without an error, installing the route in RT_TABLE_MAIN
// at the kernel default metric instead of where the routing protocol put it.
// On the 64-bit targets Ze ships the bound is above every uint32 and never bites.
func buildRichRoute(r RichRoute) (*netlink.Route, error) {
	if uint64(r.Metric) > maxNetlinkInt {
		return nil, fmt.Errorf("metric %d exceeds %d, the largest this build can program through netlink", r.Metric, maxNetlinkInt)
	}
	if uint64(r.TableID) > maxNetlinkInt {
		return nil, fmt.Errorf("table %d exceeds %d, the largest this build can program through netlink", r.TableID, maxNetlinkInt)
	}
	if uint64(r.PathMTU) > maxNetlinkInt {
		return nil, fmt.Errorf("path MTU %d exceeds the netlink integer range", r.PathMTU)
	}

	_, cidr, err := net.ParseCIDR(r.Prefix.String())
	if err != nil {
		return nil, fmt.Errorf("parse prefix %v: %w", r.Prefix, err)
	}

	route := &netlink.Route{
		Dst:      cidr,
		Protocol: rtprotZE,
		Priority: int(r.Metric),
		MTU:      int(r.PathMTU),
	}

	if r.TableID != 0 {
		route.Table = int(r.TableID)
	}

	route.Type = routeTypeToLinux(r.RouteType)

	// A discarding route answers a packet itself, so it names no next-hop. Linux
	// refuses one that does: fib_create_info rejects RTN_BLACKHOLE,
	// RTN_UNREACHABLE and RTN_PROHIBIT carrying RTA_GATEWAY, RTA_OIF or
	// RTA_MULTIPATH with EINVAL ("Gateway, device and multipath can not be
	// specified for this route type"). A BGP path always resolves a next-hop, so
	// without this the whole discard route is rejected and nothing is programmed.
	// Encap is dropped for the same reason: it describes how to reach a next-hop
	// this route does not have.
	if r.RouteType.Discards() {
		return route, nil
	}

	if len(r.ECMPPaths) > 0 {
		route.MultiPath, err = buildMultiPath(r, r.ECMPPaths)
		if err != nil {
			return nil, err
		}
	} else {
		route.Gw, route.Via, err = buildGateway(r.Prefix, r.NextHop)
		if err != nil {
			return nil, err
		}
		if r.OnLink {
			route.Flags |= unix.RTNH_F_ONLINK
		}
		// A route may name an outgoing device instead of, or beside, a gateway.
		// Without the index the kernel gets a route with no next-hop at all and
		// refuses it, which is how an interface-only route disappears.
		if r.Interface != "" {
			idx, ifErr := iface.ResolveIndex(r.Interface)
			if ifErr != nil {
				return nil, ifErr
			}
			route.LinkIndex = idx
		}
	}

	// An ECMP member owns its encapsulation. A route-level stack would also
	// apply to a member whose producer supplied a different stack or plain IP.
	if len(route.MultiPath) == 0 && len(r.Backup) == 0 {
		route.Encap = buildRouteEncap(r.Labels, r.SRv6SID)
	}

	// Fast-reroute backup (RFC 5286 / TI-LFA): program the backup next-hop(s) as
	// link-down-flagged multipath next-hops carrying the repair MPLS encap, so the
	// kernel forwards to a backup only when the primary link is down. A backup
	// requires a multipath route to hold both primary and backup, so a single-path
	// route is promoted to multipath first (its primary label stack moves onto the
	// primary next-hop).
	if len(r.Backup) > 0 {
		if route.MultiPath == nil {
			route.MultiPath, err = buildMultiPath(r, nil)
			if err != nil {
				return nil, err
			}
			route.Gw = nil
			route.Via = nil
			route.Flags &^= unix.RTNH_F_ONLINK
		}
		backup, backupErr := buildBackupNexthops(r.Prefix, r.Backup)
		if backupErr != nil {
			return nil, backupErr
		}
		route.MultiPath = append(route.MultiPath, backup...)
	}

	return route, nil
}

// buildBackupNexthops builds the link-down/backup multipath next-hops for a
// fast-reroute backup: each carries the RTNH_F_LINKDOWN flag (used only when the
// primary link is down) and, for a TI-LFA repair, the SR repair MPLS encap.
func buildBackupNexthops(prefix netip.Prefix, backup []events.ECMPPath) ([]*netlink.NexthopInfo, error) {
	out := make([]*netlink.NexthopInfo, 0, len(backup))
	for _, b := range backup {
		nhi, err := buildNexthopInfo(prefix, b.NextHop, b.Interface, b.Weight, b.OnLink)
		if err != nil {
			return nil, err
		}
		nhi.Flags |= unix.RTNH_F_LINKDOWN
		if len(b.Labels) > 0 {
			nhi.Encap = buildMPLSEncap(b.Labels)
		}
		out = append(out, nhi)
	}
	return out, nil
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

// buildMultiPath builds the RTA_MULTIPATH next-hop list: the route's own
// next-hop first, then every equal-cost member. Each entry carries its gateway,
// its outgoing device where one is named, and its share of the group.
//
// The kernel expresses a share as rtnh_hops, which is the weight MINUS ONE, so
// an unweighted member (weight 0 or 1) carries 0 and every member of an
// unweighted group gets the same traffic.
func buildMultiPath(r RichRoute, ecmpPaths []events.ECMPPath) ([]*netlink.NexthopInfo, error) {
	paths := make([]*netlink.NexthopInfo, 0, len(ecmpPaths)+1)
	if namesATarget(r.NextHop, r.Interface) {
		primary, err := buildNexthopInfo(r.Prefix, r.NextHop, r.Interface, r.Weight, r.OnLink)
		if err != nil {
			return nil, err
		}
		primary.Encap = buildRouteEncap(r.Labels, r.SRv6SID)
		paths = append(paths, primary)
	}
	for _, p := range ecmpPaths {
		nhi, err := buildNexthopInfo(r.Prefix, p.NextHop, p.Interface, p.Weight, p.OnLink)
		if err != nil {
			return nil, err
		}
		nhi.Encap = buildRouteEncap(p.Labels, r.SRv6SID)
		paths = append(paths, nhi)
	}
	return paths, nil
}

// namesATarget reports whether a next-hop names somewhere to forward to: a
// gateway address, an outgoing device, or both. A member that names neither is
// not a next-hop and never enters a multipath list.
func namesATarget(addr netip.Addr, ifaceName string) bool {
	return addr.IsValid() || ifaceName != ""
}

// buildNexthopInfo renders one multipath member.
//
// A member without a gateway or device rejects the whole route, because
// silently removing it would change the producer's forwarding decision.
func buildNexthopInfo(prefix netip.Prefix, addr netip.Addr, ifaceName string, weight uint8, onLink bool) (*netlink.NexthopInfo, error) {
	if !namesATarget(addr, ifaceName) {
		return nil, fmt.Errorf("next hop for %s has no gateway or device", prefix)
	}
	gw, via, err := buildGateway(prefix, addr)
	if err != nil {
		return nil, err
	}
	nhi := &netlink.NexthopInfo{Gw: gw, Via: via}
	if onLink {
		nhi.Flags |= unix.RTNH_F_ONLINK
	}
	if ifaceName != "" {
		idx, err := iface.ResolveIndex(ifaceName)
		if err != nil {
			return nil, err
		}
		nhi.LinkIndex = idx
	}
	if weight > 1 {
		nhi.Hops = int(weight) - 1
	}
	return nhi, nil
}

// buildGateway selects the gateway attribute for the destination family.
// Linux supports IPv4 forwarding through IPv6 gateways with RTA_VIA.
func buildGateway(prefix netip.Prefix, addr netip.Addr) (net.IP, netlink.Destination, error) {
	if !addr.IsValid() {
		return nil, nil, nil
	}
	addr = addr.Unmap()
	if prefix.Addr().Unmap().Is4() == addr.Is4() {
		return addr.AsSlice(), nil, nil
	}
	if addr.Is6() {
		return nil, &netlink.Via{AddrFamily: unix.AF_INET6, Addr: addr.AsSlice()}, nil
	}
	return nil, nil, fmt.Errorf("IPv6 route %s cannot use IPv4 gateway %s", prefix, addr)
}

// buildRouteEncap gives the Service SID precedence over NLRI label fields,
// which can hold SID transposition bits (RFC 9252 Section 5; rfc/short/rfc9252.md).
func buildRouteEncap(labels []uint32, sid netip.Addr) netlink.Encap {
	if sid.Is6() {
		return buildSEG6Encap(sid)
	}
	if len(labels) > 0 {
		return buildMPLSEncap(labels)
	}
	return nil
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
