// Design: docs/architecture/static-routes.md -- Linux netlink backend with multipath
// Related: doctor.go -- pre-flight readiness check for interface-only next-hops

//go:build linux

package static

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const rtprotStatic = rtproto.Static

type netlinkStaticBackend struct {
	handle *netlink.Handle
}

func newStaticBackend() routeBackend {
	// In a VPP data-plane deployment (VPP component active) static routes are
	// programmed into VPP's FIB; otherwise they go to the kernel via netlink.
	if vb := newVPPStaticBackend(); vb != nil {
		return vb
	}
	h, err := netlink.NewHandle()
	if err != nil {
		logger().Error("static: netlink handle failed", "error", err)
		return &failedStaticBackend{err: fmt.Errorf("netlink unavailable: %w", err)}
	}
	return &netlinkStaticBackend{handle: h}
}

type failedStaticBackend struct{ err error }

func (f *failedStaticBackend) applyRoute(_ staticRoute) error              { return f.err }
func (f *failedStaticBackend) removeRoute(_ staticRoute) error             { return f.err }
func (f *failedStaticBackend) listRoutes() ([]installedStaticRoute, error) { return nil, f.err }
func (f *failedStaticBackend) close() error                                { return nil }

func (b *netlinkStaticBackend) applyRoute(r staticRoute) error {
	route, err := b.buildRoute(r)
	if err != nil {
		return err
	}
	return b.handle.RouteReplace(route)
}

// removeRoute deletes the route static installed for (table, prefix) at its
// metric, and names no next-hop. Static is the one writer of a named table and
// holds one route per (table, prefix), so the key is exact; naming the members
// would resolve each interface first, and a member whose device was deleted
// (a pppoe-client session that dropped) no longer resolves, which turned the
// removal of a route the kernel had already flushed into a failure.
//
// ESRCH is the kernel's answer for a route that is not there: when a device is
// deleted the kernel flushes every route through it, so the removal that
// follows finds nothing. The route is gone, which is what the removal asked
// for, so it is done (the precedent is the mpls context route removal in
// internal/plugins/fib/kernel/mplscontext_linux.go).
func (b *netlinkStaticBackend) removeRoute(r staticRoute) error {
	key := r
	key.NextHops = nil
	route, err := b.buildRoute(key)
	if err != nil {
		return err
	}
	if err := b.handle.RouteDel(route); err != nil && !errors.Is(err, unix.ESRCH) {
		return fmt.Errorf("route %s table %d remove: %w", r.Prefix, r.Table, err)
	}
	return nil
}

func (b *netlinkStaticBackend) listRoutes() ([]installedStaticRoute, error) {
	routes, err := b.handle.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("route list: %w", err)
	}

	var result []installedStaticRoute
	for i := range routes {
		if routes[i].Protocol != rtprotStatic {
			continue
		}
		if routes[i].Dst == nil {
			continue
		}
		tbl := uint32(routes[i].Table)
		if tbl == 254 {
			tbl = 0 // RT_TABLE_MAIN -> our internal representation
		}
		ir := installedStaticRoute{
			prefix: routes[i].Dst.String(),
			table:  tbl,
		}
		if routes[i].Gw != nil {
			ir.nextHop = routes[i].Gw.String()
		}
		result = append(result, ir)
	}
	return result, nil
}

// resolveNexthopIndex maps a logical nexthop interface name to its kernel
// ifindex through the shared iface resolver, so a static route's nexthop honors
// the os-name / mac-match selectors instead of assuming name == kernel device.
// For the common case (name == kernel device) the resolver returns the same
// index a direct LinkByName would, so existing configs are unaffected.
//
// The two failure messages an operator needs live in iface.ResolveIndex, which
// the FIB plugin calls for the same job on the main-table path
// (spec-fixit-static-interface-nexthops C-2).
func resolveNexthopIndex(name string) (int, error) {
	return iface.ResolveIndex(name)
}

func (b *netlinkStaticBackend) close() error {
	if b.handle != nil {
		b.handle.Close()
	}
	return nil
}

// buildRoute translates a staticRoute into the netlink form.
//
// The Table and Metric bounds are not redundant with config validation:
// netlink.Route.Table and .Priority are Go ints, and the encoder emits the
// attribute only for positive values (route_linux.go:1058,1069), so a value
// that does not survive the conversion is silently dropped rather than
// rejected. Refuse to build such a route instead of installing it in the wrong
// table or at the wrong metric. See maxNetlinkInt in config.go.
func (b *netlinkStaticBackend) buildRoute(r staticRoute) (*netlink.Route, error) {
	if uint64(r.Table) > maxNetlinkInt {
		return nil, fmt.Errorf("table %d exceeds %d, the largest this build can program through netlink", r.Table, maxNetlinkInt)
	}
	if err := validateRouteMetric(r.Metric, maxNetlinkInt); err != nil {
		return nil, err
	}

	dst := prefixToIPNet(r.Prefix)

	route := &netlink.Route{
		Dst:      dst,
		Protocol: rtprotStatic,
		Priority: int(r.Metric),
		Table:    int(r.Table),
	}

	switch r.Action {
	case actionBlackhole:
		route.Type = unix.RTN_BLACKHOLE
		return route, nil
	case actionReject:
		route.Type = unix.RTN_UNREACHABLE
		return route, nil
	case actionForward:
		// handled below
	default:
		panic("BUG: static: invalid route action")
	}

	if len(r.NextHops) == 1 {
		nh := r.NextHops[0]
		if nh.Address.IsValid() {
			route.Gw = nh.Address.AsSlice()
		}
		if nh.Interface != "" {
			idx, err := resolveNexthopIndex(nh.Interface)
			if err != nil {
				return nil, err
			}
			route.LinkIndex = idx
		}
		return route, nil
	}

	var multipath []*netlink.NexthopInfo
	for _, nh := range r.NextHops {
		nhi := &netlink.NexthopInfo{
			Hops: int(nh.Weight) - 1,
		}
		if nh.Address.IsValid() {
			nhi.Gw = nh.Address.AsSlice()
		}
		if nh.Interface != "" {
			idx, err := resolveNexthopIndex(nh.Interface)
			if err != nil {
				return nil, err
			}
			nhi.LinkIndex = idx
		}
		multipath = append(multipath, nhi)
	}
	route.MultiPath = multipath

	return route, nil
}

func prefixToIPNet(p netip.Prefix) *net.IPNet {
	addr := p.Masked().Addr()
	bits := p.Bits()
	if addr.Is4() {
		ip := addr.As4()
		return &net.IPNet{
			IP:   net.IP(ip[:]),
			Mask: net.CIDRMask(bits, 32),
		}
	}
	ip := addr.As16()
	return &net.IPNet{
		IP:   net.IP(ip[:]),
		Mask: net.CIDRMask(bits, 128),
	}
}
