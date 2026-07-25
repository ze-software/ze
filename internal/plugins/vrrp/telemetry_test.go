package vrrp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
)

// These tests exercise the REAL telemetry path (recordTransition / clearMetrics /
// publishStateChange and the emitStateChange executor that drives all three).
// Every other test in the package stubs the emitState dep (instance_test.go), so
// without this file the ze_vrrp_* series and the state-change event could sit
// dead and no test would fail -- the exact R-3 "dead counter" hazard spec-vrrp-5
// warns about (AC-11, AC-12).

// --- capturing metrics registry -----------------------------------------------

type capGauge struct{ v float64 }

func (g *capGauge) Set(v float64) { g.v = v }
func (g *capGauge) Inc()          { g.v++ }
func (g *capGauge) Dec()          { g.v-- }
func (g *capGauge) Add(d float64) { g.v += d }

type capCounter struct{ v float64 }

func (c *capCounter) Inc()          { c.v++ }
func (c *capCounter) Add(d float64) { c.v += d }

type capGaugeVec struct{ g map[string]*capGauge }

func (v *capGaugeVec) With(labels ...string) metrics.Gauge {
	k := strings.Join(labels, "|")
	if _, ok := v.g[k]; !ok {
		v.g[k] = &capGauge{}
	}
	return v.g[k]
}

func (v *capGaugeVec) Delete(labels ...string) bool {
	k := strings.Join(labels, "|")
	_, ok := v.g[k]
	delete(v.g, k)
	return ok
}

type capCounterVec struct{ c map[string]*capCounter }

func (v *capCounterVec) With(labels ...string) metrics.Counter {
	k := strings.Join(labels, "|")
	if _, ok := v.c[k]; !ok {
		v.c[k] = &capCounter{}
	}
	return v.c[k]
}

func (v *capCounterVec) Delete(labels ...string) bool {
	k := strings.Join(labels, "|")
	_, ok := v.c[k]
	delete(v.c, k)
	return ok
}

type capRegistry struct {
	stateVec *capGaugeVec
	transVec *capCounterVec
}

func newCapRegistry() *capRegistry {
	return &capRegistry{
		stateVec: &capGaugeVec{g: map[string]*capGauge{}},
		transVec: &capCounterVec{c: map[string]*capCounter{}},
	}
}

func (r *capRegistry) Counter(string, string) metrics.Counter { return &capCounter{} }
func (r *capRegistry) Gauge(string, string) metrics.Gauge     { return &capGauge{} }
func (r *capRegistry) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	if name == "ze_vrrp_transitions_total" {
		return r.transVec
	}
	return &capCounterVec{c: map[string]*capCounter{}}
}

func (r *capRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	if name == "ze_vrrp_state" {
		return r.stateVec
	}
	return &capGaugeVec{g: map[string]*capGauge{}}
}

func (r *capRegistry) Histogram(string, string, []float64) metrics.Histogram { return nil }
func (r *capRegistry) HistogramVec(string, string, []float64, []string) metrics.HistogramVec {
	return nil
}

// single returns the sole gauge/counter value in a vec, or fails if the count is
// not exactly one (the tests each drive a single virtual router).
func (v *capGaugeVec) single(t *testing.T) float64 {
	t.Helper()
	if len(v.g) != 1 {
		t.Fatalf("state gauge has %d series, want 1: %v", len(v.g), v.g)
	}
	for _, g := range v.g {
		return g.v
	}
	return -1
}

func (v *capCounterVec) total(t *testing.T) float64 {
	t.Helper()
	var sum float64
	for _, c := range v.c {
		sum += c.v
	}
	return sum
}

// --- capturing event bus ------------------------------------------------------

type capturedEvent struct {
	namespace string
	eventType string
	payload   any
}

type capBus struct{ events []capturedEvent }

func (b *capBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.events = append(b.events, capturedEvent{namespace, eventType, payload})
	return 0, nil
}

func (b *capBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

// resetTelemetry clears the package-global metric/event pointers between tests.
func resetTelemetry(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		metricsPtr.Store(nil)
		eventBusPtr.Store(nil)
	})
}

// --- tests --------------------------------------------------------------------

// TestRecordTransitionDrivesStateAndCounter proves the state gauge tracks the
// current state and the transition counter increments on every transition, so
// neither can silently sit at zero.
//
// VALIDATES: AC-11 (metrics incremented, no dead counters).
// PREVENTS: R-3 -- ze_vrrp_state / ze_vrrp_transitions_total defined but never
// updated (the holo digest bug 9/10 the telemetry split exists to avoid).
func TestRecordTransitionDrivesStateAndCounter(t *testing.T) {
	resetTelemetry(t)
	reg := newCapRegistry()
	setMetricsRegistry(reg)
	spec := testSpec()

	recordTransition(spec, fsm.StateBackup)
	if got := reg.stateVec.single(t); got != stateValue(fsm.StateBackup) {
		t.Errorf("state gauge = %v, want %v (backup)", got, stateValue(fsm.StateBackup))
	}
	recordTransition(spec, fsm.StateMaster)
	if got := reg.stateVec.single(t); got != stateValue(fsm.StateMaster) {
		t.Errorf("state gauge = %v, want %v (master)", got, stateValue(fsm.StateMaster))
	}
	recordTransition(spec, fsm.StateBackup)
	// Three transitions recorded across the counter's (to)-partitioned series.
	if got := reg.transVec.total(t); got != 3 {
		t.Errorf("transitions total = %v, want 3", got)
	}
}

// TestClearMetricsDropsStateSeries proves a torn-down group stops reporting a
// stale state (clearMetrics deletes its gauge series).
//
// VALIDATES: AC-11 (a deleted group stops reporting).
// PREVENTS: a stale ze_vrrp_state series pinned at its last value after teardown.
func TestClearMetricsDropsStateSeries(t *testing.T) {
	resetTelemetry(t)
	reg := newCapRegistry()
	setMetricsRegistry(reg)
	spec := testSpec()

	recordTransition(spec, fsm.StateMaster)
	if len(reg.stateVec.g) != 1 {
		t.Fatalf("precondition: want 1 state series, got %d", len(reg.stateVec.g))
	}
	clearMetrics(spec)
	if len(reg.stateVec.g) != 0 {
		t.Errorf("clearMetrics left %d state series, want 0", len(reg.stateVec.g))
	}
}

// TestEmitStateChangeRecordsMetricsAndEmitsEvent exercises the real
// emitStateChange executor (the one register.go wires into the FSM) end to end:
// it must both increment the metrics AND publish the typed state-change event.
// The rest of the suite stubs this dep, so this is the only coverage of the live
// path.
//
// VALIDATES: AC-11 (metrics incremented via the live executor) + AC-12 (typed
// state-change event emitted with the router's identity).
// PREVENTS: the wired emitStateChange path silently doing nothing because every
// other test replaces it with a fake (instance_test.go).
func TestEmitStateChangeRecordsMetricsAndEmitsEvent(t *testing.T) {
	resetTelemetry(t)
	reg := newCapRegistry()
	setMetricsRegistry(reg)
	bus := &capBus{}
	setEventBus(bus)
	spec := testSpec()

	emitStateChange(spec, fsm.StateBackup, fsm.StateMaster, "master-down-expired")

	// Metrics side.
	if got := reg.stateVec.single(t); got != stateValue(fsm.StateMaster) {
		t.Errorf("state gauge = %v, want %v (master)", got, stateValue(fsm.StateMaster))
	}
	if got := reg.transVec.total(t); got != 1 {
		t.Errorf("transitions total = %v, want 1", got)
	}

	// Event side.
	if len(bus.events) != 1 {
		t.Fatalf("emitted %d events, want 1", len(bus.events))
	}
	ev := bus.events[0]
	if ev.namespace != Namespace || ev.eventType != EventStateChange {
		t.Errorf("event (ns=%q type=%q), want (%q,%q)", ev.namespace, ev.eventType, Namespace, EventStateChange)
	}
	sc, ok := ev.payload.(StateChange)
	if !ok {
		t.Fatalf("payload type = %T, want StateChange", ev.payload)
	}
	if sc.From != viewState(fsm.StateBackup) || sc.To != viewState(fsm.StateMaster) {
		t.Errorf("payload from/to = %q/%q, want %q/%q", sc.From, sc.To, viewState(fsm.StateBackup), viewState(fsm.StateMaster))
	}
	if sc.VRID != spec.VRID || sc.Group != spec.Name || sc.Device != spec.ParentDevice ||
		sc.Family != spec.Family || sc.Interface != spec.Interface || sc.Reason != "master-down-expired" {
		t.Errorf("payload identity mismatch: %+v", sc)
	}
}
