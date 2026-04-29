//go:build integration && linux

package fibkernel

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
)

// withNetNS creates an ephemeral network namespace, switches into it,
// runs fn, then restores the original namespace in t.Cleanup.
// Skips the test if CAP_NET_ADMIN is unavailable.
func withNetNS(t *testing.T, fn func()) {
	t.Helper()

	runtime.LockOSThread()

	origNS, err := netns.Get()
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN: cannot get current namespace: %v", err)
	}

	nsName := sanitizeNSName(t.Name())

	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close()
		t.Skipf("requires CAP_NET_ADMIN: cannot create namespace: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("failed to restore original namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
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

// VALIDATES: AC-8 -- sysrib/best-change with action "add" installs route via netlink.
// VALIDATES: AC-16 -- fib-kernel routes use their producer-specific rtm_protocol ID.
// PREVENTS: netlink backend silently failing to program real kernel routes.
func TestNetlinkIntegration_AddRoute(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)

		event := makeTestEvent([]incomingChange{
			{Action: bgptypes.RouteActionAdd, Prefix: "10.99.0.0/24", NextHop: "127.0.0.1", Protocol: "bgp"},
		})
		f.processEvent(event)

		// Verify route exists in kernel with the fib-kernel route owner.
		routes := zeRoutes(t, h)
		require.Len(t, routes, 1, "expected 1 ze route in kernel")
		assert.Equal(t, "10.99.0.0/24", routes[0].Dst.String())
		assert.Equal(t, netlink.RouteProtocol(rtprotZE), routes[0].Protocol)
	})
}

// VALIDATES: AC-9 -- sysrib/best-change with action "withdraw" removes route.
// PREVENTS: Withdrawn routes lingering in kernel.
func TestNetlinkIntegration_RemoveRoute(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)

		// Add then withdraw.
		f.processEvent(makeTestPayload([]incomingChange{
			{Action: bgptypes.RouteActionAdd, Prefix: "10.99.1.0/24", NextHop: "127.0.0.1", Protocol: "bgp"},
		}))
		require.Len(t, zeRoutes(t, h), 1)

		f.processEvent(makeTestPayload([]incomingChange{
			{Action: bgptypes.RouteActionWithdraw, Prefix: "10.99.1.0/24"},
		}))

		assert.Empty(t, zeRoutes(t, h), "route should be removed from kernel")
	})
}

// VALIDATES: AC-10 -- sysrib/best-change with action "update" replaces route.
// PREVENTS: Stale next-hops in kernel after route update.
func TestNetlinkIntegration_ReplaceRoute(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)

		// Add initial route.
		f.processEvent(makeTestPayload([]incomingChange{
			{Action: bgptypes.RouteActionAdd, Prefix: "10.99.2.0/24", NextHop: "127.0.0.1", Protocol: "bgp"},
		}))

		// Update next-hop (still loopback, but verifies replace works).
		f.processEvent(makeTestPayload([]incomingChange{
			{Action: bgptypes.RouteActionUpdate, Prefix: "10.99.2.0/24", NextHop: "127.0.0.1", Protocol: "static"},
		}))

		routes := zeRoutes(t, h)
		require.Len(t, routes, 1, "should still have exactly 1 route after replace")
		assert.Equal(t, "10.99.2.0/24", routes[0].Dst.String())
	})
}

// VALIDATES: AC-15 -- startup sweep lists existing ze routes.
// PREVENTS: stale-mark-then-sweep failing to find routes.
func TestNetlinkIntegration_ListZeRoutes(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)

		// Install two routes.
		f.processEvent(makeTestPayload([]incomingChange{
			{Action: bgptypes.RouteActionAdd, Prefix: "10.99.3.0/24", NextHop: "127.0.0.1", Protocol: "bgp"},
			{Action: bgptypes.RouteActionAdd, Prefix: "10.99.4.0/24", NextHop: "127.0.0.1", Protocol: "bgp"},
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
	})
}

// VALIDATES: AC-15 -- startup sweep marks stale, refreshes matching, sweeps rest.
// PREVENTS: Crash recovery leaving stale routes in kernel.
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

		// Create a fresh fib-kernel (simulating restart).
		f := newFIBKernel(backend)

		// Startup sweep finds both routes.
		stale := f.startupSweep()
		require.Len(t, stale, 2)

		// Simulate sysrib refreshing only one route.
		// Use "update" (replaceRoute) because the route already exists in kernel
		// from the previous run. "add" would fail with EEXIST.
		f.processEvent(makeTestPayload([]incomingChange{
			{Action: bgptypes.RouteActionUpdate, Prefix: "10.99.5.0/24", NextHop: "127.0.0.1", Protocol: "bgp"},
		}))

		// Sweep stale routes.
		f.sweepStale(stale)

		// 10.99.5.0/24 should survive (refreshed), 10.99.6.0/24 should be gone.
		routes := zeRoutes(t, h)
		require.Len(t, routes, 1, "only refreshed route should remain")
		assert.Equal(t, "10.99.5.0/24", routes[0].Dst.String())
	})
}

// VALIDATES: AC-14 -- flushRoutes removes all ze routes on shutdown.
// PREVENTS: Routes lingering after graceful shutdown with flush-on-stop.
func TestNetlinkIntegration_FlushRoutes(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		backend := newTestBackend(h)
		f := newFIBKernel(backend)

		f.processEvent(makeTestPayload([]incomingChange{
			{Action: bgptypes.RouteActionAdd, Prefix: "10.99.7.0/24", NextHop: "127.0.0.1", Protocol: "bgp"},
			{Action: bgptypes.RouteActionAdd, Prefix: "10.99.8.0/24", NextHop: "127.0.0.1", Protocol: "bgp"},
		}))
		require.Len(t, zeRoutes(t, h), 2)

		f.flushRoutes()

		assert.Empty(t, zeRoutes(t, h), "all ze routes should be flushed")
	})
}

// makeTestPayload builds a (sysrib, best-change) JSON payload for testing.
func makeTestPayload(changes []incomingChange) string {
	batch := incomingBatch{
		Family:  "ipv4/unicast",
		Changes: changes,
	}
	data, _ := json.Marshal(batch)
	return string(data)
}
