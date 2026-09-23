//go:build integration && linux

package fibkernel

import (
	"errors"
	"net"
	"net/netip"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// withNetNS creates an ephemeral network namespace, switches into it,
// runs fn, then restores the original namespace in t.Cleanup.
// Namespace creation requires CAP_SYS_ADMIN; route programming needs CAP_NET_ADMIN.
func withNetNS(t *testing.T, fn func()) {
	t.Helper()

	runtime.LockOSThread()

	origNS, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("cannot read current network namespace: %v", err)
	}

	nsName := sanitizeNSName(t.Name())

	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		current, currentErr := netns.Get()
		require.NoError(t, currentErr, "read namespace after failed creation")
		var restoreErr error
		if !current.Equal(origNS) {
			restoreErr = netns.Set(origNS)
		}
		current.Close() //nolint:errcheck // best-effort after namespace comparison
		origNS.Close()  //nolint:errcheck // best-effort after namespace restoration
		require.NoError(t, restoreErr, "restore namespace after failed creation")
		runtime.UnlockOSThread()
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("requires CAP_SYS_ADMIN to create and mount a network namespace; route programming also requires CAP_NET_ADMIN: %v", err)
		}
		t.Fatalf("cannot create test network namespace: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("failed to restore original namespace: %v", restoreErr)
		}
		origNS.Close()            //nolint:errcheck // best-effort cleanup
		newNS.Close()             //nolint:errcheck // best-effort cleanup
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})

	fn()
}

func sanitizeNSName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 15 {
		name = name[:15]
	}
	return name
}

// zeRoutes returns all fib-kernel routes in the namespace.
//
// This is a read-back: it says what the backend WROTE, not what the kernel
// would do with a packet. A route in the wrong table, shadowed by a better
// metric, or of a type that discards is listed here all the same. The
// forwarding decision is forwardingDecision below.
func zeRoutes(t *testing.T, h *netlink.Handle) []netlink.Route {
	t.Helper()
	routes, err := h.RouteList(nil, netlink.FAMILY_ALL)
	require.NoError(t, err)
	var result []netlink.Route
	for i := range routes {
		if routes[i].Protocol == rtprotZE {
			result = append(result, routes[i])
		}
	}
	return result
}

// forwardingDecision asks the kernel which FIB entry forwards a packet to
// dest, the way `ip route get fibmatch` does: RTM_GETROUTE with
// RTM_F_FIB_MATCH answers with the entry the longest-prefix match selected,
// carrying its prefix, gateway, device, protocol, metric and table, rather than
// with the resolved next-hop alone. That is the observation a read-back cannot
// make: a route that is present but never chosen reads back through RouteList
// and is absent here.
func forwardingDecision(t *testing.T, h *netlink.Handle, dest string) (netlink.Route, error) {
	t.Helper()
	ip := net.ParseIP(dest)
	require.NotNil(t, ip, "destination %q is not an IP", dest)
	routes, err := h.RouteGetWithOptions(ip, &netlink.RouteGetOptions{FIBMatch: true})
	if err != nil {
		return netlink.Route{}, err
	}
	require.Len(t, routes, 1, "kernel answered %d FIB entries for %s", len(routes), dest)
	return routes[0], nil
}

// requireForwards asserts the kernel forwards dest through the route named by
// prefix and nextHop over lo, owned by proto, in the main table, and returns
// the selected entry so a caller can read the metric the kernel chose.
func requireForwards(t *testing.T, h *netlink.Handle, dest, prefix, nextHop string, proto int) netlink.Route {
	t.Helper()
	route, err := forwardingDecision(t, h, dest)
	require.NoError(t, err, "kernel has no forwarding decision for %s", dest)
	require.NotNil(t, route.Dst, "kernel selected a default route for %s", dest)
	lo, err := h.LinkByName("lo")
	require.NoError(t, err)
	assert.Equal(t, prefix, route.Dst.String(), "kernel selected another prefix for %s", dest)
	assert.Equal(t, nextHop, route.Gw.String(), "kernel selected another next-hop for %s", dest)
	assert.Equal(t, lo.Attrs().Index, route.LinkIndex, "kernel selected another device for %s", dest)
	assert.Equal(t, netlink.RouteProtocol(proto), route.Protocol, "kernel selected another owner's route for %s", dest)
	assert.Equal(t, unix.RT_TABLE_MAIN, route.Table, "kernel selected a route outside the main table for %s", dest)
	return route
}

// requireUnroutable asserts the kernel has no forwarding decision for dest:
// RTM_GETROUTE answers ENETUNREACH. On its own this is an absence, and an
// absence is what an empty table also answers, so every caller pairs it with a
// requireForwards on the same destination taken before the removal under test.
func requireUnroutable(t *testing.T, h *netlink.Handle, dest string) {
	t.Helper()
	route, err := forwardingDecision(t, h, dest)
	require.ErrorIs(t, err, unix.ENETUNREACH, "kernel still forwards %s through %+v", dest, route)
}

// newTestBackend creates a netlink backend using a pre-existing handle.
// Allows injection of a namespace-scoped handle for integration testing.
func newTestBackend(h *netlink.Handle) routeBackend {
	return &netlinkBackend{handle: h}
}

// addLoopback brings up the loopback interface in the namespace.
// Routes need a valid device to resolve next-hops.
func addLoopback(t *testing.T, h *netlink.Handle) {
	t.Helper()
	lo, err := h.LinkByName("lo")
	require.NoError(t, err)
	require.NoError(t, h.LinkSetUp(lo))
}

func addProtocolRoute(t *testing.T, h *netlink.Handle, prefix, nextHop string, proto int) {
	t.Helper()
	addProtocolRouteWithMetric(t, h, prefix, nextHop, proto, 0)
}

func addProtocolRouteWithMetric(t *testing.T, h *netlink.Handle, prefix, nextHop string, proto, metric int) {
	t.Helper()
	_, cidr, err := net.ParseCIDR(prefix)
	require.NoError(t, err)
	gw := net.ParseIP(nextHop)
	require.NotNil(t, gw)
	require.NoError(t, h.RouteAdd(&netlink.Route{
		Dst:      cidr,
		Gw:       gw,
		Protocol: netlink.RouteProtocol(proto),
		Priority: metric,
	}))
}

func routesByProtocol(t *testing.T, h *netlink.Handle, proto int) []netlink.Route {
	t.Helper()
	routes, err := h.RouteList(nil, netlink.FAMILY_ALL)
	require.NoError(t, err)
	var out []netlink.Route
	for i := range routes {
		if routes[i].Protocol == netlink.RouteProtocol(proto) {
			out = append(out, routes[i])
		}
	}
	return out
}

func addChange(prefix, nextHop string) incomingChange { //nolint:unparam // nextHop kept explicit to mirror updateChange/withdrawChange
	return incomingChange{
		Action:   routeaction.Add,
		Prefix:   netip.MustParsePrefix(prefix),
		NextHop:  netip.MustParseAddr(nextHop),
		Protocol: "bgp",
	}
}

func updateChange(prefix, nextHop, protocol string) incomingChange {
	return incomingChange{
		Action:   routeaction.Update,
		Prefix:   netip.MustParsePrefix(prefix),
		NextHop:  netip.MustParseAddr(nextHop),
		Protocol: protocol,
	}
}

func withdrawChange(prefix string) incomingChange {
	return incomingChange{
		Action: routeaction.Withdraw,
		Prefix: netip.MustParsePrefix(prefix),
	}
}

// VALIDATES: AC-8 -- sysrib/best-change with action "add" installs route via netlink.
// VALIDATES: AC-16 -- fib-kernel routes use their producer-specific rtm_protocol ID.
// PREVENTS: netlink backend silently failing to program real kernel routes.
// PREVENTS: a route the kernel holds but never selects (discarding type, wrong
// table, unresolvable next-hop) passing as installed.
func TestNetlinkIntegration_AddRoute(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)

		// Control: the fresh namespace forwards nothing to the destination, so
		// the decision observed after the add is the backend's doing.
		requireUnroutable(t, h, "10.99.0.1")

		event := makeSysribPayload([]incomingChange{
			addChange("10.99.0.0/24", "127.0.0.1"),
		})
		f.processEvent(event)

		// Verify route exists in kernel with the fib-kernel route owner.
		routes := zeRoutes(t, h)
		require.Len(t, routes, 1, "expected 1 ze route in kernel")
		assert.Equal(t, "10.99.0.0/24", routes[0].Dst.String())
		assert.Equal(t, netlink.RouteProtocol(rtprotZE), routes[0].Protocol)

		// The kernel forwards a packet for the prefix through the route the
		// backend wrote.
		requireForwards(t, h, "10.99.0.1", "10.99.0.0/24", "127.0.0.1", rtprotZE)
	})
}

// VALIDATES: AC-9 -- sysrib/best-change with action "withdraw" removes route.
// PREVENTS: Withdrawn routes lingering in kernel.
// PREVENTS: a withdrawn prefix the kernel still forwards.
func TestNetlinkIntegration_RemoveRoute(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)

		// Add then withdraw.
		f.processEvent(makeSysribPayload([]incomingChange{
			addChange("10.99.1.0/24", "127.0.0.1"),
		}))
		require.Len(t, zeRoutes(t, h), 1)
		requireForwards(t, h, "10.99.1.1", "10.99.1.0/24", "127.0.0.1", rtprotZE)

		f.processEvent(makeSysribPayload([]incomingChange{
			withdrawChange("10.99.1.0/24"),
		}))

		assert.Empty(t, zeRoutes(t, h), "route should be removed from kernel")
		requireUnroutable(t, h, "10.99.1.1")
	})
}

// VALIDATES: AC-10 -- sysrib/best-change with action "update" replaces route.
// PREVENTS: Stale next-hops in kernel after route update.
// PREVENTS: a replace that leaves the kernel forwarding through the old next-hop.
func TestNetlinkIntegration_ReplaceRoute(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)

		// Add initial route.
		f.processEvent(makeSysribPayload([]incomingChange{
			addChange("10.99.2.0/24", "127.0.0.1"),
		}))
		requireForwards(t, h, "10.99.2.1", "10.99.2.0/24", "127.0.0.1", rtprotZE)

		// Update the next-hop to a second loopback address. Both resolve over
		// lo, so the only thing the replace changes is the gateway the kernel
		// forwards through.
		f.processEvent(makeSysribPayload([]incomingChange{
			updateChange("10.99.2.0/24", "127.0.0.2", "static"),
		}))

		routes := zeRoutes(t, h)
		require.Len(t, routes, 1, "should still have exactly 1 route after replace")
		assert.Equal(t, "10.99.2.0/24", routes[0].Dst.String())
		requireForwards(t, h, "10.99.2.1", "10.99.2.0/24", "127.0.0.2", rtprotZE)
	})
}

// VALIDATES: AC-15 -- startup sweep lists existing ze routes.
// PREVENTS: stale-mark-then-sweep failing to find routes.
// PREVENTS: a sweep list naming a route the kernel does not forward through.
func TestNetlinkIntegration_ListZeRoutes(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)

		// Install two routes.
		f.processEvent(makeSysribPayload([]incomingChange{
			addChange("10.99.3.0/24", "127.0.0.1"),
			addChange("10.99.4.0/24", "127.0.0.1"),
		}))

		// List via backend.
		listed, err := backend.listZeRoutes()
		require.NoError(t, err)
		assert.Len(t, listed, 2)

		prefixes := map[string]bool{}
		for _, r := range listed {
			prefixes[r.prefix] = true
		}
		assert.True(t, prefixes["10.99.3.0/24"])
		assert.True(t, prefixes["10.99.4.0/24"])

		// Each listed route is the one the kernel forwards its prefix through.
		requireForwards(t, h, "10.99.3.1", "10.99.3.0/24", "127.0.0.1", rtprotZE)
		requireForwards(t, h, "10.99.4.1", "10.99.4.0/24", "127.0.0.1", rtprotZE)
	})
}

// VALIDATES: AC-15 -- startup sweep marks stale, refreshes matching, sweeps rest.
// PREVENTS: Crash recovery leaving stale routes in kernel.
// PREVENTS: a sweep that removes the refreshed route's forwarding, or leaves
// the stale prefix forwarding.
func TestNetlinkIntegration_StartupSweep(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)

		// Pre-install two ze routes directly (simulating routes from a previous run).
		require.NoError(t, backend.addRoute("10.99.5.0/24", "127.0.0.1"))
		require.NoError(t, backend.addRoute("10.99.6.0/24", "127.0.0.1"))
		requireForwards(t, h, "10.99.5.1", "10.99.5.0/24", "127.0.0.1", rtprotZE)
		requireForwards(t, h, "10.99.6.1", "10.99.6.0/24", "127.0.0.1", rtprotZE)

		// Create a fresh fib-kernel (simulating restart).
		f := newFIBKernel(backend)

		// Startup sweep finds both routes.
		stale := f.startupSweep()
		require.Len(t, stale, 2)

		// Simulate sysrib refreshing only one route.
		// Use "update" (replaceRoute) because the route already exists in kernel
		// from the previous run. "add" would fail with EEXIST.
		f.processEvent(makeSysribPayload([]incomingChange{
			updateChange("10.99.5.0/24", "127.0.0.1", "bgp"),
		}))

		// Sweep stale routes.
		f.sweepStale(stale)

		// 10.99.5.0/24 should survive (refreshed), 10.99.6.0/24 should be gone.
		routes := zeRoutes(t, h)
		require.Len(t, routes, 1, "only refreshed route should remain")
		assert.Equal(t, "10.99.5.0/24", routes[0].Dst.String())
		requireForwards(t, h, "10.99.5.1", "10.99.5.0/24", "127.0.0.1", rtprotZE)
		requireUnroutable(t, h, "10.99.6.1")
	})
}

// VALIDATES: P0-8 -- restart recovery only sweeps fib-kernel-owned routes.
// PREVENTS: fib-kernel cleanup deleting routes owned by static, policyroute, or other Ze producers.
// PREVENTS: a sweep after which the kernel stops forwarding through another
// producer's route, or keeps forwarding through the swept one.
func TestNetlinkIntegration_StartupSweepPreservesOtherZeProtocols(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		require.NoError(t, backend.addRoute("10.99.9.0/24", "127.0.0.1"))
		addProtocolRoute(t, h, "10.99.10.0/24", "127.0.0.1", rtproto.Static)
		addProtocolRoute(t, h, "10.99.11.0/24", "127.0.0.1", rtproto.PolicyRoute)
		requireForwards(t, h, "10.99.9.1", "10.99.9.0/24", "127.0.0.1", rtprotZE)
		requireForwards(t, h, "10.99.10.1", "10.99.10.0/24", "127.0.0.1", rtproto.Static)
		requireForwards(t, h, "10.99.11.1", "10.99.11.0/24", "127.0.0.1", rtproto.PolicyRoute)

		f := newFIBKernel(backend)
		stale := f.startupSweep()
		require.Equal(t, map[string]string{"10.99.9.0/24": "127.0.0.1"}, stale)

		f.sweepStale(stale)

		assert.Empty(t, zeRoutes(t, h), "fib-kernel stale route should be removed")
		staticRoutes := routesByProtocol(t, h, rtproto.Static)
		require.Len(t, staticRoutes, 1, "static-owned route must survive fib-kernel sweep")
		assert.Equal(t, "10.99.10.0/24", staticRoutes[0].Dst.String())
		policyRoutes := routesByProtocol(t, h, rtproto.PolicyRoute)
		require.Len(t, policyRoutes, 1, "policyroute-owned route must survive fib-kernel sweep")
		assert.Equal(t, "10.99.11.0/24", policyRoutes[0].Dst.String())

		requireUnroutable(t, h, "10.99.9.1")
		requireForwards(t, h, "10.99.10.1", "10.99.10.0/24", "127.0.0.1", rtproto.Static)
		requireForwards(t, h, "10.99.11.1", "10.99.11.0/24", "127.0.0.1", rtproto.PolicyRoute)
	})
}

// VALIDATES: P0-8 -- flush-on-stop only removes fib-kernel-owned routes.
// PREVENTS: graceful shutdown cleanup deleting static or policyroute producers.
// PREVENTS: a flush after which the kernel stops forwarding through another
// producer's route, or keeps forwarding through the flushed one.
func TestNetlinkIntegration_FlushRoutesPreservesOtherZeProtocols(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)
		f.processEvent(makeSysribPayload([]incomingChange{
			addChange("10.99.12.0/24", "127.0.0.1"),
		}))
		addProtocolRoute(t, h, "10.99.13.0/24", "127.0.0.1", rtproto.Static)
		addProtocolRoute(t, h, "10.99.14.0/24", "127.0.0.1", rtproto.PolicyRoute)
		requireForwards(t, h, "10.99.12.1", "10.99.12.0/24", "127.0.0.1", rtprotZE)
		requireForwards(t, h, "10.99.13.1", "10.99.13.0/24", "127.0.0.1", rtproto.Static)
		requireForwards(t, h, "10.99.14.1", "10.99.14.0/24", "127.0.0.1", rtproto.PolicyRoute)

		f.flushRoutes()

		assert.Empty(t, zeRoutes(t, h), "fib-kernel route should be removed")
		staticRoutes := routesByProtocol(t, h, rtproto.Static)
		require.Len(t, staticRoutes, 1, "static-owned route must survive fib-kernel flush")
		assert.Equal(t, "10.99.13.0/24", staticRoutes[0].Dst.String())
		policyRoutes := routesByProtocol(t, h, rtproto.PolicyRoute)
		require.Len(t, policyRoutes, 1, "policyroute-owned route must survive fib-kernel flush")
		assert.Equal(t, "10.99.14.0/24", policyRoutes[0].Dst.String())

		requireUnroutable(t, h, "10.99.12.1")
		requireForwards(t, h, "10.99.13.1", "10.99.13.0/24", "127.0.0.1", rtproto.Static)
		requireForwards(t, h, "10.99.14.1", "10.99.14.0/24", "127.0.0.1", rtproto.PolicyRoute)
	})
}

// VALIDATES: AC-14 -- flushRoutes removes all ze routes on shutdown.
// PREVENTS: Routes lingering after graceful shutdown with flush-on-stop.
// PREVENTS: a flushed prefix the kernel still forwards.
func TestNetlinkIntegration_FlushRoutes(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)

		f.processEvent(makeSysribPayload([]incomingChange{
			addChange("10.99.7.0/24", "127.0.0.1"),
			addChange("10.99.8.0/24", "127.0.0.1"),
		}))
		require.Len(t, zeRoutes(t, h), 2)
		requireForwards(t, h, "10.99.7.1", "10.99.7.0/24", "127.0.0.1", rtprotZE)
		requireForwards(t, h, "10.99.8.1", "10.99.8.0/24", "127.0.0.1", rtprotZE)

		f.flushRoutes()

		assert.Empty(t, zeRoutes(t, h), "all ze routes should be flushed")
		requireUnroutable(t, h, "10.99.7.1")
		requireUnroutable(t, h, "10.99.8.1")
	})
}

// VALIDATES: a metric-carrying Add takes the richRouteBackend path
// (addRichRoute -> netlink RTA_PRIORITY) and the matching Withdraw still removes
// the route from the kernel, even though delRichRoute sends no RTA_PRIORITY and
// no gateway.
// VALIDATES: the metric the backend writes is the one the kernel ranks by. A
// static route for the same prefix sits at metric 20; the kernel selects the
// fib-kernel route at metric 10 while it is installed, and falls back to the
// static one once the withdraw removes it. The fall-back is the positive
// observation that a bare "route gone" cannot make.
// PREVENTS: test/plugin/forked-route-install-kernel.ci's failure
// "route-remove: 10.99.0.0/24 still in kernel after withdrawal". Every other test
// in this file builds changes with addChange(), which sets no Metric, so
// hasRichFields() is false and they exercise only the PLAIN addRoute/delRoute
// pair. On Linux the real backend is a richRouteBackend, and a forked
// route-install carrying a metric therefore programs the kernel through
// addRichRoute -- an add/delete round-trip nothing covered before this test.
// PREVENTS: a metric that reaches RTA_PRIORITY but loses the kernel's ranking,
// and a withdraw whose key (prefix + protocol) deletes the other owner's route.
func TestNetlinkIntegration_RemoveRichRouteWithMetric(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)
		require.NotNil(t, f.asRichBackend(), "netlink backend must be a richRouteBackend")

		const prefix = "10.99.7.0/24"
		const dest = "10.99.7.1"

		// A competing route from another owner, ranked worse than the one the
		// backend is about to write. The kernel selects it until the fib-kernel
		// route arrives and again once that route is withdrawn.
		addProtocolRouteWithMetric(t, h, prefix, "127.0.0.2", rtproto.Static, 20)
		competing := requireForwards(t, h, dest, prefix, "127.0.0.2", rtproto.Static)
		assert.Equal(t, 20, competing.Priority)

		// The forked-route-install .ci values: distance 110, metric 10.
		// A non-zero Metric alone makes hasRichFields() true, so this Add goes
		// through addRichRoute and lands with RTA_PRIORITY=10.
		f.processEvent(makeSysribPayload([]incomingChange{{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix(prefix),
			NextHop:  netip.MustParseAddr("127.0.0.1"),
			Protocol: "bgp",
			Metric:   10,
		}}))

		routes := zeRoutes(t, h)
		require.Len(t, routes, 1, "metric-carrying Add must program exactly one ze kernel route")
		assert.Equal(t, prefix, routes[0].Dst.String())
		assert.Equal(t, 10, routes[0].Priority, "Metric must reach the kernel as RTA_PRIORITY")

		// The kernel ranks the fib-kernel route above the static one.
		selected := requireForwards(t, h, dest, prefix, "127.0.0.1", rtprotZE)
		assert.Equal(t, 10, selected.Priority, "kernel must select the metric-10 route")

		// sysrib's Withdraw carries only Action+Prefix (recomputeBest's
		// len(protocols)==0 branch), so delRichRoute sends prefix + protocol with
		// no priority and no gateway. The kernel must still match and delete the
		// metric-10 route.
		f.processEvent(makeSysribPayload([]incomingChange{withdrawChange(prefix)}))

		assert.Empty(t, zeRoutes(t, h),
			"withdraw must remove the metric-carrying route; a surviving route means "+
				"delRichRoute's key (prefix+protocol, no priority) did not match the "+
				"route addRichRoute installed")

		// The kernel falls back to the static route, so the withdraw removed the
		// fib-kernel route and only that one.
		fallback := requireForwards(t, h, dest, prefix, "127.0.0.2", rtproto.Static)
		assert.Equal(t, 20, fallback.Priority)
	})
}
