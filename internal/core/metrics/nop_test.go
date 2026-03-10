package metrics_test

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// TestNopRegistry_NoOps verifies all NopRegistry methods are safe to call.
//
// VALIDATES: NopRegistry implements Registry without panics.
// PREVENTS: Nil pointer dereference when metrics are disabled.
func TestNopRegistry_NoOps(t *testing.T) {
	var reg metrics.NopRegistry

	// Counters
	c := reg.Counter("test_counter", "help")
	c.Inc()
	c.Add(5)

	// Gauges
	g := reg.Gauge("test_gauge", "help")
	g.Set(42)
	g.Inc()
	g.Dec()
	g.Add(-1)

	// CounterVec
	cv := reg.CounterVec("test_counter_vec", "help", []string{"label"})
	cv.With("value").Inc()
	cv.With("value").Add(3)

	// GaugeVec
	gv := reg.GaugeVec("test_gauge_vec", "help", []string{"label"})
	gv.With("value").Set(1)
	gv.With("value").Inc()
	gv.With("value").Dec()
	gv.With("value").Add(-2)
}

// TestNopRegistry_ImplementsRegistry verifies NopRegistry satisfies the Registry interface.
//
// VALIDATES: NopRegistry is a valid Registry implementation.
// PREVENTS: Interface mismatch at compile time.
func TestNopRegistry_ImplementsRegistry(t *testing.T) {
	var _ metrics.Registry = metrics.NopRegistry{}
}
