// Design: docs/architecture/core-design.md -- FIB Linux netlink backend
// Overview: fibkernel.go -- FIB kernel plugin
// Related: backend.go -- backend abstraction and shared helpers
// Related: backend_other.go -- noop backend for non-Linux
//
// Linux route programming via netlink. All ze-installed routes use
// rtm_protocol=RTPROT_ZE (250) so they can be identified for crash
// recovery (stale-mark-then-sweep) and distinguished from external changes.

//go:build linux

package fibkernel

import (
	"fmt"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/core/rtproto"

	"github.com/vishvananda/netlink"
)

// rtprotZE is the custom rtm_protocol ID used for FIB-kernel routes.
// Linux: identifies routes in the kernel routing table as belonging to this producer.
// RFC 3549 Section 3.1.1: protocol field in rtmsg.
const rtprotZE = rtproto.FIBKernel

// netlinkBackend programs routes via Linux netlink.
type netlinkBackend struct {
	handle *netlink.Handle
}

func newBackend() routeBackend {
	h, err := netlink.NewHandle()
	if err != nil {
		logger().Error("fib-kernel: netlink handle failed, route programming disabled", "error", err)
		return &failedBackend{err: fmt.Errorf("netlink unavailable: %w", err)}
	}
	return &netlinkBackend{handle: h}
}

// failedBackend returns errors for all operations when netlink init failed.
type failedBackend struct{ err error }

func (f *failedBackend) addRoute(_, _ string) error              { return f.err }
func (f *failedBackend) delRoute(_ string) error                 { return f.err }
func (f *failedBackend) replaceRoute(_, _ string) error          { return f.err }
func (f *failedBackend) listZeRoutes() ([]installedRoute, error) { return nil, f.err }
func (f *failedBackend) close() error                            { return nil }

func (n *netlinkBackend) addRoute(prefix, nextHop string) error {
	route, err := buildRoute(prefix, nextHop)
	if err != nil {
		return err
	}
	return n.handle.RouteAdd(route)
}

func (n *netlinkBackend) delRoute(prefix string) error {
	_, cidr, err := net.ParseCIDR(prefix)
	if err != nil {
		return fmt.Errorf("parse prefix %q: %w", prefix, err)
	}
	route := &netlink.Route{
		Dst:      cidr,
		Protocol: rtprotZE,
	}
	return n.handle.RouteDel(route)
}

func (n *netlinkBackend) replaceRoute(prefix, nextHop string) error {
	route, err := buildRoute(prefix, nextHop)
	if err != nil {
		return err
	}
	return n.handle.RouteReplace(route)
}

func (n *netlinkBackend) listZeRoutes() ([]installedRoute, error) {
	routes, err := n.handle.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("route list: %w", err)
	}

	var result []installedRoute
	for i := range routes {
		if routes[i].Protocol != rtprotZE {
			continue
		}
		if routes[i].Dst == nil {
			continue
		}
		ir := installedRoute{prefix: routes[i].Dst.String()}
		if routes[i].Gw != nil {
			ir.nextHop = routes[i].Gw.String()
		}
		result = append(result, ir)
	}
	return result, nil
}

func (n *netlinkBackend) close() error {
	if n.handle != nil {
		n.handle.Close()
	}
	return nil
}

// buildRoute creates a netlink.Route from prefix and next-hop strings.
func buildRoute(prefix, nextHop string) (*netlink.Route, error) {
	_, cidr, err := net.ParseCIDR(prefix)
	if err != nil {
		return nil, fmt.Errorf("parse prefix %q: %w", prefix, err)
	}

	gw := net.ParseIP(nextHop)
	if gw == nil {
		return nil, fmt.Errorf("parse next-hop %q: invalid IP", nextHop)
	}

	return &netlink.Route{
		Dst:      cidr,
		Gw:       gw,
		Protocol: rtprotZE,
	}, nil
}
