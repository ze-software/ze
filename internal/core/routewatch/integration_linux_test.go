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

	"codeberg.org/thomas-mangin/ze/internal/core/rtproto"
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
		origNS.Close()
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("restore namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
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

func TestIntegration_FanoutFromNetlink(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		w := New()

		var mu sync.Mutex
		var events []RouteEvent
		w.Register(func(ev RouteEvent) {
			mu.Lock()
			events = append(events, ev)
			mu.Unlock()
		})

		w.Start(func(err error) {
			t.Logf("watcher error: %v", err)
		})
		defer w.Stop()

		time.Sleep(100 * time.Millisecond)

		_, cidr, _ := net.ParseCIDR("10.77.0.0/24")
		require.NoError(t, h.RouteAdd(&netlink.Route{
			Dst:      cidr,
			Gw:       net.ParseIP("127.0.0.1"),
			Protocol: 16,
		}))

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		require.NotEmpty(t, events, "expected at least one RouteEvent from netlink")

		var found bool
		for _, ev := range events {
			if ev.Prefix == netip.MustParsePrefix("10.77.0.0/24") && ev.Action == ActionAdd {
				assert.Equal(t, 16, ev.Protocol)
				assert.Equal(t, netip.MustParseAddr("127.0.0.1"), ev.NextHop)
				found = true
				break
			}
		}
		assert.True(t, found, "10.77.0.0/24 add event not found in %v", events)
	})
}

func TestIntegration_FilterZeOwned(t *testing.T) {
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()

		addLoopback(t, h)

		w := New()

		var mu sync.Mutex
		var events []RouteEvent
		w.Register(func(ev RouteEvent) {
			mu.Lock()
			events = append(events, ev)
			mu.Unlock()
		})

		w.Start(func(err error) {
			t.Logf("watcher error: %v", err)
		})
		defer w.Stop()

		time.Sleep(100 * time.Millisecond)

		_, cidr, _ := net.ParseCIDR("10.88.0.0/24")
		require.NoError(t, h.RouteAdd(&netlink.Route{
			Dst:      cidr,
			Gw:       net.ParseIP("127.0.0.1"),
			Protocol: netlink.RouteProtocol(rtproto.FIBKernel),
		}))

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
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

		var mu sync.Mutex
		var events []RouteEvent
		w.Register(func(ev RouteEvent) {
			mu.Lock()
			events = append(events, ev)
			mu.Unlock()
		})

		w.Start(func(err error) {
			t.Logf("watcher error: %v", err)
		})
		defer w.Stop()

		time.Sleep(200 * time.Millisecond)

		require.NoError(t, h.RouteDel(&netlink.Route{
			Dst:      cidr,
			Gw:       net.ParseIP("127.0.0.1"),
			Protocol: 16,
		}))

		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		var addCount, removeCount int
		for _, ev := range events {
			if ev.Prefix == netip.MustParsePrefix("10.99.0.0/24") {
				if ev.Action == ActionAdd {
					addCount++
				} else if ev.Action == ActionRemove {
					removeCount++
				}
			}
		}
		assert.Equal(t, 1, addCount, "expected 1 add from ListExisting")
		assert.Equal(t, 1, removeCount, "expected 1 remove from deletion")
	})
}
