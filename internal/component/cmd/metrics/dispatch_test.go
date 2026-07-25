package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	"github.com/ze-software/ze/internal/core/metrics"

	registry "github.com/ze-software/ze/internal/component/plugin/registry"
)

// newDispatchContext creates a CommandContext with all init()-registered RPCs,
// simulating the production dispatch chain.
func newDispatchContext() *pluginserver.CommandContext {
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	return &pluginserver.CommandContext{Server: server}
}

// TestDispatchBGPMetricsValues verifies "show metrics values" dispatches through init() registration.
//
// VALIDATES: AC-5 — show metrics values registered and dispatchable.
// PREVENTS: Metrics values handler not registered in dispatcher.
func TestDispatchBGPMetricsValues(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	reg.Counter("dispatch_test_total", "test").Inc()

	old := registry.GetMetricsRegistry()
	registry.SetMetricsRegistry(reg)
	defer registry.SetMetricsRegistry(old)

	ctx := newDispatchContext()
	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show metrics values")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

// TestDispatchBGPMetricsList verifies "show metrics list" dispatches through init() registration.
//
// VALIDATES: AC-5 — show metrics list registered and dispatchable.
// PREVENTS: Metrics list handler not registered in dispatcher.
func TestDispatchBGPMetricsList(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	reg.Counter("dispatch_list_total", "test").Inc()

	old := registry.GetMetricsRegistry()
	registry.SetMetricsRegistry(reg)
	defer registry.SetMetricsRegistry(old)

	ctx := newDispatchContext()
	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "show metrics list")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}
