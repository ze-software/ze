// Design: plan/spec-fib-depth.md -- rich route programming
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
	Prefix    netip.Prefix
	NextHop   netip.Addr
	RouteType sysribevents.RouteType
	Metric    uint32
	TableID   uint32
	Labels    []uint32
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
