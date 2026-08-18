// Design: docs/architecture/iface/management.md -- carrier events are queued per interface
// Related: link_queue.go -- the queue, the worker and the carrier resync
// Related: register.go -- the subscribers that push and the handlers the worker calls

package iface

import (
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// capturingCounterVec is a metrics.CounterVec that keeps its per-label totals
// in memory. capturingGaugeRegistry (device_owner_test.go) hands one out for
// every CounterVec bindMetricsRegistry creates.
type capturingCounterVec struct {
	mu       sync.Mutex
	counters map[string]*capturingCounter
}

func newCapturingCounterVec() *capturingCounterVec {
	return &capturingCounterVec{counters: map[string]*capturingCounter{}}
}

func (v *capturingCounterVec) With(labels ...string) metrics.Counter {
	v.mu.Lock()
	defer v.mu.Unlock()
	key := labels[0]
	if _, ok := v.counters[key]; !ok {
		v.counters[key] = &capturingCounter{}
	}
	return v.counters[key]
}

func (v *capturingCounterVec) Delete(labels ...string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, ok := v.counters[labels[0]]
	delete(v.counters, labels[0])
	return ok
}

// value returns the total recorded for key, or -1 when the series was never
// created. -1 rather than 0 so a test tells "never incremented" from "zero".
func (v *capturingCounterVec) value(key string) float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	if c, ok := v.counters[key]; ok {
		return c.get()
	}
	return -1
}

type capturingCounter struct {
	mu sync.Mutex
	v  float64
}

func (c *capturingCounter) Inc() {
	c.mu.Lock()
	c.v++
	c.mu.Unlock()
}

func (c *capturingCounter) Add(d float64) {
	c.mu.Lock()
	c.v += d
	c.mu.Unlock()
}

func (c *capturingCounter) get() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.v
}

// bindCapturingMetrics binds a metrics registry whose counters the test can
// read back, and unbinds it afterwards.
func bindCapturingMetrics(t *testing.T) *capturingGaugeRegistry {
	t.Helper()
	reg := newCapturingGaugeRegistry()
	bindMetricsRegistry(reg)
	t.Cleanup(func() { ifaceMetricsPtr.Store(nil) })
	return reg
}

// blockedWorker drives a queue whose worker is stuck inside its first apply,
// which is what a config commit does to it: the commit holds dhcpMu across DHCP
// client stop and start, and every apply takes that lock.
type blockedWorker struct {
	queue    *linkEventQueue
	entered  chan struct{}
	release  chan struct{}
	mu       sync.Mutex
	applied  []linkEventEntry
	blockeOn sync.Once
}

// newBlockedWorker starts a queue, pushes one carrier event to get the worker
// into an apply, and waits there. The caller MUST call finish.
func newBlockedWorker(t *testing.T, firstIface string) *blockedWorker {
	t.Helper()
	w := &blockedWorker{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	w.queue = newLinkEventQueue(slog.Default())
	w.queue.start(func(key linkEventKey, value linkEventValue) {
		w.blockeOn.Do(func() {
			close(w.entered)
			<-w.release
		})
		w.mu.Lock()
		w.applied = append(w.applied, linkEventEntry{key: key, value: value})
		w.mu.Unlock()
	})
	w.queue.pushCarrier(firstIface, false)
	<-w.entered
	return w
}

// finish releases the worker and waits for it to apply everything queued.
func (w *blockedWorker) finish() []linkEventEntry {
	close(w.release)
	w.queue.stop()
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]linkEventEntry(nil), w.applied...)
}

// TestExtractedWorkerStartsAndStops proves the queue and its worker have a
// lifecycle a caller drives, which is what makes every other test in this file
// possible.
//
// VALIDATES: R-1 -- the subscriber, the queue and the worker moved out of the
// runEngine closure without losing their start and stop. stop applies what is
// still pending before the worker leaves, exactly as the closed channel it
// replaced did.
// PREVENTS: a worker that outlives runEngine, and an event accepted after the
// last drain and never applied. `select` does not prefer the wake channel over
// the stop channel when both are ready, so a push that races the stop reaches a
// handler only because stop drains once more on its way out.
func TestExtractedWorkerStartsAndStops(t *testing.T) {
	var mu sync.Mutex
	var applied []string
	q := newLinkEventQueue(slog.Default())
	q.start(func(key linkEventKey, _ linkEventValue) {
		mu.Lock()
		applied = append(applied, key.ifaceName)
		mu.Unlock()
	})

	q.pushCarrier("eth0", true)
	q.stop()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"eth0"}, applied, "the worker must apply what was pushed before it stopped")

	// A second stop is a no-op rather than a close of a closed channel.
	q.stop()
}

// TestLinkQueueKeepsFinalStateUnderPressure is the defect this spec exists for.
//
// VALIDATES: AC-1 and AC-2 -- with the worker stuck in an apply, a down
// followed by an up for one interface leaves the worker acting on UP, and a
// burst far larger than the 16-deep channel this replaced loses no interface's
// final state.
// PREVENTS: the measured loss. The EventUp and EventDown subscribers pushed
// into a 16-deep channel with a `default:` branch that dropped the event, and a
// config commit is when that buffer fills, because the commit holds dhcpMu
// across DHCP client stop and start while every apply takes it. A dropped UP
// after an applied DOWN left the DHCP default route at base + 1024 for as long
// as the link stayed up: handleLinkUp is idempotent by routeMetricState, which
// makes it safe against a duplicate event and helpless against a missing one,
// and nothing else reads live carrier state.
func TestLinkQueueKeepsFinalStateUnderPressure(t *testing.T) {
	const interfaces = 64
	const transitionsEach = 7

	w := newBlockedWorker(t, "eth0")

	// eth0 is the interface the worker is already stuck applying: its DOWN is
	// in flight, and this UP must still be the state it ends on.
	w.queue.pushCarrier("eth0", true)

	// Every other interface flaps far more times than the old 16-deep channel
	// could hold. transitionsEach is odd, so each one ends UP.
	for i := range interfaces {
		name := "zeq" + strconv.Itoa(i)
		for step := range transitionsEach {
			w.queue.pushCarrier(name, step%2 == 0)
		}
	}

	applied := w.finish()

	final := make(map[string]bool, interfaces+1)
	for _, entry := range applied {
		require.Equal(t, linkEventCarrier, entry.key.class)
		final[entry.key.ifaceName] = entry.value.present
	}

	require.Len(t, final, interfaces+1,
		"every interface that flapped must reach the worker; a missing one is an unrecoverable lost carrier state")
	assert.True(t, final["eth0"],
		"the UP that arrived while the worker was blocked is the state eth0 ended in")
	for i := range interfaces {
		name := "zeq" + strconv.Itoa(i)
		assert.True(t, final[name], "%s ended UP and the worker must act on UP", name)
	}
}

// TestLinkQueueCoalesceCounted proves the condition that used to be silent is
// now countable and named.
//
// VALIDATES: AC-3 -- an event superseded before the worker takes it increments
// ze_iface_link_events_coalesced_total for its interface.
// PREVENTS: the invisibility. The drop it replaced had no counter, no log line
// and no doctor check; the only operator-visible symptom was a default route
// sitting at a deprioritized metric with the link up.
func TestLinkQueueCoalesceCounted(t *testing.T) {
	reg := bindCapturingMetrics(t)

	// The worker drained the DOWN before it blocked, so the queue is empty and
	// the first push below supersedes nothing. The two after it each find the
	// previous one still pending.
	w := newBlockedWorker(t, "eth0")
	w.queue.pushCarrier("eth0", true)
	w.queue.pushCarrier("eth0", false)
	w.queue.pushCarrier("eth0", true)
	// One push for an interface nothing is pending for supersedes nothing.
	w.queue.pushCarrier("eth1", true)
	w.finish()

	counter := reg.counterVecs["ze_iface_link_events_coalesced_total"]
	require.NotNil(t, counter, "bindMetricsRegistry must create the coalesced-events counter")
	assert.Equal(t, float64(2), counter.value("eth0"),
		"two eth0 events were superseded before the worker took one")
	assert.Equal(t, float64(-1), counter.value("eth1"),
		"a first push for an interface supersedes nothing and must not raise a series")
}

// TestLinkQueueAppliesKeysInArrivalOrder pins the order the worker sees.
//
// VALIDATES: a key keeps the position of its FIRST push, so a router event that
// arrived before a carrier event is applied before it. Coalescing changes the
// VALUE at a position, never the position.
// PREVENTS: reordering a router-discovered behind a carrier-down, which would
// install an IPv6 default route at the base metric on a link the worker had
// already deprioritized.
func TestLinkQueueAppliesKeysInArrivalOrder(t *testing.T) {
	w := newBlockedWorker(t, "eth0")

	w.queue.pushRouter(RouterEventPayload{Name: "eth1", RouterIP: "fe80::1"}, `{"name":"eth1","router-ip":"fe80::1"}`, true)
	w.queue.pushCarrier("eth1", false)
	// eth1's router entry is pushed again: it keeps its earlier position.
	w.queue.pushRouter(RouterEventPayload{Name: "eth1", RouterIP: "fe80::1"}, `{"name":"eth1","router-ip":"fe80::1"}`, false)

	applied := w.finish()

	require.Len(t, applied, 3)
	assert.Equal(t, linkEventKey{class: linkEventCarrier, ifaceName: "eth0"}, applied[0].key)
	assert.Equal(t, linkEventKey{class: linkEventRouter, ifaceName: "eth1", routerIP: "fe80::1"}, applied[1].key)
	assert.False(t, applied[1].value.present, "the router entry carries the state of its LAST push")
	assert.Equal(t, linkEventKey{class: linkEventCarrier, ifaceName: "eth1"}, applied[2].key)
}

// TestRouterEventDoesNotBlockTheMonitorLoop proves the second half of the
// defect is gone.
//
// VALIDATES: AC-4 -- a router-discovered or router-lost event hands off, so the
// emitter's goroutine returns while the lock a config apply holds is still
// held. The lock in this test stands for dhcpMu, which the apply closure in
// register.go takes and which reconcileDHCP holds across DHCP client stop and
// start. The two routers on one interface stay independent, and each keeps only
// the state it ended in.
// PREVENTS: the stall. The router subscribers called handleRouterDiscovered and
// handleRouterLost under dhcpMu inline, and an event handler runs synchronously
// on the emitter's goroutine (EmitEngineEvent,
// internal/component/plugin/server/engine_event.go), which for these events is
// the netlink monitor's own read loop. During a commit that loop blocked, its
// 64-deep subscription channel stopped draining, and the kernel-side queue
// overflowed one layer further from anything that could report it.
//
// The timing half is what discriminates: an inline lock makes the push below
// wait for applyRelease, and the deadline fires instead.
func TestRouterEventDoesNotBlockTheMonitorLoop(t *testing.T) {
	// applyLock stands for dhcpMu, and a config apply is holding it.
	var applyLock sync.Mutex
	applyLock.Lock()

	var mu sync.Mutex
	var applied []linkEventEntry
	q := newLinkEventQueue(slog.Default())
	q.start(func(key linkEventKey, value linkEventValue) {
		applyLock.Lock()
		defer applyLock.Unlock()
		mu.Lock()
		applied = append(applied, linkEventEntry{key: key, value: value})
		mu.Unlock()
	})

	// The monitor's read loop, with the commit still holding the lock.
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		for step := range 200 {
			q.pushRouter(RouterEventPayload{Name: "eth1", RouterIP: "fe80::1"},
				`{"name":"eth1","router-ip":"fe80::1"}`, step%2 == 0)
			q.pushRouter(RouterEventPayload{Name: "eth1", RouterIP: "fe80::2"},
				`{"name":"eth1","router-ip":"fe80::2"}`, step%2 == 1)
		}
	}()

	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		applyLock.Unlock()
		t.Fatal("the router event handlers blocked the emitter goroutine for the whole of the config apply")
	}

	applyLock.Unlock()
	q.stop()

	mu.Lock()
	defer mu.Unlock()
	routers := make(map[string]bool)
	for _, entry := range applied {
		require.Equal(t, linkEventRouter, entry.key.class)
		routers[entry.key.routerIP] = entry.value.present
	}
	require.Len(t, routers, 2, "two routers on one interface are two subjects, not one")
	assert.False(t, routers["fe80::1"], "fe80::1 ended lost")
	assert.True(t, routers["fe80::2"], "fe80::2 ended discovered")
}

// TestCarrierResyncRepairsAContradictedRouteMetric proves ze reads live carrier
// state after the fact, which nothing did before.
//
// VALIDATES: AC-6 -- with no carrier event to carry the truth, the resync moves
// a route whose acted-on metric contradicts the live carrier, and leaves an
// interface that agrees alone. The rate tracker already dumps the interface
// list every second, so this costs no extra netlink call.
// PREVENTS: a permanent divergence. A failed AddRoute leaves routeMetricUnknown
// and the route at neither metric, and a device that was already down when its
// DHCP client started never sends a transition; without a second reader the
// route stays where it is until an event that may never come.
func TestCarrierResyncRepairsAContradictedRouteMetric(t *testing.T) {
	t.Run("a deprioritized route on a live link is restored", func(t *testing.T) {
		fb := setupFakeBackendForTest(t)
		key := dhcpUnitKey{ifaceName: "eth0", unit: "0"}
		active := map[dhcpUnitKey]dhcpEntry{key: {
			gateway:     "192.0.2.1",
			params:      dhcpParams{routePriority: 254},
			metricState: routeMetricDeprioritized,
		}}

		repaired := resyncCarrierState(map[string]bool{"eth0": true}, active, map[routerKey]routerEntry{}, slog.Default())

		assert.Equal(t, 1, repaired)
		require.Len(t, fb.routeAdds, 1)
		assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.0.2.1", 254, rtproto.Iface}, fb.routeAdds[0])
		assert.Equal(t, routeMetricBase, active[key].metricState)
	})

	t.Run("a base-metric route on a dead link is deprioritized", func(t *testing.T) {
		fb := setupFakeBackendForTest(t)
		key := routerKey{ifaceName: "eth0", routerIP: "fe80::1"}
		routers := map[routerKey]routerEntry{key: {metric: 5, metricState: routeMetricBase}}

		repaired := resyncCarrierState(map[string]bool{"eth0": false}, map[dhcpUnitKey]dhcpEntry{}, routers, slog.Default())

		assert.Equal(t, 1, repaired)
		require.Len(t, fb.routeAdds, 1)
		assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5 + deprioritizedMetric, rtproto.Iface}, fb.routeAdds[0])
		assert.Equal(t, routeMetricDeprioritized, routers[key].metricState)
	})

	t.Run("agreement and not-knowing are both left alone", func(t *testing.T) {
		fb := setupFakeBackendForTest(t)
		active := map[dhcpUnitKey]dhcpEntry{
			// Agrees with the live carrier.
			{ifaceName: "eth0", unit: "0"}: {gateway: "192.0.2.1", params: dhcpParams{routePriority: 254}, metricState: routeMetricBase},
			// A lease has just landed, so ze does not know where the route is.
			// Moving it here would re-install it on every tick (R-3).
			{ifaceName: "eth1", unit: "0"}: {gateway: "192.0.2.2", params: dhcpParams{routePriority: 254}, metricState: routeMetricUnknown},
			// The device is gone, and its routes went with it.
			{ifaceName: "eth2", unit: "0"}: {gateway: "192.0.2.3", params: dhcpParams{routePriority: 254}, metricState: routeMetricBase},
		}

		repaired := resyncCarrierState(map[string]bool{"eth0": true, "eth1": true}, active, map[routerKey]routerEntry{}, slog.Default())

		assert.Equal(t, 0, repaired)
		assert.Empty(t, fb.routeAdds, "the resync must move nothing when nothing contradicts live carrier")
		assert.Empty(t, fb.routeRemoves)
	})

	t.Run("a repair is counted and names the interface", func(t *testing.T) {
		setupFakeBackendForTest(t)
		reg := bindCapturingMetrics(t)
		key := dhcpUnitKey{ifaceName: "eth0", unit: "0"}
		active := map[dhcpUnitKey]dhcpEntry{key: {
			gateway:     "192.0.2.1",
			params:      dhcpParams{routePriority: 254},
			metricState: routeMetricDeprioritized,
		}}

		resyncCarrierState(map[string]bool{"eth0": true}, active, map[routerKey]routerEntry{}, slog.Default())

		counter := reg.counterVecs["ze_iface_carrier_resyncs_total"]
		require.NotNil(t, counter, "bindMetricsRegistry must create the carrier-resync counter")
		assert.Equal(t, float64(1), counter.value("eth0"))
	})
}

// TestCarrierFromInterfacesReadsTheSameStateTheMonitorEmits ties the resync's
// notion of "up" to the monitor's.
//
// VALIDATES: carrierFromInterfaces agrees with isLinkUp
// (internal/plugins/iface/netlink/monitor_linux.go) through the state string
// linkToInfo writes, spelled either case.
// PREVENTS: a resync that fights the event stream, moving a route back on every
// tick because the two disagree about what up means.
func TestCarrierFromInterfacesReadsTheSameStateTheMonitorEmits(t *testing.T) {
	carrier := carrierFromInterfaces([]InterfaceInfo{
		{Name: "eth0", State: "up"},
		{Name: "eth1", State: "UP"},
		{Name: "eth2", State: "down"},
		{Name: "eth3", State: ""},
	})
	assert.Equal(t, map[string]bool{"eth0": true, "eth1": true, "eth2": false, "eth3": false}, carrier)
}
