package metrics_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/metrics"
)

// TestRegisterRuntimeCollectors verifies the standard Go runtime (go_*) and
// process (process_*) collectors are exposed on /metrics once registered.
//
// VALIDATES: L84 phase 6 -- process/runtime metrics available to scrape.
// PREVENTS: a metrics endpoint with no visibility into goroutine growth, GC, or
// process memory/CPU/FD usage.
func TestRegisterRuntimeCollectors(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	reg.RegisterRuntimeCollectors()

	body := scrapeMetrics(t, reg)

	// Go runtime collector (always present).
	assert.Contains(t, body, "go_goroutines", "go_goroutines must be exposed")
	assert.Contains(t, body, "go_gc_duration_seconds", "go GC metrics must be exposed")
	// Process collector (process_start_time_seconds is always present on Linux).
	assert.Contains(t, body, "process_start_time_seconds", "process_* metrics must be exposed")

	// Registering twice must not panic (collectors are idempotent via the guard).
	assert.NotPanics(t, func() { reg.RegisterRuntimeCollectors() },
		"repeat registration must be a safe no-op")
	// After the second call the collectors are still present exactly once.
	body2 := scrapeMetrics(t, reg)
	assert.Equal(t, strings.Count(body, "\ngo_goroutines "), strings.Count(body2, "\ngo_goroutines "),
		"go_goroutines must not be double-registered")
}
