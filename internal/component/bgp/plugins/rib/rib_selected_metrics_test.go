// Design: docs/guide/monitoring.md -- bounded selected-output metrics.
package rib

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestSelectedDispatchMetricsFollowCurrentObligations(t *testing.T) {
	previous := metricsPtr.Load()
	t.Cleanup(func() { metricsPtr.Store(previous) })
	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)
	r := newTestRIBManager(t)
	ctx := selectedDispatchContext(t)
	peer := netip.MustParseAddr("192.0.2.95")
	entered, release := make(chan struct{}, 1), make(chan struct{})
	d, err := newSelectedDispatch(ctx, func(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		if input.Routes[0].Kind == rpc.SelectedRouteReceived {
			entered <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
	}, nil, nil)
	require.NoError(t, err)
	r.selectedDispatch = d
	t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
	route := selectedDispatchRoute(95, 77)
	require.NoError(t, d.publish(route))
	require.NoError(t, d.destinationUp(peer, 1005))
	selectedDispatchAwait(t, ctx, entered)
	r.updateMetrics()
	body := scrapeRIBMetrics(t, reg)
	require.Contains(t, body, "ze_rib_selected_current_keys 1")
	require.Contains(t, body, "ze_rib_selected_destinations 1")
	require.Contains(t, body, "ze_rib_selected_pending_keys 1")
	require.Contains(t, body, "ze_rib_selected_inflight_keys 1")
	require.Contains(t, body, "ze_rib_selected_failed_destinations 0")
	close(release)
	require.NoError(t, d.barrier(ctx, peer))
	require.NoError(t, d.remove(route.key))
	require.NoError(t, d.barrier(ctx, peer))
	r.updateMetrics()
	body = scrapeRIBMetrics(t, reg)
	require.Contains(t, body, "ze_rib_selected_current_keys 0")
	require.Contains(t, body, "ze_rib_selected_pending_keys 0")
	require.Contains(t, body, "ze_rib_selected_inflight_keys 0")
	require.Contains(t, body, `ze_rib_selected_batches_total{outcome="completed"} 2`)
	require.NoError(t, d.destinationDown(ctx, peer, 1005))
	r.updateMetrics()
	require.Contains(t, scrapeRIBMetrics(t, reg), "ze_rib_selected_destinations 0")
}
