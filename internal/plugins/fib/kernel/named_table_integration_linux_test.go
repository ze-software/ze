//go:build integration && linux

// Design: docs/architecture/core-design.md -- FIB shutdown flush.
// Related: backend_linux.go -- ownedRouteKey, the delete key that names the table the route was installed in.
// Related: fibkernel.go -- delChange and flushOneLocked, which pass the change's table to the delete.

package fibkernel

import (
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/test/userns"
)

// namedTableID is a routing table other than main, as a static route in a
// named table or a VRF carries.
const namedTableID = 100

// namedTableChange is an IPv4 device-only route in namedTableID: link scope
// (forwardingScope), so only a scope-free key in the right table removes it.
func namedTableChange() incomingChange {
	return incomingChange{
		Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.18.120.0/24"),
		Interface: "ze-scope0", RouteType: events.RouteTypeUnicast, Protocol: "static", TableID: namedTableID,
	}
}

// namedTableRoutes lists the IPv4 routes of one owner in the named table. A
// bare RouteList shows the main table only.
func namedTableRoutes(t *testing.T, h *netlink.Handle, protocol netlink.RouteProtocol) []netlink.Route {
	t.Helper()
	routes, err := h.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: namedTableID}, netlink.RT_FILTER_TABLE)
	if err != nil {
		t.Fatalf("list table %d: %v", namedTableID, err)
	}
	var owned []netlink.Route
	for i := range routes {
		if routes[i].Protocol == protocol {
			owned = append(owned, routes[i])
		}
	}
	return owned
}

// installNamedTableRoute programs namedTableChange through processEvent with
// two foreign routes on the same prefix: one in the named table at another
// metric, one in the main table. It proves Ze's route landed in the named
// table at link scope and nowhere else, so a delete that names no table, or
// the main one, misses it.
func installNamedTableRoute(t *testing.T, fib *fibKernel, h *netlink.Handle, index int) {
	t.Helper()
	change := namedTableChange()
	_, cidr, err := net.ParseCIDR(change.Prefix.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.RouteAdd(&netlink.Route{
		Dst: cidr, LinkIndex: index, Scope: netlink.SCOPE_LINK,
		Protocol: foreignProtocol, Priority: 500, Table: namedTableID,
	}); err != nil {
		t.Fatalf("install foreign route in table %d: %v", namedTableID, err)
	}
	addForeignDeviceRoute(t, h, change.Prefix.String(), index, 0)
	fib.processEvent(makeSysribPayload([]incomingChange{change}))
	if failures := report.Errors(0); len(failures) != 0 {
		t.Fatalf("installation raised FIB errors: %+v", failures)
	}
	installed := namedTableRoutes(t, h, rtprotZE)
	if len(installed) != 1 {
		t.Fatalf("table %d holds %d Ze routes after installation, want 1: %+v", namedTableID, len(installed), installed)
	}
	if installed[0].Scope != netlink.SCOPE_LINK {
		t.Fatalf("Ze's route in table %d has scope %v, want link: %+v", namedTableID, installed[0].Scope, installed[0])
	}
	if main := zeRoutes(t, h); len(main) != 0 {
		t.Fatalf("the main table holds %d Ze routes, want none: %+v", len(main), main)
	}
}

func requireNamedTableForeignRoutesSurvive(t *testing.T, h *netlink.Handle) {
	t.Helper()
	if named := namedTableRoutes(t, h, foreignProtocol); len(named) != 1 {
		t.Errorf("table %d holds %d foreign routes, want the one survivor: %+v", namedTableID, len(named), named)
	}
	if main := routesByProtocol(t, h, foreignProtocol); len(main) != 1 {
		t.Errorf("the main table holds %d foreign routes, want the one survivor: %+v", len(main), main)
	}
}

// Goal: a producer's withdraw of a route in a table other than main removes
// Ze's route from that table and nothing another daemon owns. Method: in a
// private namespace, install the route through processEvent beside two foreign
// routes on the same prefix (named table and main), withdraw it through
// processEvent, and list both tables. A delete key without the table asks the
// main table, finds no Ze route there, and the kernel's ESRCH reads as "already
// gone", so the route would stay forwarding with Ze no longer tracking it.
func TestKernelWithdrawRemovesNamedTableRoute(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withScopeBackend(t, func(backend *netlinkBackend, h *netlink.Handle, indices [2]int) {
		report.ResetForTest()
		defer report.ResetForTest()
		fib := newFIBKernel(backend)
		installNamedTableRoute(t, fib, h, indices[0])

		withdrawal := namedTableChange()
		withdrawal.Action = routeaction.Withdraw
		fib.processEvent(makeSysribPayload([]incomingChange{withdrawal}))

		if left := namedTableRoutes(t, h, rtprotZE); len(left) != 0 {
			t.Errorf("table %d still holds %d Ze routes after the withdraw: %+v", namedTableID, len(left), left)
		}
		if len(fib.installed) != 0 {
			t.Errorf("writer still tracks %d routes after the withdraw", len(fib.installed))
		}
		if failures := report.Errors(0); len(failures) != 0 {
			t.Errorf("the withdraw raised FIB errors: %+v", failures)
		}
		requireNamedTableForeignRoutesSurvive(t, h)
	})
}

// Goal: flush-on-stop removes Ze's route from a table other than main, and
// nothing another daemon owns. Method: install the route through processEvent
// beside the two foreign routes, run flushRoutes, the function the daemon's
// stop runs with flush-on-stop, and list both tables. The kernel listing here
// is the evidence; the flush's own residue check, which lists every table, is
// the second witness and must stay silent.
func TestKernelFlushRemovesNamedTableRoute(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withScopeBackend(t, func(backend *netlinkBackend, h *netlink.Handle, indices [2]int) {
		report.ResetForTest()
		defer report.ResetForTest()
		fib := newFIBKernel(backend)
		installNamedTableRoute(t, fib, h, indices[0])

		fib.flushRoutes()

		if left := namedTableRoutes(t, h, rtprotZE); len(left) != 0 {
			t.Errorf("table %d still holds %d Ze routes after the flush: %+v", namedTableID, len(left), left)
		}
		if issues := flushIssues(); len(issues) != 0 {
			t.Errorf("the flush raised flush failures: %+v", issues)
		}
		requireNamedTableForeignRoutesSurvive(t, h)
	})
}

// addLeftoverZeRoute installs a Ze-protocol device route directly, as a
// previous run of Ze left it in the kernel, without this writer knowing.
func addLeftoverZeRoute(t *testing.T, h *netlink.Handle, prefix string, index, table int) {
	t.Helper()
	_, cidr, err := net.ParseCIDR(prefix)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.RouteAdd(&netlink.Route{
		Dst: cidr, LinkIndex: index, Scope: netlink.SCOPE_LINK, Protocol: rtprotZE, Table: table,
	}); err != nil {
		t.Fatalf("install leftover Ze route %s in table %d: %v", prefix, table, err)
	}
}

// Goal: the flush's residue check sees a Ze route the flush did not remove
// when it sits in a table other than main. Method: install a route through
// processEvent in the main table, and leave a Ze route for the same prefix in
// namedTableID that this writer does not track (no startup sweep ran), so the
// flush removes the main one and misses the other. The flush MUST report one
// residue naming that table. A listing of the main table alone counts zero
// and reports a clean flush over a route that still forwards.
func TestKernelFlushReportsNamedTableResidue(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withScopeBackend(t, func(backend *netlinkBackend, h *netlink.Handle, indices [2]int) {
		report.ResetForTest()
		defer report.ResetForTest()
		fib := newFIBKernel(backend)
		prefix := "198.18.121.0/24"
		addLeftoverZeRoute(t, h, prefix, indices[0], namedTableID)
		fib.processEvent(makeSysribPayload([]incomingChange{{
			Action: routeaction.Add, Prefix: netip.MustParsePrefix(prefix),
			Interface: "ze-scope0", RouteType: events.RouteTypeUnicast, Protocol: "static",
		}}))
		if main := zeRoutes(t, h); len(main) != 1 {
			t.Fatalf("the main table holds %d Ze routes after installation, want 1: %+v", len(main), main)
		}

		fib.flushRoutes()

		if main := zeRoutes(t, h); len(main) != 0 {
			t.Fatalf("the flush left %d Ze routes in the main table: %+v", len(main), main)
		}
		if left := namedTableRoutes(t, h, rtprotZE); len(left) != 1 {
			t.Fatalf("table %d holds %d Ze routes, want the one leftover the flush did not know: %+v", namedTableID, len(left), left)
		}
		issues := flushIssues()
		if len(issues) != 1 {
			t.Fatalf("the flush raised %d flush failures, want one naming the residue: %+v", len(issues), issues)
		}
		if issues[0].Detail["residue"] != 1 {
			t.Errorf("residue = %v, want 1", issues[0].Detail["residue"])
		}
		if issues[0].Detail["first-residue"] != prefix {
			t.Errorf("first residue = %v, want %s", issues[0].Detail["first-residue"], prefix)
		}
		if issues[0].Detail["first-residue-table"] != uint32(namedTableID) {
			t.Errorf("first residue table = %v, want %d", issues[0].Detail["first-residue-table"], namedTableID)
		}
	})
}

// Goal: the startup sweep removes a Ze route a previous run left in a table
// other than main, keeps the route sysrib refreshed, and touches nothing
// another daemon owns. Method: leave three Ze routes as a crashed run would:
// 198.18.122.0/24 in namedTableID only, and 198.18.123.0/24 in both main and
// namedTableID. A new writer runs startupSweep, sysrib refreshes
// 198.18.123.0/24 in main through processEvent, and sweepStale runs. Only the
// refreshed main route may stay. A sweep that lists the main table alone never
// sees either named-table route, and one that deletes by prefix alone asks
// the main table for them.
func TestKernelStartupSweepRemovesNamedTableRoute(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withScopeBackend(t, func(backend *netlinkBackend, h *netlink.Handle, indices [2]int) {
		report.ResetForTest()
		defer report.ResetForTest()
		addLeftoverZeRoute(t, h, "198.18.122.0/24", indices[0], namedTableID)
		addLeftoverZeRoute(t, h, "198.18.123.0/24", indices[0], namedTableID)
		addLeftoverZeRoute(t, h, "198.18.123.0/24", indices[0], 0)
		addForeignDeviceRoute(t, h, "198.18.124.0/24", indices[0], 0)
		if err := h.RouteAdd(&netlink.Route{
			Dst: &net.IPNet{IP: net.IPv4(198, 18, 122, 0).To4(), Mask: net.CIDRMask(24, 32)}, LinkIndex: indices[0],
			Scope: netlink.SCOPE_LINK, Protocol: foreignProtocol, Priority: 500, Table: namedTableID,
		}); err != nil {
			t.Fatalf("install foreign route in table %d: %v", namedTableID, err)
		}

		fib := newFIBKernel(backend)
		stale := fib.startupSweep()
		if len(stale) != 2 {
			t.Fatalf("startup listed %d stale prefixes, want both leftover prefixes: %v", len(stale), stale)
		}
		fib.processEvent(makeSysribPayload([]incomingChange{{
			Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.18.123.0/24"),
			Interface: "ze-scope0", RouteType: events.RouteTypeUnicast, Protocol: "static",
		}}))
		if failures := report.Errors(0); len(failures) != 0 {
			t.Fatalf("the refresh raised FIB errors: %+v", failures)
		}

		fib.sweepStale(stale)

		if left := namedTableRoutes(t, h, rtprotZE); len(left) != 0 {
			t.Errorf("table %d still holds %d Ze routes after the sweep: %+v", namedTableID, len(left), left)
		}
		main := zeRoutes(t, h)
		if len(main) != 1 {
			t.Fatalf("the main table holds %d Ze routes after the sweep, want the refreshed one: %+v", len(main), main)
		}
		if main[0].Dst.String() != "198.18.123.0/24" {
			t.Errorf("the main table kept %s, want the refreshed 198.18.123.0/24", main[0].Dst)
		}
		if _, tracked := fib.installed["198.18.123.0/24"]; !tracked {
			t.Error("the sweep dropped the refreshed route from the writer's state")
		}
		if named := namedTableRoutes(t, h, foreignProtocol); len(named) != 1 {
			t.Errorf("table %d holds %d foreign routes, want the one survivor: %+v", namedTableID, len(named), named)
		}
		if foreign := routesByProtocol(t, h, foreignProtocol); len(foreign) != 1 {
			t.Errorf("the main table holds %d foreign routes, want the one survivor: %+v", len(foreign), foreign)
		}
	})
}
