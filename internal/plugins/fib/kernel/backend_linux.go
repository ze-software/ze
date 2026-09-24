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
	"net/netip"

	"github.com/ze-software/ze/internal/core/rtproto"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// rtprotZE is the custom rtm_protocol ID used for FIB-kernel routes.
// Linux: identifies routes in the kernel routing table as belonging to this producer.
// RFC 3549 Section 3.1.1: protocol field in rtmsg.
const rtprotZE = rtproto.FIBKernel

// netlinkBackend programs routes via Linux netlink.
type netlinkBackend struct {
	handle   *netlink.Handle
	contexts mplsContextState
}

func newBackend() routeBackend {
	h, err := netlink.NewHandle()
	if err != nil {
		logger().Error("fib-kernel: netlink handle failed, route programming disabled", "error", err)
		return &failedBackend{err: fmt.Errorf("netlink unavailable: %w", err)}
	}
	backend := &netlinkBackend{handle: h}
	for _, family := range []int{netlink.FAMILY_V4, netlink.FAMILY_V6} {
		if err := backend.ensureMPLSGuard(family); err != nil {
			logger().Error("fib-kernel: private MPLS guard unavailable", "family", family, "error", err)
		}
	}
	return backend
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
	return n.replaceOwnedRoute(route)
}

// replaceOwnedRoute never treats RTM_NEWROUTE's protocol field as an ownership
// filter: Linux can replace another protocol's route at the same priority.
// Check the current slot, then use an exclusive add for an empty slot. A metric
// change creates a different slot, so remove superseded Ze entries only after
// the new route is installed; unrelated protocols and tables stay untouched.
func (n *netlinkBackend) replaceOwnedRoute(route *netlink.Route) error {
	table := route.Table
	if table == 0 {
		table = unix.RT_TABLE_MAIN
	}
	family := netlink.FAMILY_V6
	priority := route.Priority
	if route.Dst.IP.To4() != nil {
		family = netlink.FAMILY_V4
	} else if priority == 0 {
		// IPv6 gives an omitted metric the user-route default, unlike IPv4.
		priority = 1024
	}
	current, err := n.handle.RouteListFiltered(family, &netlink.Route{Dst: route.Dst, Table: table, Tos: route.Tos},
		netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE|netlink.RT_FILTER_TOS)
	if err != nil {
		return fmt.Errorf("route ownership lookup %s: %w", route.Dst, err)
	}
	replace := false
	for i := range current {
		if current[i].Priority != priority {
			continue
		}
		if current[i].Protocol != rtprotZE {
			return fmt.Errorf("route %s table %d priority %d belongs to protocol %d: %w",
				route.Dst, table, priority, current[i].Protocol, unix.EEXIST)
		}
		replace = true
	}
	if replace {
		err = n.handle.RouteReplace(route)
	} else {
		err = n.handle.RouteAdd(route)
	}
	if err != nil {
		return err
	}
	for i := range current {
		if current[i].Protocol == rtprotZE && current[i].Priority != priority {
			if err := n.handle.RouteDel(&current[i]); err != nil {
				return fmt.Errorf("remove superseded route %s priority %d: %w", route.Dst, current[i].Priority, err)
			}
		}
	}
	return nil
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

// close removes private routes/selectors even when ordinary routes survive
// graceful restart. Fixed namespace guards remain for late marked packets.
func (n *netlinkBackend) close() error {
	n.contexts.mu.Lock()
	defer n.contexts.mu.Unlock()
	if n.contexts.closed {
		return nil
	}
	n.contexts.closed = true
	err := n.closeMPLSContexts()
	if n.handle != nil {
		n.handle.Close()
	}
	return err
}

// buildRoute creates a netlink.Route from prefix and next-hop strings.
func buildRoute(prefix, nextHop string) (*netlink.Route, error) {
	_, cidr, err := net.ParseCIDR(prefix)
	if err != nil {
		return nil, fmt.Errorf("parse prefix %q: %w", prefix, err)
	}

	gateway, err := netip.ParseAddr(nextHop)
	if err != nil {
		return nil, fmt.Errorf("parse next-hop %q: %w", nextHop, err)
	}
	destination, _ := netip.AddrFromSlice(cidr.IP)
	bits, _ := cidr.Mask.Size()
	gw, via, err := buildGateway(netip.PrefixFrom(destination, bits), gateway)
	if err != nil {
		return nil, err
	}

	return &netlink.Route{
		Dst:      cidr,
		Gw:       gw,
		Via:      via,
		Protocol: rtprotZE,
	}, nil
}
