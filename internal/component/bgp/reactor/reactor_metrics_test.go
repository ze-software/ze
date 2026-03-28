package reactor

import (
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Spy types for metrics testing ---
// These capture values so tests can inspect what updatePeriodicMetrics set.

type spyGauge struct {
	mu    sync.Mutex
	value float64
	set   bool
}

func (g *spyGauge) Set(v float64)  { g.mu.Lock(); g.value = v; g.set = true; g.mu.Unlock() }
func (g *spyGauge) Inc()           { g.mu.Lock(); g.value++; g.set = true; g.mu.Unlock() }
func (g *spyGauge) Dec()           { g.mu.Lock(); g.value--; g.set = true; g.mu.Unlock() }
func (g *spyGauge) Add(v float64)  { g.mu.Lock(); g.value += v; g.set = true; g.mu.Unlock() }
func (g *spyGauge) Value() float64 { g.mu.Lock(); defer g.mu.Unlock(); return g.value }
func (g *spyGauge) WasSet() bool   { g.mu.Lock(); defer g.mu.Unlock(); return g.set }

type spyCounter struct {
	mu    sync.Mutex
	value float64
}

func (c *spyCounter) Inc()           { c.mu.Lock(); c.value++; c.mu.Unlock() }
func (c *spyCounter) Add(v float64)  { c.mu.Lock(); c.value += v; c.mu.Unlock() }
func (c *spyCounter) Value() float64 { c.mu.Lock(); defer c.mu.Unlock(); return c.value }

type spyGaugeVec struct {
	mu     sync.Mutex
	gauges map[string]*spyGauge
}

func newSpyGaugeVec() *spyGaugeVec {
	return &spyGaugeVec{gauges: make(map[string]*spyGauge)}
}

func (v *spyGaugeVec) With(labels ...string) metrics.Gauge {
	key := strings.Join(labels, ",")
	v.mu.Lock()
	defer v.mu.Unlock()
	if g, ok := v.gauges[key]; ok {
		return g
	}
	g := &spyGauge{}
	v.gauges[key] = g
	return g
}

func (v *spyGaugeVec) Delete(...string) bool { return true }

func (v *spyGaugeVec) get(labels ...string) *spyGauge {
	key := strings.Join(labels, ",")
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.gauges[key]
}

type spyCounterVec struct {
	mu       sync.Mutex
	counters map[string]*spyCounter
}

func newSpyCounterVec() *spyCounterVec {
	return &spyCounterVec{counters: make(map[string]*spyCounter)}
}

func (v *spyCounterVec) With(labels ...string) metrics.Counter {
	key := strings.Join(labels, ",")
	v.mu.Lock()
	defer v.mu.Unlock()
	if c, ok := v.counters[key]; ok {
		return c
	}
	c := &spyCounter{}
	v.counters[key] = c
	return c
}

func (v *spyCounterVec) Delete(...string) bool { return true }

func (v *spyCounterVec) get(labels ...string) *spyCounter {
	key := strings.Join(labels, ",")
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.counters[key]
}

type spyRegistry struct {
	mu          sync.Mutex
	gauges      map[string]*spyGauge
	counters    map[string]*spyCounter
	gaugeVecs   map[string]*spyGaugeVec
	counterVecs map[string]*spyCounterVec
}

func newSpyRegistry() *spyRegistry {
	return &spyRegistry{
		gauges:      make(map[string]*spyGauge),
		counters:    make(map[string]*spyCounter),
		gaugeVecs:   make(map[string]*spyGaugeVec),
		counterVecs: make(map[string]*spyCounterVec),
	}
}

func (r *spyRegistry) Counter(name, _ string) metrics.Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &spyCounter{}
	r.counters[name] = c
	return c
}

func (r *spyRegistry) Gauge(name, _ string) metrics.Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[name]; ok {
		return g
	}
	g := &spyGauge{}
	r.gauges[name] = g
	return g
}

func (r *spyRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cv, ok := r.counterVecs[name]; ok {
		return cv
	}
	cv := newSpyCounterVec()
	r.counterVecs[name] = cv
	return cv
}

func (r *spyRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gv, ok := r.gaugeVecs[name]; ok {
		return gv
	}
	gv := newSpyGaugeVec()
	r.gaugeVecs[name] = gv
	return gv
}

func (r *spyRegistry) Histogram(string, string, []float64) metrics.Histogram {
	return spyNopHistogram{}
}

func (r *spyRegistry) HistogramVec(string, string, []float64, []string) metrics.HistogramVec {
	return spyNopHistogramVec{}
}

type spyNopHistogram struct{}

func (spyNopHistogram) Observe(float64) {}

type spyNopHistogramVec struct{}

func (spyNopHistogramVec) With(...string) metrics.Histogram { return spyNopHistogram{} }
func (spyNopHistogramVec) Delete(...string) bool            { return false }

func (r *spyRegistry) gauge(name string) *spyGauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gauges[name]
}

func (r *spyRegistry) gaugeVec(name string) *spyGaugeVec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gaugeVecs[name]
}

// --- Tests ---

// TestUpdatePeriodicMetrics_SetsOverflowGauges verifies that updatePeriodicMetrics
// reads forward pool state and sets the corresponding Prometheus gauges.
//
// VALIDATES: Phase 2 deferred — updatePeriodicMetrics() unit test with mock registry.
// PREVENTS: Overflow gauge values never being written after pool state changes.
func TestUpdatePeriodicMetrics_SetsOverflowGauges(t *testing.T) {
	reg := newSpyRegistry()
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 8, overflowPoolSize: 100})
	defer pool.Stop()

	// Pre-populate source stats: source A has 80% forwarded, 20% overflowed.
	for range 8 {
		pool.RecordForwarded(netip.MustParseAddr("10.0.0.1"))
	}
	for range 2 {
		pool.RecordOverflowed(netip.MustParseAddr("10.0.0.1"))
	}

	// Create a worker so WorkerCount > 0 and OverflowDepths has an entry.
	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("192.168.1.1:179")}, fwdItem{})
	require.Eventually(t, func() bool {
		return pool.WorkerCount() == 1
	}, time.Second, time.Millisecond)

	r := &Reactor{
		fwdPool:       pool,
		rmetrics:      initReactorMetrics(reg, "test", "1.2.3.4", "65000"),
		clock:         clock.RealClock{},
		startTime:     time.Now().Add(-5 * time.Second),
		recentUpdates: NewRecentUpdateCache(100),
	}

	r.updatePeriodicMetrics()

	// AC-18: pool used ratio gauge was set.
	poolRatio := reg.gauge("ze_bgp_pool_used_ratio")
	require.NotNil(t, poolRatio, "ze_bgp_pool_used_ratio should be registered")
	assert.True(t, poolRatio.WasSet(), "pool used ratio should have been set")
	// Pool has 100 tokens, none acquired (dispatching to non-blocking handler returns tokens fast),
	// so ratio is 0 or close to 0.
	assert.InDelta(t, 0.0, poolRatio.Value(), 0.1, "pool ratio should be near 0 with idle pool")

	// Forward workers active was set.
	workers := reg.gauge("ze_forward_workers_active")
	require.NotNil(t, workers, "ze_forward_workers_active should be registered")
	assert.True(t, workers.WasSet(), "workers active should have been set")
	assert.Equal(t, 1.0, workers.Value(), "should have 1 active worker")

	// AC-16: per-source overflow ratio gauge was set for source A.
	overflowRatioVec := reg.gaugeVec("ze_bgp_overflow_ratio")
	require.NotNil(t, overflowRatioVec, "ze_bgp_overflow_ratio should be registered")
	srcGauge := overflowRatioVec.get("10.0.0.1")
	require.NotNil(t, srcGauge, "overflow ratio should have been set for source 10.0.0.1")
	assert.InDelta(t, 0.2, srcGauge.Value(), 0.001, "source A: 2/10 = 0.2")

	// AC-17: per-destination overflow depth gauge was set.
	overflowItemsVec := reg.gaugeVec("ze_bgp_overflow_items")
	require.NotNil(t, overflowItemsVec, "ze_bgp_overflow_items should be registered")
	destGauge := overflowItemsVec.get("192.168.1.1")
	require.NotNil(t, destGauge, "overflow items should have been set for destination 192.168.1.1")
	// Worker channel is not blocked, so overflow buffer should be empty.
	assert.Equal(t, 0.0, destGauge.Value(), "overflow depth should be 0 for non-congested peer")

	// Uptime was set (>0 since startTime is 5s ago).
	uptime := reg.gauge("ze_uptime_seconds")
	require.NotNil(t, uptime, "ze_uptime_seconds should be registered")
	assert.True(t, uptime.WasSet(), "uptime should have been set")
	assert.Greater(t, uptime.Value(), 0.0, "uptime should be positive")
}

// TestForwardDispatch_RecordForwarded_UpdatesMetrics verifies the end-to-end wiring
// from TryDispatch -> RecordForwarded -> updatePeriodicMetrics -> overflow_ratio gauge.
// This proves the forward path records source stats and the metrics loop picks them up.
//
// VALIDATES: Phase 2 deferred — ForwardUpdate -> RecordForwarded/RecordOverflowed wiring.
// PREVENTS: Source stats recorded but never read by metrics, or vice versa.
func TestForwardDispatch_RecordForwarded_UpdatesMetrics(t *testing.T) {
	reg := newSpyRegistry()
	blocker := make(chan struct{})

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-blocker
	}, fwdPoolConfig{chanSize: 4, overflowPoolSize: 100})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("192.168.1.1:179")}

	// First dispatch: worker starts and blocks in handler on <-blocker.
	ok := pool.TryDispatch(key, fwdItem{})
	require.True(t, ok, "first TryDispatch should succeed")
	pool.RecordForwarded(netip.MustParseAddr("10.0.0.1"))

	require.Eventually(t, func() bool {
		return pool.WorkerCount() == 1
	}, time.Second, time.Millisecond)
	// Wait for worker to consume from channel and block in handler.
	time.Sleep(10 * time.Millisecond)

	// Fill the channel (chanSize=4): worker is blocked, so these stay queued.
	for range 4 {
		ok = pool.TryDispatch(key, fwdItem{})
		require.True(t, ok, "TryDispatch should succeed while channel has space")
	}
	pool.RecordForwarded(netip.MustParseAddr("10.0.0.1"))

	// Channel is full (4/4). TryDispatch should fail.
	ok = pool.TryDispatch(key, fwdItem{})
	require.False(t, ok, "TryDispatch should fail (channel full)")
	ok = pool.DispatchOverflow(key, fwdItem{})
	require.True(t, ok, "DispatchOverflow should succeed")
	pool.RecordOverflowed(netip.MustParseAddr("10.0.0.1"))

	// Verify source stats: 2 forwarded, 1 overflowed = 1/3 ratio.
	ratios := pool.SourceOverflowRatios()
	assert.InDelta(t, 1.0/3.0, ratios["10.0.0.1"], 0.001)

	// Wire metrics and call updatePeriodicMetrics.
	r := &Reactor{
		fwdPool:       pool,
		rmetrics:      initReactorMetrics(reg, "test", "1.2.3.4", "65000"),
		clock:         clock.RealClock{},
		startTime:     time.Now(),
		recentUpdates: NewRecentUpdateCache(100),
	}

	r.updatePeriodicMetrics()

	// Verify the overflow ratio gauge reflects the source stats.
	overflowRatioVec := reg.gaugeVec("ze_bgp_overflow_ratio")
	require.NotNil(t, overflowRatioVec)
	srcGauge := overflowRatioVec.get("10.0.0.1")
	require.NotNil(t, srcGauge, "overflow ratio should be set for source 10.0.0.1")
	assert.InDelta(t, 1.0/3.0, srcGauge.Value(), 0.001,
		"overflow ratio gauge should reflect 1 overflowed / 3 total")

	// Verify overflow items gauge shows the overflow buffer depth.
	overflowItemsVec := reg.gaugeVec("ze_bgp_overflow_items")
	require.NotNil(t, overflowItemsVec)
	destGauge := overflowItemsVec.get("192.168.1.1")
	require.NotNil(t, destGauge, "overflow items should be set for destination")
	assert.Equal(t, 1.0, destGauge.Value(), "overflow buffer should have 1 item")

	close(blocker)
}

// TestMetricNames_MatchRegistration verifies that initReactorMetrics registers
// all expected Prometheus metric names, including overflow-specific ones.
//
// VALIDATES: Phase 2 deferred — exact Prometheus metric names match registration.
// PREVENTS: Metric name typo or missing registration breaking dashboards.
func TestMetricNames_MatchRegistration(t *testing.T) {
	reg := newSpyRegistry()
	_ = initReactorMetrics(reg, "1.0.0", "1.2.3.4", "65000")

	// Verify all expected scalar gauge names are registered.
	expectedGauges := []string{
		"ze_peers_configured",
		"ze_uptime_seconds",
		"ze_cache_entries",
		"ze_forward_workers_active",
		"ze_bgp_pool_used_ratio",
	}
	for _, name := range expectedGauges {
		g := reg.gauge(name)
		assert.NotNil(t, g, "expected gauge %q to be registered", name)
	}

	// Verify all expected gauge vec names are registered.
	expectedGaugeVecs := []string{
		"ze_info",
		"ze_peer_state",
		"ze_bgp_overflow_items",
		"ze_bgp_overflow_ratio",
		"ze_peer_session_duration_seconds",
		"ze_bgp_prefix_count",
		"ze_bgp_prefix_maximum",
		"ze_bgp_prefix_warning",
		"ze_bgp_prefix_warning_exceeded",
		"ze_bgp_prefix_ratio",
		"ze_bgp_prefix_stale",
	}
	for _, name := range expectedGaugeVecs {
		gv := reg.gaugeVec(name)
		assert.NotNil(t, gv, "expected gauge vec %q to be registered", name)
	}

	// Verify all expected scalar counter names are registered.
	expectedCounters := []string{
		"ze_config_reloads_total",
		"ze_peers_added_total",
		"ze_peers_removed_total",
	}
	for _, name := range expectedCounters {
		reg.mu.Lock()
		_, ok := reg.counters[name]
		reg.mu.Unlock()
		assert.True(t, ok, "expected counter %q to be registered", name)
	}

	// Verify all expected counter vec names are registered.
	expectedCounterVecs := []string{
		"ze_peer_messages_received_total",
		"ze_peer_messages_sent_total",
		"ze_peer_sessions_established_total",
		"ze_peer_session_flaps_total",
		"ze_peer_state_transitions_total",
		"ze_peer_notifications_sent_total",
		"ze_peer_notifications_received_total",
		"ze_forward_congestion_events_total",
		"ze_forward_congestion_resumed_total",
		"ze_config_reload_errors_total",
		"ze_wire_bytes_received_total",
		"ze_wire_bytes_sent_total",
		"ze_wire_read_errors_total",
		"ze_wire_write_errors_total",
		"ze_bgp_prefix_maximum_exceeded_total",
		"ze_bgp_prefix_teardown_total",
	}
	for _, name := range expectedCounterVecs {
		reg.mu.Lock()
		_, ok := reg.counterVecs[name]
		reg.mu.Unlock()
		assert.True(t, ok, "expected counter vec %q to be registered", name)
	}
}
