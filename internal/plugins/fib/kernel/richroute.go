// Design: plan/immediate/spec-fib-depth.md -- rich route programming
// Related: fibkernel.go -- processEvent uses richRouteBackend when available
// Related: backend_linux.go -- netlinkBackend implements richRouteBackend

package fibkernel

import (
	"net/netip"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
)

// RichRoute carries all attributes needed for full FIB programming.
// Value-typed: no heap escapes in the hot path when stack-allocated.
type RichRoute struct {
	Prefix  netip.Prefix
	NextHop netip.Addr
	// Interface is the outgoing device name for NextHop, empty when the gateway
	// alone names the next-hop. Weight is NextHop's share of the multipath group
	// ECMPPaths completes, zero for an unweighted route. OnLink makes the kernel
	// use this device directly even without a covering gateway subnet.
	Interface string
	OnLink    bool
	Weight    uint8
	RouteType sysribevents.RouteType
	Metric    uint32
	TableID   uint32
	Labels    []uint32
	// PathMTU is a labeled path's frame budget, before MPLS encapsulation.
	// Linux's IP MTU calculation subtracts the LWT label headroom.
	PathMTU uint32
	SRv6SID   netip.Addr
	ECMPPaths []sysribevents.ECMPPath
	// Backup is the fast-reroute backup next-hop set: programmed as link-down /
	// backup multipath next-hop(s) with the repair MPLS encap, so the kernel
	// forwards to a backup only when the primary next-hop's link is down. Distinct
	// from ECMPPaths (which load-share in steady state).
	Backup []sysribevents.ECMPPath
}

// richRouteBackend extends routeBackend with rich route programming.
// Backends that implement this interface receive full route attributes.
type richRouteBackend interface {
	addRichRoute(r RichRoute) error
	delRichRoute(prefix netip.Prefix, tableID uint32) error
	replaceRichRoute(r RichRoute) error
}
