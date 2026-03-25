package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/transaction"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"

	registry "codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// newDispatchContext creates a CommandContext with all init()-registered RPCs,
// simulating the production dispatch chain.
func newDispatchContext() *pluginserver.CommandContext {
	server, _ := pluginserver.NewServer(&pluginserver.ServerConfig{
		CommitManager: transaction.NewCommitManager(),
	}, nil)
	return &pluginserver.CommandContext{Server: server}
}

// TestDispatchBGPMetricsValues verifies "metrics values" dispatches through init() registration.
//
// VALIDATES: AC-5 — metrics values registered and dispatchable.
// PREVENTS: Metrics values handler not registered in dispatcher.
func TestDispatchBGPMetricsValues(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	reg.Counter("dispatch_test_total", "test").Inc()

	old := registry.GetMetricsRegistry()
	registry.SetMetricsRegistry(reg)
	defer registry.SetMetricsRegistry(old)

	ctx := newDispatchContext()
	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "metrics values")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}

// TestDispatchBGPMetricsList verifies "metrics list" dispatches through init() registration.
//
// VALIDATES: AC-5 — metrics list registered and dispatchable.
// PREVENTS: Metrics list handler not registered in dispatcher.
func TestDispatchBGPMetricsList(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	reg.Counter("dispatch_list_total", "test").Inc()

	old := registry.GetMetricsRegistry()
	registry.SetMetricsRegistry(reg)
	defer registry.SetMetricsRegistry(old)

	ctx := newDispatchContext()
	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "metrics list")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)
}
