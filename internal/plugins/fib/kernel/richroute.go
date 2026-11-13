// Design: plan/release/validation/spec-fib-depth.md -- rich route programming
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
	PathMTU   uint32
	SRv6SID   netip.Addr
	ECMPPaths []sysribevents.ECMPPath
	// Backup is the fast-reroute backup next-hop set the routing protocol
	// computed, distinct from ECMPPaths (which load-share in steady state). It
	// is part of the route's identity for replay, and it is not programmed: the
	// kernel has no standby member, so buildRichRoute sends the primary path(s)
	// alone and backup_report.go reports the backups the kernel lacks.
	Backup []sysribevents.ECMPPath
}

// richRouteBackend extends routeBackend with rich route programming.
// Backends that implement this interface receive full route attributes.
type richRouteBackend interface {
	addRichRoute(r RichRoute) error
	delRichRoute(prefix netip.Prefix, tableID uint32) error
	replaceRichRoute(r RichRoute) error
}
