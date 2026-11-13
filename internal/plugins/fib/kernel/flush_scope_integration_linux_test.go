//go:build integration && linux

// Design: docs/architecture/core-design.md -- FIB shutdown flush.
// Related: backend_linux.go -- ownedRouteKey, the delete key that matches every scope.
// Related: fibkernel.go -- flushRoutes and the withdraw branch of processEvent.

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

// foreignProtocol is an rtm_protocol no Ze producer uses, standing for another
// routing daemon on the same box.
const foreignProtocol = 99

// scopeFlushChanges are the four route shapes the daemon installs: an IPv4
// device-only route and an IPv4 device-only two-member ECMP route (both at
// RT_SCOPE_LINK, forwardingScope), an IPv4 gateway route and an IPv6
// device-only route (both universe). The two IPv4 device-only prefixes and
// metrics are the ones the daemon's flush-on-stop scenarios left behind.
func scopeFlushChanges() []incomingChange {
	return []incomingChange{
		{
			Action: routeaction.Add, Prefix: netip.MustParsePrefix("10.248.0.0/24"),
			Interface: "ze-scope0", RouteType: events.RouteTypeUnicast, Protocol: "static",
		},
		{
			Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.18.250.0/24"),
			Interface: "ze-scope0", Weight: 3, Metric: 7, RouteType: events.RouteTypeUnicast, Protocol: "static",
			ECMPPaths: []events.ECMPPath{{Interface: "ze-scope1", Weight: 7}},
		},
		{
			Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.19.90.0/24"),
			NextHop: netip.MustParseAddr("198.18.83.1"), Protocol: "static",
		},
		{
			Action: routeaction.Add, Prefix: netip.MustParsePrefix("2001:db8:90::/64"),
			Interface: "ze-scope0", RouteType: events.RouteTypeUnicast, Protocol: "static",
		},
	}
}

// addForeignDeviceRoute installs another daemon's link-scope device route.
func addForeignDeviceRoute(t *testing.T, h *netlink.Handle, prefix string, index, metric int) {
	t.Helper()
	_, cidr, err := net.ParseCIDR(prefix)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.RouteAdd(&netlink.Route{
		Dst: cidr, LinkIndex: index, Scope: netlink.SCOPE_LINK,
		Protocol: foreignProtocol, Priority: metric,
	}); err != nil {
		t.Fatalf("install foreign route %s: %v", prefix, err)
	}
}

// installScopeFlushRoutes programs scopeFlushChanges through processEvent, the
// daemon's entry, and a foreign route beside them: one on a prefix Ze also
// owns (another metric) and one on a prefix Ze does not touch. It proves the
// IPv4 device-only routes really carry link scope, so the test exercises the
// case it is named for.
func installScopeFlushRoutes(t *testing.T, fib *fibKernel, h *netlink.Handle, index int) {
	t.Helper()
	addForeignDeviceRoute(t, h, "10.248.0.0/24", index, 500)
	addForeignDeviceRoute(t, h, "10.249.90.0/24", index, 0)
	fib.processEvent(makeSysribPayload(scopeFlushChanges()))
	if failures := report.Errors(0); len(failures) != 0 {
		t.Fatalf("installation raised FIB errors: %+v", failures)
	}
	installed := zeRoutes(t, h)
	if len(installed) != 4 {
		t.Fatalf("kernel holds %d Ze routes after installation, want 4: %+v", len(installed), installed)
	}
	linkScoped := 0
	for i := range installed {
		if installed[i].Scope == netlink.SCOPE_LINK {
			linkScoped++
		}
	}
	if linkScoped != 2 {
		t.Fatalf("kernel holds %d link-scope Ze routes, want the 2 IPv4 device-only ones: %+v", linkScoped, installed)
	}
}

func requireForeignRoutesSurvive(t *testing.T, h *netlink.Handle) {
	t.Helper()
	foreign := routesByProtocol(t, h, foreignProtocol)
	if len(foreign) != 2 {
		t.Fatalf("kernel holds %d foreign routes, want both survivors: %+v", len(foreign), foreign)
	}
}

// Goal: flush-on-stop leaves none of Ze's routes in the kernel, whatever their
// scope, and removes nothing another daemon owns. Method: install the four
// route shapes through processEvent in a private namespace with the devices
// still present (deleting a device would erase its routes and hide the
// defect), run flushRoutes, the function the daemon's stop runs with
// flush-on-stop, then list the kernel. The foreign routes, one of them on a
// prefix Ze also owned, are the survivors the flush must keep.
func TestKernelFlushRemovesEveryScope(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withScopeBackend(t, func(backend *netlinkBackend, h *netlink.Handle, indices [2]int) {
		report.ResetForTest()
		defer report.ResetForTest()
		fib := newFIBKernel(backend)
		installScopeFlushRoutes(t, fib, h, indices[0])

		fib.flushRoutes()

		if left := zeRoutes(t, h); len(left) != 0 {
			t.Errorf("kernel still holds %d Ze routes after flush: %+v", len(left), left)
		}
		requireForeignRoutesSurvive(t, h)
		if issues := flushIssues(); len(issues) != 0 {
			t.Errorf("a complete flush raised flush failures: %+v", issues)
		}
	})
}

// Goal: a producer's withdraw removes Ze's route, whatever its scope. Method:
// install the four route shapes through processEvent, withdraw each one
// through processEvent, and list the kernel. A delete that misses a link-scope
// route is answered ESRCH, which the backend reads as "already gone", so the
// route would stay with Ze no longer tracking it.
func TestKernelWithdrawRemovesEveryScope(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withScopeBackend(t, func(backend *netlinkBackend, h *netlink.Handle, indices [2]int) {
		report.ResetForTest()
		defer report.ResetForTest()
		fib := newFIBKernel(backend)
		installScopeFlushRoutes(t, fib, h, indices[0])

		withdrawals := scopeFlushChanges()
		for i := range withdrawals {
			withdrawals[i].Action = routeaction.Withdraw
		}
		fib.processEvent(makeSysribPayload(withdrawals))

		if left := zeRoutes(t, h); len(left) != 0 {
			t.Errorf("kernel still holds %d Ze routes after their withdraws: %+v", len(left), left)
		}
		if len(fib.installed) != 0 {
			t.Errorf("writer still tracks %d routes after their withdraws", len(fib.installed))
		}
		if failures := report.Errors(0); len(failures) != 0 {
			t.Errorf("withdraws raised FIB errors: %+v", failures)
		}
		requireForeignRoutesSurvive(t, h)
	})
}
