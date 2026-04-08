package process

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// scrapeMetrics returns the Prometheus text exposition from the given registry.
func scrapeMetrics(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	ts := httptest.NewServer(reg.Handler())
	defer ts.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL, http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // test-only URL
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// TestPluginMetricsRegistered verifies that all 3 plugin metrics are created
// from the registry when SetMetricsRegistry is called on ProcessManager.
//
// VALIDATES: AC-1/AC-2/AC-3 prerequisite -- metrics are registered with Prometheus.
// PREVENTS: Metrics not appearing in scrape output.
func TestPluginMetricsRegistered(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	pm := NewProcessManager(nil)
	pm.SetMetricsRegistry(reg)

	// Create a process and trigger all three metric types.
	proc := NewProcess(plugin.PluginConfig{Name: "test-plugin"})
	pm.AddProcess("test-plugin", proc)

	// Trigger status gauge via stage change.
	proc.SetStage(plugin.StageRunning)

	// Trigger restart and delivery counters directly.
	// Vec metrics only appear in scrape output after first label use.
	pm.pmetrics.restarts.With("test-plugin").Inc()
	pm.pmetrics.delivered.With("test-plugin").Inc()

	body := scrapeMetrics(t, reg)
	assert.Contains(t, body, "ze_plugin_status", "status gauge should be registered")
	assert.Contains(t, body, "ze_plugin_restarts_total", "restart counter should be registered")
	assert.Contains(t, body, "ze_plugin_events_delivered_total", "delivery counter should be registered")
}

// TestPluginStatusMetric verifies that the status gauge updates on stage transitions.
//
// VALIDATES: AC-1 -- ze_plugin_status{plugin=X} reflects running state.
// VALIDATES: AC-4 -- ze_plugin_status{plugin=X} reflects stopped state (stage 0).
// PREVENTS: Stale gauge values after stage transitions.
func TestPluginStatusMetric(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	pm := NewProcessManager(nil)
	pm.SetMetricsRegistry(reg)

	proc := NewProcess(plugin.PluginConfig{Name: "test-status"})
	pm.AddProcess("test-status", proc)

	// Stage transitions: Init (0) -> Registration (1) -> Running (6)
	proc.SetStage(plugin.StageRegistration)
	body := scrapeMetrics(t, reg)
	assert.Contains(t, body, `ze_plugin_status{plugin="test-status"} 1`,
		"status gauge should show stage 1 (Registration)")

	proc.SetStage(plugin.StageRunning)
	body = scrapeMetrics(t, reg)
	assert.Contains(t, body, `ze_plugin_status{plugin="test-status"} 6`,
		"status gauge should show stage 6 (Running)")

	// Back to Init (simulates restart/stopped)
	proc.SetStage(plugin.StageInit)
	body = scrapeMetrics(t, reg)
	assert.Contains(t, body, `ze_plugin_status{plugin="test-status"} 0`,
		"status gauge should show stage 0 (Init) after reset")
}

// TestPluginRestartMetric verifies that the restart counter increments on respawn.
//
// VALIDATES: AC-2 -- ze_plugin_restarts_total{plugin=X} increments by 1.
// PREVENTS: Restart counter not incrementing or wrong label.
func TestPluginRestartMetric(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()

	configs := []plugin.PluginConfig{{
		Name:           "cycle",
		Internal:       true,
		Respawn:        true,
		RespawnEnabled: true,
	}}
	pm := NewProcessManager(configs)
	pm.SetMetricsRegistry(reg)

	pm.ctx, pm.cancel = context.WithCancel(t.Context())
	defer pm.cancel()

	// First respawn should increment counter.
	err := pm.Respawn("cycle")
	require.NoError(t, err)

	body := scrapeMetrics(t, reg)
	assert.Contains(t, body, `ze_plugin_restarts_total{plugin="cycle"} 1`,
		"restart counter should be 1 after first respawn")

	// Second respawn should increment to 2.
	err = pm.Respawn("cycle")
	require.NoError(t, err)

	body = scrapeMetrics(t, reg)
	assert.Contains(t, body, `ze_plugin_restarts_total{plugin="cycle"} 2`,
		"restart counter should be 2 after second respawn")
}

// TestPluginEventDeliveryMetric verifies that the delivery counter increments
// when events are successfully enqueued via Deliver().
//
// VALIDATES: AC-3 -- ze_plugin_events_delivered_total{plugin=X} increments.
// PREVENTS: Delivery counter not wired into Deliver() path.
func TestPluginEventDeliveryMetric(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	pm := NewProcessManager(nil)
	pm.SetMetricsRegistry(reg)

	proc := NewProcess(plugin.PluginConfig{Name: "deliver-test"})
	pm.AddProcess("deliver-test", proc)

	// Start delivery goroutine so Deliver() can enqueue.
	proc.StartDelivery(t.Context())
	defer proc.Stop()

	// Deliver 3 events.
	for range 3 {
		ok := proc.Deliver(EventDelivery{Output: "test-event"})
		require.True(t, ok, "Deliver should succeed")
	}

	body := scrapeMetrics(t, reg)
	assert.Contains(t, body, `ze_plugin_events_delivered_total{plugin="deliver-test"} 3`,
		"delivery counter should be 3 after 3 deliveries")
}

// TestPluginMetricsNilRegistry verifies that ProcessManager handles nil
// registry gracefully -- no panics, no metrics.
//
// VALIDATES: AC-6 -- no panic when metrics registry not set.
// PREVENTS: Nil pointer dereference when metrics disabled.
func TestPluginMetricsNilRegistry(t *testing.T) {
	pm := NewProcessManager(nil)
	// No SetMetricsRegistry call -- registry stays nil.

	proc := NewProcess(plugin.PluginConfig{Name: "test-plugin"})
	pm.AddProcess("test-plugin", proc)

	// Must not panic on stage change.
	assert.NotPanics(t, func() {
		proc.SetStage(plugin.StageRunning)
	})
}

// TestPluginMetricsDeletedOnDisable verifies that the status gauge label is
// deleted when a plugin is disabled due to respawn limit exceeded.
//
// VALIDATES: AC-5 -- ze_plugin_status{plugin=X} label deleted (metric absent).
// PREVENTS: Stale gauge values for disabled plugins.
func TestPluginMetricsDeletedOnDisable(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()

	configs := []plugin.PluginConfig{{
		Name:           "crash-metrics",
		Internal:       true,
		Run:            "crash",
		Respawn:        true,
		RespawnEnabled: true,
	}}
	pm := NewProcessManager(configs)
	pm.SetMetricsRegistry(reg)

	// Seed delivery counter before disable to verify it survives.
	pm.pmetrics.delivered.With("crash-metrics").Inc()

	// Exhaust respawn limit to trigger disable.
	// Each successful Respawn() increments the restart counter (manager.go:412).
	// RespawnLimit=5: first 5 calls succeed (counter=5), 6th hits limit and disables.
	pm.ctx, pm.cancel = context.WithCancel(t.Context())
	defer pm.cancel()
	for range RespawnLimit + 1 {
		_ = pm.Respawn("crash-metrics")
	}

	// Plugin should be disabled.
	require.True(t, pm.IsDisabled("crash-metrics"))

	body := scrapeMetrics(t, reg)

	// Status gauge should NOT contain the disabled plugin.
	assert.NotContains(t, body, `ze_plugin_status{plugin="crash-metrics"}`,
		"disabled plugin should have its status label deleted")

	// Restart and delivery counters should be preserved for post-mortem monitoring.
	// Counters must not be deleted mid-lifetime (breaks rate() queries).
	assert.Contains(t, body, `ze_plugin_restarts_total{plugin="crash-metrics"} 5`,
		"restart counter should be preserved after disable for post-mortem")
	assert.Contains(t, body, `ze_plugin_events_delivered_total{plugin="crash-metrics"} 1`,
		"delivery counter should be preserved after disable for post-mortem")
}
