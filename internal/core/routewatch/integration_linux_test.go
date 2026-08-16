// Design: docs/architecture/core-design.md -- routewatch integration tests

//go:build integration && linux

package routewatch

import (
	"net"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/core/rtproto"
)

func withNetNS(t *testing.T, fn func()) {
	t.Helper()
	runtime.LockOSThread()

	origNS, err := netns.Get()
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}

	nsName := sanitizeNSName(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close() //nolint:errcheck // best-effort cleanup
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("restore namespace: %v", restoreErr)
		}
		origNS.Close()            //nolint:errcheck // best-effort cleanup
		newNS.Close()             //nolint:errcheck // best-effort cleanup
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup in test teardown
		runtime.UnlockOSThread()
	})

	fn()
}

func sanitizeNSName(name string) string {
	s := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(name)
	if len(s) > 15 {
		s = s[:15]
	}
	return s
}

func addLoopback(t *testing.T, h *netlink.Handle) {
	t.Helper()
	lo, err := h.LinkByName("lo")
	require.NoError(t, err)
	require.NoError(t, h.LinkSetUp(lo))
}

// eventRecorder collects RouteEvents and lets tests wait for a condition
// instead of sleeping a fixed interval (fixed sleeps flake under QEMU load).
type eventRecorder struct {
	mu     sync.Mutex
	events []RouteEvent
}

func (r *eventRecorder) record(ev RouteEvent) {
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
}

func (r *eventRecorder) snapshot() []RouteEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]RouteEvent(nil), r.events...)
}

// waitFor polls until pred sees a satisfying event list or the deadline
// passes; returns the final snapshot.
func (r *eventRecorder) waitFor(t *testing.T, pred func([]RouteEvent) bool) []RouteEvent {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if snap := r.snapshot(); pred(snap) {
			return snap
		}
		time.Sleep(20 * time.Millisecond)
	}
	return r.snapshot()
}

func hasEvent(events []RouteEvent, prefix string, action Action) bool {
	p := netip.MustParsePrefix(prefix)
	for _, ev := range events {
		if ev.Prefix == p && ev.Action == action {
			return true
		}
	}
	return false
}

func TestIntegration_FanoutFromNetlink(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		w := New()
		rec := &eventRecorder{}
		w.Register(rec.record)

		w.Start(func(err error) {
			t.Logf("watcher error: %v", err)
		})
		defer w.Stop()

		// No settle sleep needed: if the subscription binds after RouteAdd,
		// the ListExisting dump still delivers the route as an add.
		_, cidr, _ := net.ParseCIDR("10.77.0.0/24")
		require.NoError(t, h.RouteAdd(&netlink.Route{
			Dst:      cidr,
			Gw:       net.ParseIP("127.0.0.1"),
			Protocol: 16,
		}))

		events := rec.waitFor(t, func(evs []RouteEvent) bool {
			return hasEvent(evs, "10.77.0.0/24", ActionAdd)
		})
		require.True(t, hasEvent(events, "10.77.0.0/24", ActionAdd),
			"10.77.0.0/24 add event not found in %v", events)

		for _, ev := range events {
			if ev.Prefix == netip.MustParsePrefix("10.77.0.0/24") && ev.Action == ActionAdd {
				assert.Equal(t, 16, ev.Protocol)
				assert.Equal(t, netip.MustParseAddr("127.0.0.1"), ev.NextHop)
				break
			}
		}
	})
}

func TestIntegration_FilterZeOwned(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		w := New()
		rec := &eventRecorder{}
		w.Register(rec.record)

		w.Start(func(err error) {
			t.Logf("watcher error: %v", err)
		})
		defer w.Stop()

		// Ze-owned route first, then a marker route. The kernel delivers
		// in order, so once the marker event arrives the filtered route
		// would already have been delivered if the filter were broken.
		_, zeCidr, _ := net.ParseCIDR("10.88.0.0/24")
		require.NoError(t, h.RouteAdd(&netlink.Route{
			Dst:      zeCidr,
			Gw:       net.ParseIP("127.0.0.1"),
			Protocol: netlink.RouteProtocol(rtproto.FIBKernel),
		}))
		_, markerCidr, _ := net.ParseCIDR("10.89.0.0/24")
		require.NoError(t, h.RouteAdd(&netlink.Route{
			Dst:      markerCidr,
			Gw:       net.ParseIP("127.0.0.1"),
			Protocol: 16,
		}))

		events := rec.waitFor(t, func(evs []RouteEvent) bool {
			return hasEvent(evs, "10.89.0.0/24", ActionAdd)
		})
		require.True(t, hasEvent(events, "10.89.0.0/24", ActionAdd),
			"marker route event not received: %v", events)
		for _, ev := range events {
			if ev.Prefix == netip.MustParsePrefix("10.88.0.0/24") {
				t.Fatalf("Ze-owned route should have been filtered, got: %+v", ev)
			}
		}
	})
}

func TestIntegration_RouteDelete(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		_, cidr, _ := net.ParseCIDR("10.99.0.0/24")
		require.NoError(t, h.RouteAdd(&netlink.Route{
			Dst:      cidr,
			Gw:       net.ParseIP("127.0.0.1"),
			Protocol: 16,
		}))

		w := New()
		rec := &eventRecorder{}
		w.Register(rec.record)

		w.Start(func(err error) {
			t.Logf("watcher error: %v", err)
		})
		defer w.Stop()

		// Wait for the ListExisting dump to deliver the add before
		// deleting; the delete must arrive via the live subscription.
		events := rec.waitFor(t, func(evs []RouteEvent) bool {
			return hasEvent(evs, "10.99.0.0/24", ActionAdd)
		})
		require.True(t, hasEvent(events, "10.99.0.0/24", ActionAdd),
			"expected add from ListExisting, got %v", events)

		require.NoError(t, h.RouteDel(&netlink.Route{
			Dst:      cidr,
			Gw:       net.ParseIP("127.0.0.1"),
			Protocol: 16,
		}))

		events = rec.waitFor(t, func(evs []RouteEvent) bool {
			return hasEvent(evs, "10.99.0.0/24", ActionRemove)
		})

		var addCount, removeCount int
		for _, ev := range events {
			if ev.Prefix == netip.MustParsePrefix("10.99.0.0/24") {
				switch ev.Action {
				case ActionAdd:
					addCount++
				case ActionRemove:
					removeCount++
				}
			}
		}
		assert.Equal(t, 1, addCount, "expected 1 add from ListExisting")
		assert.Equal(t, 1, removeCount, "expected 1 remove from deletion")
	})
}
