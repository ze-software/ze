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
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// rtprotZE is the custom rtm_protocol ID used for FIB-kernel routes.
// Linux: identifies routes in the kernel routing table as belonging to this producer.
// RFC 3549 Section 3.1.1: protocol field in rtmsg.
const rtprotZE = rtproto.FIBKernel

// netlinkHandle is the subset of *netlink.Handle used by netlinkBackend.
// *netlink.Handle satisfies it directly; unit tests substitute a fake to drive
// kernel errnos (such as EOPNOTSUPP on a kernel without CONFIG_IPV6_SEG6_LWTUNNEL)
// without privilege.
type netlinkHandle interface {
	replayRouteOperations
	RouteAdd(route *netlink.Route) error
	RouteReplace(route *netlink.Route) error
	RouteDel(route *netlink.Route) error
	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
	RouteListFiltered(family int, filter *netlink.Route, filterMask uint64) ([]netlink.Route, error)
	RouteGetWithOptions(destination net.IP, options *netlink.RouteGetOptions) ([]netlink.Route, error)
	LinkByName(name string) (netlink.Link, error)
	LinkByIndex(index int) (netlink.Link, error)
	RuleList(family int) ([]netlink.Rule, error)
	RuleAdd(rule *netlink.Rule) error
	RuleDel(rule *netlink.Rule) error
	Close()
}

var _ netlinkHandle = (*netlink.Handle)(nil)

// netlinkBackend programs routes via Linux netlink.
type netlinkBackend struct {
	handle   netlinkHandle
	contexts mplsContextState
	// transitMTU is the kernel's answer to whether an AF_MPLS route may carry
	// a path MTU. Guarded by contexts.mu; Unspecified until the first transit
	// route with a path MTU asks (mplsentry_linux.go, transitRouteMTU).
	transitMTU kernelcap.State
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
func (f *failedBackend) delRoute(_ string) (bool, error)         { return false, f.err }
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

func (n *netlinkBackend) delRoute(prefix string) (bool, error) {
	route, err := ownedRouteKey(prefix, 0)
	if err != nil {
		return false, err
	}
	return delOwnedRoute(n.handle, route)
}

// ownedRouteKey names the route Ze installed for a prefix in a table, in the
// form an RTM_DELROUTE matches whatever scope the route was installed with.
//
// The scope is the reason this is not a bare {Dst, Protocol}. The bindings copy
// Route.Scope into rtm_scope, so a zero Route asks for RT_SCOPE_UNIVERSE, and
// fib_table_delete (net/ipv4/fib_trie.c) matches an alias only when
//
//	(cfg->fc_scope == RT_SCOPE_NOWHERE ||
//	 fa->fa_info->fib_scope == cfg->fc_scope)
//
// buildRichRoute installs an IPv4 device-only route at RT_SCOPE_LINK
// (forwardingScope), so a universe-scope delete misses it, the kernel answers
// ESRCH, and delOwnedRoute reads that as "already gone": the route stays. The
// protocol keeps the delete to Ze's own route, the table to the one it was
// installed in. IPv6 ignores rtm_scope on delete, so the same key serves both.
func ownedRouteKey(prefix string, tableID uint32) (*netlink.Route, error) {
	_, cidr, err := net.ParseCIDR(prefix)
	if err != nil {
		return nil, fmt.Errorf("parse prefix %q: %w", prefix, err)
	}
	if uint64(tableID) > maxNetlinkInt {
		return nil, fmt.Errorf("table %d exceeds %d, the largest this build can program through netlink", tableID, maxNetlinkInt)
	}
	route := &netlink.Route{
		Dst:      cidr,
		Protocol: rtprotZE,
		Scope:    netlink.SCOPE_NOWHERE,
	}
	if tableID != 0 {
		route.Table = int(tableID)
	}
	return route, nil
}

// delOwnedRoute deletes a route Ze installed and reports whether the kernel
// actually removed a route. The route MUST come from ownedRouteKey: only a key
// that matches every scope makes ESRCH mean absence. ESRCH is the kernel's
// answer for a route that is not there (for example, when a device is deleted
// the kernel flushes every route through it, so the withdraw sysrib sends next
// finds nothing). In that case delOwnedRoute returns (false, nil): the route is
// gone, so no fib-sync-failure is raised, and callers such as the stale sweep
// do not count it as a route they removed.
func delOwnedRoute(h netlinkHandle, route *netlink.Route) (bool, error) {
	err := h.RouteDel(route)
	if errors.Is(err, unix.ESRCH) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
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
	var delErr error
	failed := 0
	for i := range current {
		if current[i].Protocol != rtprotZE {
			continue
		}
		if current[i].Priority == priority {
			continue
		}
		err := n.handle.RouteDel(&current[i])
		if errors.Is(err, unix.ESRCH) {
			continue
		}
		if err != nil {
			failed++
			delErr = errors.Join(delErr, fmt.Errorf("remove superseded route %s priority %d: %w", route.Dst, current[i].Priority, err))
		}
	}
	if delErr != nil {
		fwdTable := uint32(0)
		if table > 0 && uint64(table) <= math.MaxUint32 {
			fwdTable = forwardingTable(uint32(table))
		}
		return &supersededDeleteError{
			table: fwdTable,
			count: failed,
			err:   delErr,
		}
	}
	return nil
}

// mainTableID is Linux's RT_TABLE_MAIN. A change names the main table as 0,
// and the kernel lists it as 254, so the two compare through forwardingTable.
const mainTableID = 254

// forwardingTable answers a kernel table ID in a change's form: the main
// table becomes 0, the table a change carries when it names none.
func forwardingTable(kernelTable uint32) uint32 {
	if kernelTable == mainTableID {
		return 0
	}
	return kernelTable
}

// listZeRoutes lists Ze's routes in every routing table.
//
// A bare RouteList answers the main table only: the bindings drop every
// other table unless the filter mask holds RT_FILTER_TABLE, and with that bit
// a filter table of RT_TABLE_UNSPEC (0) matches every table
// (vendor/github.com/vishvananda/netlink/route_linux.go,
// RouteListFilteredIter). A route in a named table would otherwise be
// invisible to the startup sweep and to the flush's residue check.
//
// The private MPLS push-context tables are left out: their routes are not
// forwarding state the sysrib path installs, and the context lifecycle in
// mplscontext_linux.go removes them at close, after the flush.
func (n *netlinkBackend) listZeRoutes() ([]installedRoute, error) {
	routes, err := n.handle.RouteListFiltered(netlink.FAMILY_ALL,
		&netlink.Route{Table: unix.RT_TABLE_UNSPEC, Protocol: rtprotZE},
		netlink.RT_FILTER_TABLE|netlink.RT_FILTER_PROTOCOL)
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
		if routes[i].Table < 0 || uint64(routes[i].Table) > math.MaxUint32 {
			return nil, fmt.Errorf("route list: %s carries table %d, outside the kernel's table range", routes[i].Dst, routes[i].Table)
		}
		table := uint32(routes[i].Table)
		if table&mplsContextMask == mplsContextMark {
			continue
		}
		ir := installedRoute{prefix: routes[i].Dst.String(), table: forwardingTable(table)}
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
