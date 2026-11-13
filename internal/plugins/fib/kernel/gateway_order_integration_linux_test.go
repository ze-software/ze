//go:build integration && linux

// Design: docs/architecture/rib/unified-locrib.md -- publication order.
// Related: fibkernel.go -- processEvent applies a batch strictly in order.

package fibkernel

import (
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/test/userns"
)

const (
	gatewayOrderCover     = "10.248.0.0/24"
	gatewayOrderDependent = "10.249.3.0/24"
	gatewayOrderGateway   = "10.248.0.3"
)

// gatewayOrderChanges are the two routes of the system-RIB BGP metric
// scenario that the kernel refused: a device-only cover on ze-scope0 and a
// route whose resolved gateway only that cover reaches.
func gatewayOrderChanges() (cover, dependent incomingChange) {
	cover = incomingChange{
		Action: routeaction.Add, Prefix: netip.MustParsePrefix(gatewayOrderCover),
		Interface: "ze-scope0", RouteType: events.RouteTypeUnicast, Protocol: "ospf",
	}
	dependent = incomingChange{
		Action: routeaction.Add, Prefix: netip.MustParsePrefix(gatewayOrderDependent),
		NextHop: netip.MustParseAddr(gatewayOrderGateway), Interface: "ze-scope0",
		RouteType: events.RouteTypeUnicast, Protocol: "bgp",
	}
	return cover, dependent
}

// gatewayOrderRoute returns the Ze route the kernel holds for prefix.
func gatewayOrderRoute(t *testing.T, h *netlink.Handle, prefix string) (netlink.Route, bool) {
	t.Helper()
	routes := zeRoutes(t, h)
	for i := range routes {
		if routes[i].Dst != nil && routes[i].Dst.String() == prefix {
			return routes[i], true
		}
	}
	return netlink.Route{}, false
}

// Goal: the control for the order sysrib publishes. A batch [cover, dependent]
// applied through processEvent installs both, the dependent with its gateway
// and device. Method: a private namespace, the daemon's entry, and a read-back
// of each route from the kernel.
func TestFIBKernelInstallsDependentAfterCover(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withScopeBackend(t, func(backend *netlinkBackend, h *netlink.Handle, indices [2]int) {
		report.ResetForTest()
		defer report.ResetForTest()
		fib := newFIBKernel(backend)
		cover, dependent := gatewayOrderChanges()
		fib.processEvent(makeSysribPayload([]incomingChange{cover, dependent}))
		if failures := report.Errors(0); len(failures) != 0 {
			t.Fatalf("cover-first batch raised FIB errors: %+v", failures)
		}
		if _, present := gatewayOrderRoute(t, h, gatewayOrderCover); !present {
			t.Fatalf("kernel lacks the cover %s: %+v", gatewayOrderCover, zeRoutes(t, h))
		}
		route, present := gatewayOrderRoute(t, h, gatewayOrderDependent)
		if !present {
			t.Fatalf("kernel lacks the dependent %s: %+v", gatewayOrderDependent, zeRoutes(t, h))
		}
		if !route.Gw.Equal(net.ParseIP(gatewayOrderGateway)) || route.LinkIndex != indices[0] {
			t.Fatalf("dependent installed as %+v, want via %s dev ze-scope0", route, gatewayOrderGateway)
		}
	})
}

// Goal: pin the kernel contract the publication order exists for. The kernel
// refuses a route whose gateway no installed route reaches with ENETUNREACH,
// and processEvent, which applies a batch strictly in order, then leaves the
// dependent out of the kernel for good in a [dependent, cover] batch. Method:
// a raw netlink add of the dependent with no cover, then the reversed batch
// through processEvent, then a read-back.
func TestFIBKernelRefusesDependentBeforeCover(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withScopeBackend(t, func(backend *netlinkBackend, h *netlink.Handle, indices [2]int) {
		report.ResetForTest()
		defer report.ResetForTest()
		_, destination, err := net.ParseCIDR(gatewayOrderDependent)
		if err != nil {
			t.Fatal(err)
		}
		err = h.RouteAdd(&netlink.Route{Dst: destination, Gw: net.ParseIP(gatewayOrderGateway), LinkIndex: indices[0], Protocol: rtprotZE})
		if !errors.Is(err, unix.ENETUNREACH) {
			t.Fatalf("kernel answered %v to a gateway no route reaches, want ENETUNREACH", err)
		}

		fib := newFIBKernel(backend)
		cover, dependent := gatewayOrderChanges()
		fib.processEvent(makeSysribPayload([]incomingChange{dependent, cover}))
		failures := report.Errors(0)
		if len(failures) != 1 || failures[0].Subject != gatewayOrderDependent || failures[0].Code != reportCodeFIBSyncFailure {
			t.Fatalf("reversed batch raised %+v, want one %s failure for %s", failures, reportCodeFIBSyncFailure, gatewayOrderDependent)
		}
		if _, present := gatewayOrderRoute(t, h, gatewayOrderDependent); present {
			t.Fatalf("kernel installed %s ahead of its cover: %+v", gatewayOrderDependent, zeRoutes(t, h))
		}
		if _, present := gatewayOrderRoute(t, h, gatewayOrderCover); !present {
			t.Fatalf("kernel lacks the cover %s: %+v", gatewayOrderCover, zeRoutes(t, h))
		}
	})
}

// Goal: withdrawal needs no order. Deleting the cover first leaves the
// dependent installed, and deleting the dependent next still succeeds, so
// sysrib never moves a withdrawal. Method: install [cover, dependent], withdraw
// the cover alone and read the dependent back, then withdraw the dependent.
func TestFIBKernelDeletesCoverBeforeDependent(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withScopeBackend(t, func(backend *netlinkBackend, h *netlink.Handle, indices [2]int) {
		report.ResetForTest()
		defer report.ResetForTest()
		fib := newFIBKernel(backend)
		cover, dependent := gatewayOrderChanges()
		fib.processEvent(makeSysribPayload([]incomingChange{cover, dependent}))
		if _, present := gatewayOrderRoute(t, h, gatewayOrderDependent); !present {
			t.Fatalf("setup: kernel lacks the dependent %s: %+v", gatewayOrderDependent, zeRoutes(t, h))
		}

		cover.Action = routeaction.Withdraw
		fib.processEvent(makeSysribPayload([]incomingChange{cover}))
		if _, present := gatewayOrderRoute(t, h, gatewayOrderCover); present {
			t.Fatalf("withdrawn cover %s is still installed", gatewayOrderCover)
		}
		route, present := gatewayOrderRoute(t, h, gatewayOrderDependent)
		if !present {
			t.Fatalf("kernel removed %s with its cover: deletion order would matter", gatewayOrderDependent)
		}
		if route.LinkIndex != indices[0] {
			t.Fatalf("dependent changed after its cover went: %+v", route)
		}

		dependent.Action = routeaction.Withdraw
		fib.processEvent(makeSysribPayload([]incomingChange{dependent}))
		if left := zeRoutes(t, h); len(left) != 0 {
			t.Fatalf("kernel still holds Ze routes after both withdrawals: %+v", left)
		}
		if failures := report.Errors(0); len(failures) != 0 {
			t.Fatalf("cover-first withdrawals raised FIB errors: %+v", failures)
		}
	})
}
