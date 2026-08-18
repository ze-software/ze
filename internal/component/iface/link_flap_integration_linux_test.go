//go:build integration && linux

// Design: docs/architecture/iface/management.md -- carrier events are queued per interface
// Related: link_queue.go -- the queue, subscribeLinkEvents and applyLinkEvent
// Related: register.go -- the route handlers applyLinkEvent calls

package iface

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// dispatchingBus is an EventBus that delivers to its subscribers on the
// caller's goroutine, which is what EmitEngineEvent
// (internal/component/plugin/server/engine_event.go) does. collectingBus
// records events and delivers to nobody, so it cannot drive a subscriber.
type dispatchingBus struct {
	mu   sync.Mutex
	subs map[string][]func(any)
}

func newDispatchingBus() *dispatchingBus {
	return &dispatchingBus{subs: map[string][]func(any){}}
}

func (b *dispatchingBus) key(namespace, eventType string) string {
	return namespace + "/" + eventType
}

func (b *dispatchingBus) Emit(namespace, eventType string, payload any) (int, error) {
	key := b.key(namespace, eventType)
	b.mu.Lock()
	handlers := make([]func(any), len(b.subs[key]))
	copy(handlers, b.subs[key])
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (b *dispatchingBus) Subscribe(namespace, eventType string, fn func(any)) func() {
	key := b.key(namespace, eventType)
	b.mu.Lock()
	b.subs[key] = append(b.subs[key], fn)
	b.mu.Unlock()
	return func() {}
}

// startMonitorOnBus starts the active backend's link monitor against bus and
// stops it on cleanup. startTestMonitor takes the collecting bus, which
// delivers to no subscriber, so it cannot drive this wiring.
func startMonitorOnBus(t *testing.T, bus *dispatchingBus) {
	t.Helper()
	b := GetBackend()
	if b == nil {
		t.Fatal("no iface backend loaded")
	}
	if err := b.StartMonitor(bus); err != nil {
		t.Fatalf("StartMonitor: %v", err)
	}
	t.Cleanup(func() { b.StopMonitor() })
}

// pendingCarrier returns the carrier state the queue holds for ifaceName, and
// whether it holds one at all.
func pendingCarrier(q *linkEventQueue, ifaceName string) (bool, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	value, ok := q.pending[linkEventKey{class: linkEventCarrier, ifaceName: ifaceName}]
	return value.present, ok
}

// waitForPendingCarrier waits until the monitor has delivered the state the
// link settled in. It is the deterministic replacement for a sleep: the flap is
// finished when the state the kernel ended in is the state the queue holds.
func waitForPendingCarrier(t *testing.T, q *linkEventQueue, ifaceName string, want bool) {
	t.Helper()
	deadline := time.Now().Add(monitorEventTimeout)
	for time.Now().Before(deadline) {
		if got, ok := pendingCarrier(q, ifaceName); ok && got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, ok := pendingCarrier(q, ifaceName)
	t.Fatalf("the queue holds carrier=%v (present=%v) for %s, want carrier=%v", got, ok, ifaceName, want)
}

// flapCarrier drives the peer end of a veth pair admin down and up, which moves
// the local end's CARRIER while it stays admin up, so its address and its
// connected route survive the flap. transitions is the number of carrier
// changes; the last one leaves the carrier at endUp.
func flapCarrier(t *testing.T, peer string, transitions int, endUp bool) {
	t.Helper()
	// Walk backwards from the state the caller wants to end in.
	up := endUp
	states := make([]bool, transitions)
	for i := transitions - 1; i >= 0; i-- {
		states[i] = up
		up = !up
	}
	for _, want := range states {
		var err error
		if want {
			err = SetAdminUp(peer)
		} else {
			err = SetAdminDown(peer)
		}
		if err != nil {
			t.Fatalf("set %s admin state: %v", peer, err)
		}
	}
}

// metricOf returns the metric the kernel holds the default route via gateway
// at, or -1 when no such route exists.
func metricOf(t *testing.T, ifaceName, gateway string) int {
	t.Helper()
	routes, err := ListRoutes(ifaceName, "0.0.0.0/0")
	if err != nil {
		t.Fatalf("list routes on %s: %v", ifaceName, err)
	}
	for _, r := range routes {
		if r.Gateway == gateway {
			return r.Metric
		}
	}
	return -1
}

// TestIntegrationLinkFlapDuringCommitKeepsTheRouteMetric is the end-to-end
// proof, from a real kernel carrier change to a real kernel route.
//
// VALIDATES: AC-5 -- a link that flaps far more times than the replaced 16-deep
// channel could hold, while the queue is not being drained, leaves the default
// route at the metric the LIVE carrier calls for. The monitor's read loop, the
// subscribers subscribeLinkEvents registers and the work applyLinkEvent does
// are all the production ones: runEngine registers the same subscribers on the
// same queue, and its worker calls the same function.
// PREVENTS: the unrecoverable loss. The subscribers used to push into a 16-deep
// channel with a `default:` branch that dropped on a full buffer, and a config
// commit was when it filled: the commit holds dhcpMu across DHCP client stop
// and start, and every apply takes that lock. The burst below is three times
// the channel's depth and it is ODD, so the 16 events that would have fit end
// on the OPPOSITE state from the one the link settles in. The dropped design
// therefore leaves the route at the wrong metric, and no later event repairs
// it: the handlers are idempotent by routeMetricState.
//
// Not draining IS the commit: a worker blocked on dhcpMu is a worker that does
// not drain. The drain runs on the test's own goroutine because withNetNS
// switches the namespace of ONE locked OS thread, and a netlink call from any
// other goroutine would program the wrong namespace.
func TestIntegrationLinkFlapDuringCommitKeepsTheRouteMetric(t *testing.T) {
	withNetNS(t, func() {
		const local = "zeflap0"
		const peer = "zeflap1"
		const gateway = "198.51.100.2"
		const metric = 254
		// Odd, and more than three times the 16 the replaced channel held.
		const transitions = 51

		createVethForTest(t, local, peer)
		if err := AddAddress(local, "198.51.100.1/24"); err != nil {
			t.Fatalf("add address: %v", err)
		}
		if err := SetAdminUp(local); err != nil {
			t.Fatalf("set %s up: %v", local, err)
		}
		if err := SetAdminUp(peer); err != nil {
			t.Fatalf("set %s up: %v", peer, err)
		}
		if err := AddRoute(local, "0.0.0.0/0", gateway, metric, rtproto.Iface); err != nil {
			t.Fatalf("install the base-metric default route: %v", err)
		}
		if got := metricOf(t, local, gateway); got != metric {
			t.Fatalf("default route via %s is at metric %d, want %d", gateway, got, metric)
		}

		// The production wiring: subscribers that only push, and one apply that
		// does every route call.
		active := map[dhcpUnitKey]dhcpEntry{
			{ifaceName: local, unit: "0"}: {
				gateway:     gateway,
				params:      dhcpParams{routePriority: metric},
				metricState: routeMetricBase,
			},
		}
		routers := map[routerKey]routerEntry{}
		queue := newLinkEventQueue(slog.Default())
		bus := newDispatchingBus()
		subscribeLinkEvents(bus, queue, nil)
		drain := func() {
			queue.applyAll(func(key linkEventKey, value linkEventValue) {
				applyLinkEvent(key, value, active, routers, map[string]int{}, slog.Default())
			})
		}

		startMonitorOnBus(t, bus)
		time.Sleep(200 * time.Millisecond) // the monitor's initial dump
		drain()                            // discard what the dump reported

		// A burst that ends carrier-down deprioritizes the route.
		flapCarrier(t, peer, transitions, false)
		waitForPendingCarrier(t, queue, local, false)
		drain()
		if got := metricOf(t, local, gateway); got != metric+deprioritizedMetric {
			t.Fatalf("after a burst that ended carrier-down the route via %s sits at metric %d, want %d",
				gateway, got, metric+deprioritizedMetric)
		}

		// A burst that ends carrier-up restores the base metric.
		flapCarrier(t, peer, transitions, true)
		waitForPendingCarrier(t, queue, local, true)
		drain()
		if got := metricOf(t, local, gateway); got != metric {
			t.Fatalf("after a burst that ended carrier-up the route via %s sits at metric %d, want %d",
				gateway, got, metric)
		}
	})
}
