// Design: plan/learned/650-static-routes.md -- Linux netlink backend with multipath
// Related: doctor.go -- pre-flight readiness check for interface-only next-hops

//go:build linux

package static

import (
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

func (b *netlinkStaticBackend) removeRoute(r staticRoute) error {
	route, err := b.buildRoute(r)
	if err != nil {
		return err
	}
	return b.handle.RouteDel(route)
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
func resolveNexthopIndex(name string) (int, error) {
	b, err := iface.Resolve(name)
	if err != nil {
		// Distinguish the no-backend case (the whole iface component is
		// absent because the config has no `interface { backend ... }`
		// stanza) from a device-absent case, so the operator sees an
		// actionable message instead of the bare "iface: no backend loaded"
		// (spec-fixit-static-interface-nexthops C-2).
		if iface.GetBackend() == nil {
			return 0, fmt.Errorf("interface %q: no interface backend loaded; add an `interface { backend ... }` stanza so static next-hop interfaces can be resolved: %w", name, err)
		}
		return 0, fmt.Errorf("interface %q: %w", name, err)
	}
	return b.Ifindex, nil
}

func (b *netlinkStaticBackend) close() error {
	if b.handle != nil {
		b.handle.Close()
	}
	return nil
}

func (b *netlinkStaticBackend) buildRoute(r staticRoute) (*netlink.Route, error) {
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
		return nil, fmt.Errorf("unknown action %d", r.Action)
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
