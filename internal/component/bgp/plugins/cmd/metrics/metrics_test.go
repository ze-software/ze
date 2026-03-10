package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"

	registry "codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// TestMetricsShowWithRegistry verifies handler returns Prometheus text when registry available.
//
// VALIDATES: AC-1 — bgp metrics values returns Prometheus text format output.
// PREVENTS: Handler returning empty or wrong format when metrics are available.
func TestMetricsShowWithRegistry(t *testing.T) {
	// Set up a Prometheus registry with a test metric
	reg := metrics.NewPrometheusRegistry()
	reg.Counter("test_counter_total", "A test counter").Inc()

	old := registry.GetMetricsRegistry()
	registry.SetMetricsRegistry(reg)
	defer registry.SetMetricsRegistry(old)

	ctx := &pluginserver.CommandContext{}
	resp, err := handleMetricsValues(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "expected map[string]any data")
	output, ok := data["metrics"].(string)
	require.True(t, ok, "expected string in metrics field")
	assert.Contains(t, output, "test_counter_total")
}

// TestMetricsShowNoRegistry verifies handler returns error when no registry.
//
// VALIDATES: AC-3 — bgp metrics values with no metrics registry returns error.
// PREVENTS: Panic when telemetry is disabled and registry is nil.
func TestMetricsShowNoRegistry(t *testing.T) {
	old := registry.GetMetricsRegistry()
	registry.SetMetricsRegistry(nil)
	defer registry.SetMetricsRegistry(old)

	ctx := &pluginserver.CommandContext{}
	resp, err := handleMetricsValues(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "metrics not available")
}

// TestMetricsListWithRegistry verifies handler returns metric names only.
//
// VALIDATES: AC-2 — bgp metrics list returns metric name strings only.
// PREVENTS: Handler returning values or help text instead of just names.
func TestMetricsListWithRegistry(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	reg.Counter("test_list_total", "A test counter").Inc()
	reg.Gauge("test_gauge_value", "A test gauge").Set(42)

	old := registry.GetMetricsRegistry()
	registry.SetMetricsRegistry(reg)
	defer registry.SetMetricsRegistry(old)

	ctx := &pluginserver.CommandContext{}
	resp, err := handleMetricsList(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "expected map[string]any data")
	names, ok := data["names"].([]string)
	require.True(t, ok, "expected []string in names field")
	assert.Contains(t, names, "test_list_total")
	assert.Contains(t, names, "test_gauge_value")
}

// TestMetricsListNoRegistry verifies handler returns error when no registry.
//
// VALIDATES: AC-4 — bgp metrics list with no metrics registry returns error.
// PREVENTS: Panic when telemetry is disabled and registry is nil.
func TestMetricsListNoRegistry(t *testing.T) {
	old := registry.GetMetricsRegistry()
	registry.SetMetricsRegistry(nil)
	defer registry.SetMetricsRegistry(old)

	ctx := &pluginserver.CommandContext{}
	resp, err := handleMetricsList(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Data, "metrics not available")
}
