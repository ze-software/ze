// Design: docs/architecture/core-design.md -- routewatch integration tests

//go:build linux

package routewatch

import (
	"net"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/test/userns"
)

func withNetNS(t *testing.T, fn func()) {
	t.Helper()
	runtime.LockOSThread()

	origNS, err := netns.Get()
	if err != nil {
		t.Fatalf("get the current network namespace: %v", err)
	}

	nsName := sanitizeNSName(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close() //nolint:errcheck // best-effort cleanup
		t.Fatalf("create network namespace %s: %v", nsName, err)
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
	if !userns.Enter(t) {
		return
	}
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
		defer func() {
			w.Stop()
			w.Wait()
		}()

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

func TestIntegration_DeliverZeOwned(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		addLoopback(t, h)

		w := New()
		rec := &eventRecorder{}
		w.Register(rec.record)
		w.Start(func(err error) { t.Logf("watcher error: %v", err) })
		defer func() {
			w.Stop()
			w.Wait()
		}()

		_, cidr, err := net.ParseCIDR("10.88.0.0/24")
		require.NoError(t, err)
		route := &netlink.Route{
			Dst: cidr, Gw: net.ParseIP("127.0.0.1"),
			Protocol: netlink.RouteProtocol(rtproto.FIBKernel),
			Table:    100, Priority: 37,
		}
		require.NoError(t, h.RouteAdd(route))
		events := rec.waitFor(t, func(evs []RouteEvent) bool {
			return hasEvent(evs, "10.88.0.0/24", ActionAdd)
		})
		require.True(t, hasEvent(events, "10.88.0.0/24", ActionAdd))
		require.NoError(t, h.RouteDel(route))
		events = rec.waitFor(t, func(evs []RouteEvent) bool {
			return hasEvent(evs, "10.88.0.0/24", ActionRemove)
		})
		require.True(t, hasEvent(events, "10.88.0.0/24", ActionRemove))
		for _, ev := range events {
			if ev.Prefix == netip.MustParsePrefix("10.88.0.0/24") {
				require.Equal(t, rtproto.FIBKernel, ev.Protocol)
				require.Equal(t, uint32(100), ev.TableID)
				require.Equal(t, uint32(37), ev.Metric)
			}
		}
	})
}

func TestIntegration_RouteDelete(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
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
		defer func() {
			w.Stop()
			w.Wait()
		}()

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
				default:
					panic("BUG: routewatch: invalid action")
				}
			}
		}
		assert.Equal(t, 1, addCount, "expected 1 add from ListExisting")
		assert.Equal(t, 1, removeCount, "expected 1 remove from deletion")
	})
}

// receiveGoroutineFrame names the netlink library's receive goroutine, the one
// that calls the error callback and closes the update channel. Its closer
// goroutine is routeSubscribeAt.func1 and may still be returning when Wait
// returns; only the receive goroutine is owed to Wait.
const receiveGoroutineFrame = "netlink.routeSubscribeAt.func2"

// receiveGoroutinesAlive counts live goroutines whose stack holds the netlink
// receive goroutine's frame.
func receiveGoroutinesAlive() int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	alive := 0
	for g := range strings.SplitSeq(string(buf[:n]), "\n\n") {
		if strings.Contains(g, receiveGoroutineFrame) {
			alive++
		}
	}
	return alive
}

// stopCycles is how many watchers TestIntegration_StopWaitEndsTheReceiveGoroutine
// starts and stops. Without the stop drain the receive goroutine still exits on
// its own soon after Wait, and one dump right after Wait caught it in 8 of 10
// runs. Twenty cycles leave a missing drain no realistic way to pass.
const stopCycles = 20

// TestIntegration_StopWaitEndsTheReceiveGoroutine proves the stop contract
// both halves of the fix carry: once Stop is called the error callback is
// never called (the receive error our own socket close causes is not a
// failure), and once Wait returns the netlink receive goroutine has exited.
//
// Method: in its own namespace, stopCycles times, a live subscription proven
// up by the ListExisting event for one route, then Stop and Wait. Any callback
// after the stopped flag is set is recorded and fails the test; the goroutine
// dump taken right after Wait must hold no receive goroutine. Without the
// suppression the close reports "Receive failed"; without the drain the
// receive goroutine outlives Wait.
func TestIntegration_StopWaitEndsTheReceiveGoroutine(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withNetNS(t, func() {
		h, err := netlink.NewHandle()
		require.NoError(t, err)
		defer h.Close()
		addLoopback(t, h)

		_, cidr, err := net.ParseCIDR("10.66.0.0/24")
		require.NoError(t, err)
		require.NoError(t, h.RouteAdd(&netlink.Route{
			Dst: cidr, Gw: net.ParseIP("127.0.0.1"), Protocol: 16,
		}))

		for cycle := range stopCycles {
			stopAndWaitOnce(t, cycle)
		}
	})
}

// stopAndWaitOnce runs one start, live, Stop, Wait cycle and asserts the stop
// contract on it.
func stopAndWaitOnce(t *testing.T, cycle int) {
	t.Helper()
	var stopped atomic.Bool
	var lateMu sync.Mutex
	var late []string
	w := New()
	rec := &eventRecorder{}
	w.Register(rec.record)
	w.Start(func(err error) {
		if !stopped.Load() {
			t.Logf("cycle %d: watcher error before Stop: %v", cycle, err)
			return
		}
		lateMu.Lock()
		late = append(late, err.Error())
		lateMu.Unlock()
	})
	rec.waitFor(t, func(evs []RouteEvent) bool {
		return hasEvent(evs, "10.66.0.0/24", ActionAdd)
	})
	require.Positive(t, receiveGoroutinesAlive(), "cycle %d: the subscription is live, so its receive goroutine must be found", cycle)

	stopped.Store(true)
	w.Stop()
	w.Wait()

	require.Zero(t, receiveGoroutinesAlive(), "cycle %d: the netlink receive goroutine outlived Wait", cycle)
	lateMu.Lock()
	defer lateMu.Unlock()
	require.Empty(t, late, "cycle %d: the error callback ran after Stop", cycle)
}
