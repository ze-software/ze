package fibvpp

import (
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// testGauge records Set calls for assertions.
type testGauge struct {
	mu  sync.Mutex
	val float64
}

func (g *testGauge) Set(v float64) { g.mu.Lock(); g.val = v; g.mu.Unlock() }
func (g *testGauge) Inc()          { g.mu.Lock(); g.val++; g.mu.Unlock() }
func (g *testGauge) Dec()          { g.mu.Lock(); g.val--; g.mu.Unlock() }
func (g *testGauge) Add(v float64) { g.mu.Lock(); g.val += v; g.mu.Unlock() }
func (g *testGauge) get() float64  { g.mu.Lock(); defer g.mu.Unlock(); return g.val }

// testCounter records Inc/Add calls for assertions.
type testCounter struct {
	mu  sync.Mutex
	val float64
}

func (c *testCounter) Inc()          { c.mu.Lock(); c.val++; c.mu.Unlock() }
func (c *testCounter) Add(v float64) { c.mu.Lock(); c.val += v; c.mu.Unlock() }
func (c *testCounter) get() float64  { c.mu.Lock(); defer c.mu.Unlock(); return c.val }

// fibTestRegistry returns collectable metrics for fibvpp test assertions.
type fibTestRegistry struct {
	gauges   map[string]*testGauge
	counters map[string]*testCounter
}

func newFibTestRegistry() *fibTestRegistry {
	return &fibTestRegistry{
		gauges:   make(map[string]*testGauge),
		counters: make(map[string]*testCounter),
	}
}

func (r *fibTestRegistry) Counter(name, _ string) metrics.Counter {
	c := &testCounter{}
	r.counters[name] = c
	return c
}

func (r *fibTestRegistry) Gauge(name, _ string) metrics.Gauge {
	g := &testGauge{}
	r.gauges[name] = g
	return g
}

func (r *fibTestRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec { return nil }
func (r *fibTestRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec     { return nil }
func (r *fibTestRegistry) Histogram(_, _ string, _ []float64) metrics.Histogram     { return nil }
func (r *fibTestRegistry) HistogramVec(_, _ string, _ []float64, _ []string) metrics.HistogramVec {
	return nil
}

func makeBatch(changes ...incomingChange) *incomingBatch {
	return &incomingBatch{
		Family:  family.IPv4Unicast,
		Changes: changes,
	}
}

// VALIDATES: AC-9 — ze_fibvpp_routes_installed gauge present.
// VALIDATES: AC-10 — ze_fibvpp_route_installs_total counter present.
// PREVENTS: fibvpp metrics not tracking route changes.
func TestFibRouteCount(t *testing.T) {
	reg := newFibTestRegistry()
	SetMetricsRegistry(reg)
	defer fibVPPMetricsPtr.Store(nil)

	fib := newFibVPP(&mockBackend{})

	// Add two routes.
	fib.processEvent(makeBatch(
		incomingChange{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1"},
		incomingChange{Action: "add", Prefix: "10.0.1.0/24", NextHop: "192.168.1.1"},
	))

	installed := reg.gauges["ze_fibvpp_routes_installed"]
	if installed == nil {
		t.Fatal("ze_fibvpp_routes_installed not registered")
	}
	if got := installed.get(); got != 2 {
		t.Errorf("routes_installed after 2 adds: got %v, want 2", got)
	}

	installs := reg.counters["ze_fibvpp_route_installs_total"]
	if installs == nil {
		t.Fatal("ze_fibvpp_route_installs_total not registered")
	}
	if got := installs.get(); got != 2 {
		t.Errorf("route_installs_total after 2 adds: got %v, want 2", got)
	}

	// Update one route.
	fib.processEvent(makeBatch(
		incomingChange{Action: "update", Prefix: "10.0.0.0/24", NextHop: "192.168.1.2"},
	))

	updates := reg.counters["ze_fibvpp_route_updates_total"]
	if updates == nil {
		t.Fatal("ze_fibvpp_route_updates_total not registered")
	}
	if got := updates.get(); got != 1 {
		t.Errorf("route_updates_total after update: got %v, want 1", got)
	}
	// Installed count unchanged after update.
	if got := installed.get(); got != 2 {
		t.Errorf("routes_installed after update: got %v, want 2", got)
	}

	// Withdraw one route.
	fib.processEvent(makeBatch(
		incomingChange{Action: "withdraw", Prefix: "10.0.1.0/24"},
	))

	removals := reg.counters["ze_fibvpp_route_removals_total"]
	if removals == nil {
		t.Fatal("ze_fibvpp_route_removals_total not registered")
	}
	if got := removals.get(); got != 1 {
		t.Errorf("route_removals_total after withdraw: got %v, want 1", got)
	}
	if got := installed.get(); got != 1 {
		t.Errorf("routes_installed after withdraw: got %v, want 1", got)
	}

	// Flush all.
	fib.flushRoutes()
	if got := installed.get(); got != 0 {
		t.Errorf("routes_installed after flush: got %v, want 0", got)
	}
}
