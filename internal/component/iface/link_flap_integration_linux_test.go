//go:build linux

// Design: docs/architecture/iface/management.md -- carrier events are queued per interface
// Related: link_queue.go -- the queue, subscribeLinkEvents and applyLinkEvent
// Related: register.go -- the route handlers applyLinkEvent calls

package iface

import (
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/events"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/test/userns"
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

// kernelCarrier reads the carrier state the kernel reports for ifaceName now,
// through carrierFromInterfaces, the reading the monitor's isLinkUp and the
// carrier resync share.
func kernelCarrier(t *testing.T, ifaceName string) bool {
	t.Helper()
	info, err := GetInterface(ifaceName)
	if err != nil {
		t.Fatalf("read %s from the kernel: %v", ifaceName, err)
	}
	if info == nil {
		t.Fatalf("the kernel reports no interface %s", ifaceName)
	}
	up, ok := carrierFromInterfaces([]InterfaceInfo{*info})[ifaceName]
	if !ok {
		t.Fatalf("the kernel answered for %q when asked for %s", info.Name, ifaceName)
	}
	return up
}

// markerAnnounced returns a channel that closes when the monitor announces, on
// bus, the creation of the link named marker.
func markerAnnounced(t *testing.T, bus *dispatchingBus, marker string) <-chan struct{} {
	t.Helper()
	announced := make(chan struct{})
	var once sync.Once
	bus.Subscribe(ifaceevents.Namespace, ifaceevents.EventCreated, events.AsString(func(data string) {
		var ev struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			t.Errorf("decode the %s event %q: %v", ifaceevents.EventCreated, data, err)
			return
		}
		if ev.Name == marker {
			once.Do(func() { close(announced) })
		}
	}))
	return announced
}

// awaitCarrierSettled returns once the monitor has pushed every carrier event
// the kernel sent for ifaceName before its carrier settled at want, and fails
// unless the queue then holds want. It never drains: the burst stays in the
// queue until the caller's one drain, because an undrained queue is what the
// test stands for.
//
// Polling the queue for want cannot tell the end of an alternating burst from
// its middle: the queue passes through want on every second event, so the poll
// can return while later events are still on their way, and a drain then
// applies a state the burst went on to leave. Two facts close that gap.
//
// The kernel's own state is the first. flapCarrier returns with the carrier at
// want, but the operstate the monitor reads, and the RTM_NEWLINK that carries
// it, are written later by the kernel's linkwatch, in one pass under RTNL that
// reads the carrier as it is then. Once the operstate reads want, the last
// notification sent for ifaceName reports want, and so does every one after it.
//
// The marker link is the second. Creating it takes RTNL after that read, so its
// RTM_NEWLINK follows every notification sent before the read on the monitor's
// one link subscription. The monitor handles link updates in order and emits
// each on the dispatching bus before it reads the next, so when the marker's
// creation event arrives the queue holds the last state sent before the read,
// and nothing still behind it reports anything but want.
func awaitCarrierSettled(t *testing.T, bus *dispatchingBus, q *linkEventQueue, ifaceName, marker string, want bool) {
	t.Helper()
	deadline := time.Now().Add(monitorEventTimeout)
	for kernelCarrier(t, ifaceName) != want {
		if !time.Now().Before(deadline) {
			t.Fatalf("the kernel still holds carrier=%v for %s %v after the burst, want carrier=%v",
				!want, ifaceName, monitorEventTimeout, want)
		}
		time.Sleep(10 * time.Millisecond)
	}

	announced := markerAnnounced(t, bus, marker)
	createDummyForTest(t, marker)
	select {
	case <-announced:
	case <-time.After(monitorEventTimeout):
		t.Fatalf("the monitor did not announce the marker link %s within %v", marker, monitorEventTimeout)
	}

	if got, ok := pendingCarrier(q, ifaceName); !ok || got != want {
		t.Fatalf("the queue holds carrier=%v (queued=%v) for %s, want carrier=%v", got, ok, ifaceName, want)
	}
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
	if !userns.Enter(t) {
		return
	}
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
		awaitCarrierSettled(t, bus, queue, local, "zemark0", false)
		drain()
		if got := metricOf(t, local, gateway); got != metric+deprioritizedMetric {
			t.Fatalf("after a burst that ended carrier-down the route via %s sits at metric %d, want %d",
				gateway, got, metric+deprioritizedMetric)
		}

		// A burst that ends carrier-up restores the base metric.
		flapCarrier(t, peer, transitions, true)
		awaitCarrierSettled(t, bus, queue, local, "zemark1", true)
		drain()
		if got := metricOf(t, local, gateway); got != metric {
			t.Fatalf("after a burst that ended carrier-up the route via %s sits at metric %d, want %d",
				gateway, got, metric)
		}
	})
}
