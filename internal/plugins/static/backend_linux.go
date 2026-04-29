// Design: plan/spec-static-routes.md -- Linux netlink backend with multipath

//go:build linux

package static

import (
	"fmt"
	"net"
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/rtproto"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const rtprotStatic = rtproto.Static

type netlinkStaticBackend struct {
	handle *netlink.Handle
}

func newStaticBackend() routeBackend {
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
		ir := installedStaticRoute{prefix: routes[i].Dst.String()}
		if routes[i].Gw != nil {
			ir.nextHop = routes[i].Gw.String()
		}
		result = append(result, ir)
	}
	return result, nil
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
		route.Gw = nh.Address.AsSlice()
		if nh.Interface != "" {
			link, err := b.handle.LinkByName(nh.Interface)
			if err != nil {
				return nil, fmt.Errorf("interface %q: %w", nh.Interface, err)
			}
			route.LinkIndex = link.Attrs().Index
		}
		return route, nil
	}

	var multipath []*netlink.NexthopInfo
	for _, nh := range r.NextHops {
		nhi := &netlink.NexthopInfo{
			Gw:   nh.Address.AsSlice(),
			Hops: int(nh.Weight) - 1,
		}
		if nh.Interface != "" {
			link, err := b.handle.LinkByName(nh.Interface)
			if err != nil {
				return nil, fmt.Errorf("interface %q: %w", nh.Interface, err)
			}
			nhi.LinkIndex = link.Attrs().Index
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
