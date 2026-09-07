// Design: docs/architecture/static-routes.md -- main-table routes reach the FIB through the Loc-RIB
// Related: inject.go -- routeManager, which calls applyProgrammed and withdrawProgrammed
// Related: internal/component/sysrib/sysrib.go -- the arbitration a static Path enters

package static

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribdistance "github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/nexthop"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	staticevents "github.com/ze-software/ze/internal/plugins/static/events"
)

// DefaultAdminDistance is the classical static administrative distance, and the
// value ze-rib-conf.yang declares as the default of `rib { distance { static } }`.
// It is reachable only before sysrib publishes the declaration, which sysrib does
// at process start from that same schema, so a running daemon stamps the
// operator's value.
//
// Exported and named for the bootstrap-distance gate, which pairs every seam
// reader's constant with the YANG leaf it stands in for
// (internal/component/sysrib/distance_bootstrap_test.go).
const DefaultAdminDistance uint8 = 10

// routeSink receives Loc-RIB install and remove operations when the plugin holds
// no local Loc-RIB, which is a forked subprocess: locrib.Default() answers nil
// there (internal/core/rib/locrib/default.go, gated on ze.plugin.hub.token). The
// forked wiring installs one, and it ships each operation to the engine over the
// route-install RPC. In-process the local RIB is preferred and this stays nil.
// The method set is the one ospfspf.RouteSink declares, so *routeinstall.Sink
// satisfies both.
type routeSink interface {
	InsertForward(fam family.Family, prefix netip.Prefix, p locrib.Path)
	Remove(fam family.Family, prefix netip.Prefix, source redistevents.ProtocolID, instance uint32)
	Flush()
}

// setLocRIB records where main-table routes are installed. loc is the shared
// Loc-RIB when static runs in-process and nil when it runs forked, in which case
// remote carries the operations to the engine. With neither, a main-table route
// has nowhere to go and applyProgrammed says so rather than dropping it.
//
// Called once at plugin start, before any route is applied.
func (rm *routeManager) setLocRIB(loc *locrib.RIB, remote routeSink) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.loc = loc
	rm.remote = remote
}

// inMainTable reports whether the route belongs to the kernel main table, which
// is the table static spells 0 and the only one the Loc-RIB arbitrates.
//
// The Loc-RIB is keyed by (family, prefix) and carries no table, so a route in a
// NAMED table cannot enter it without colliding with the main-table route for the
// same prefix. A named table also has exactly one writer by construction, so
// there is nothing there for an administrative distance to decide. Named-table
// routes therefore keep the direct data-plane write. The table dimension belongs
// to plan/immediate/spec-fib-depth.md, which owns BestChangeEntry.TableID.
func (r staticRoute) inMainTable() bool { return r.Table == 0 }

// applyProgrammed installs one route as the operator declared it, with the
// next-hop set the caller has already narrowed to the members BFD reports up.
//
// A main-table route becomes a Loc-RIB Path, so Ze's declared distance decides
// between it and a route another protocol offers for the same prefix, and the FIB
// plugin is the single writer that programs the winner. A named-table route goes
// straight to the data plane, unchanged.
func (rm *routeManager) applyProgrammed(r staticRoute) error {
	if !r.inMainTable() {
		return rm.backend.applyRoute(r)
	}
	return rm.insertPathLocked(r)
}

// withdrawProgrammed removes one route from wherever applyProgrammed put it.
func (rm *routeManager) withdrawProgrammed(r staticRoute) error {
	if !r.inMainTable() {
		return rm.backend.removeRoute(r)
	}
	rm.removePathLocked(r)
	return nil
}

// insertPathLocked builds the route's Loc-RIB Path and inserts it.
//
// Every reason to REFUSE a route is evaluated here, before the insert, so a route
// the operator cannot have stays synchronous: the config transaction fails, the
// route is recorded as skipped, and no Path reaches the Loc-RIB. What moved to
// the FIB plugin is the netlink write itself, whose failures the FIB reports as
// fib-sync-failure.
//
// REQUIRES: the caller holds rm.mu.
func (rm *routeManager) insertPathLocked(r staticRoute) error {
	path, err := staticPath(r)
	if err != nil {
		return err
	}
	fam := familyOf(r.Prefix)
	if rm.loc != nil {
		rm.loc.InsertForward(fam, r.Prefix, path, nil)
		return nil
	}
	if rm.remote != nil {
		rm.remote.InsertForward(fam, r.Prefix, path)
		rm.remote.Flush()
		return nil
	}
	return fmt.Errorf("no system RIB to install into: static runs with neither a shared Loc-RIB nor a route-install channel")
}

// removePathLocked withdraws the route's Loc-RIB Path. A prefix static never
// installed is not an error: the Loc-RIB drops an unknown (source, instance).
//
// REQUIRES: the caller holds rm.mu.
func (rm *routeManager) removePathLocked(r staticRoute) {
	fam := familyOf(r.Prefix)
	if rm.loc != nil {
		rm.loc.Remove(fam, r.Prefix, staticevents.ProtocolID, pathInstance)
		return
	}
	if rm.remote != nil {
		rm.remote.Remove(fam, r.Prefix, staticevents.ProtocolID, pathInstance)
		rm.remote.Flush()
	}
}

// pathInstance is the within-protocol identifier every static Path carries.
//
// It is a constant because the route map is keyed by (table, prefix): one
// main-table prefix holds at most one static route, and that route names its
// whole next-hop set on one Path. Static therefore never inserts two Paths for a
// prefix the way IS-IS and OSPF do, and needs nothing to tell them apart.
const pathInstance uint32 = 0

// staticPath renders one configured route as a Loc-RIB Path, or refuses it.
//
// The first next-hop becomes the Path's own, and the rest its equal-cost group;
// the parser sorts a route's next-hops, so which one is first does not change
// between applies. A blackhole or reject route names no next-hop and carries its
// forwarding action instead.
func staticPath(r staticRoute) (locrib.Path, error) {
	if err := validateRouteMetric(r.Metric, maxNetlinkInt); err != nil {
		return locrib.Path{}, err
	}
	path := locrib.Path{
		Source:   staticevents.ProtocolID,
		Instance: pathInstance,
		Metric:   r.Metric,
		// The seam carries `rib { distance { static } }` from sysrib, which is
		// the one declaration. Read at insert so a reload takes effect on the
		// next apply rather than at the next restart.
		AdminDistance: ribdistance.OrDefault("static", DefaultAdminDistance),
	}

	switch r.Action {
	case actionBlackhole:
		path.RouteType = routetype.Blackhole
		return path, nil
	case actionReject:
		path.RouteType = routetype.Unreachable
		return path, nil
	case actionForward:
		// Handled below.
	default:
		return locrib.Path{}, fmt.Errorf("unknown action %d", r.Action)
	}

	if len(r.NextHops) == 0 {
		return locrib.Path{}, fmt.Errorf("route %s: no next-hop to install", r.Prefix)
	}
	for i := range r.NextHops {
		if err := checkNextHopResolvable(r.NextHops[i]); err != nil {
			return locrib.Path{}, err
		}
	}
	path.RouteType = routetype.Unicast
	path.NextHop = r.NextHops[0].Address
	path.Interface = r.NextHops[0].Interface
	path.Weight = capNextHopWeight(r.NextHops[0].Weight)
	if len(r.NextHops) > 1 {
		group := make([]nexthop.NextHop, 0, len(r.NextHops)-1)
		for _, nh := range r.NextHops[1:] {
			group = append(group, nexthop.NextHop{
				Addr:      nh.Address,
				Interface: nh.Interface,
				Weight:    capNextHopWeight(nh.Weight),
			})
		}
		path.ECMP = group
	}
	return path, nil
}

// checkNextHopResolvable refuses a next-hop whose interface no backend can name.
//
// The FIB plugin resolves the name again when it programs the route, and that is
// not a duplicate decision: this one answers the operator at commit time, so a
// name that cannot resolve fails the transaction instead of leaving a prefix
// quietly unrouted. Both ask iface.ResolveIndex, so they cannot disagree about
// what resolves.
func checkNextHopResolvable(nh nextHop) error {
	if nh.Interface == "" {
		if !nh.Address.IsValid() {
			return fmt.Errorf("next-hop names neither an address nor an interface")
		}
		return nil
	}
	if _, err := iface.ResolveIndex(nh.Interface); err != nil {
		return err
	}
	return nil
}

// capNextHopWeight narrows the configured uint16 weight to what a data plane can
// express. The kernel carries a share in rtnh_hops, one octet, and VPP carries it
// in fib_path.Weight, also one octet, so 255 is the ceiling on both. A larger
// configured value used to wrap inside the netlink encoder and give the next-hop
// a smaller share than any other member of the group.
func capNextHopWeight(w uint16) uint8 {
	if w > 255 {
		return 255
	}
	return uint8(w)
}

// familyOf returns the Loc-RIB family a prefix belongs to.
func familyOf(p netip.Prefix) family.Family {
	if p.Addr().Is4() {
		return family.IPv4Unicast
	}
	return family.IPv6Unicast
}
