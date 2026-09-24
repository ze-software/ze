package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The subprocess-facing SDK must read the engine's recursive route metrics,
// including a BGP hop whose received AIGP changes without a new client UPDATE.
// Both calls use the registered dispatch, not a synthetic RPC response.
func TestRouteMetricsTransportResolvesRecursiveChanges(t *testing.T) {
	redistevents.RegisterProtocol("aigp-metric-rpc-test")
	for _, direct := range []bool{false, true} {
		name := "socket"
		if direct {
			name = "direct"
		}
		t.Run(name, func(t *testing.T) {
			client := stateRPCClient(t, direct)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			routes := []rpc.RouteInstallEntry{
				{Protocol: "aigp-metric-rpc-test", AFI: 1, SAFI: 1, Prefix: "198.19.231.1/32", Metric: 7},
				{Protocol: "aigp-metric-rpc-test", AFI: 1, SAFI: 1, Prefix: "198.19.230.1/32", NextHop: "198.19.231.1", IsBGP: true, AIGPPresent: true, AIGP: 100},
			}
			_, err := client.RouteInstall(ctx, routes)
			require.NoError(t, err)
			t.Cleanup(func() {
				cleanupCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
				defer done()
				_, err := client.RouteRemove(cleanupCtx, []rpc.RouteRemoveEntry{
					{Protocol: routes[0].Protocol, AFI: 1, SAFI: 1, Prefix: routes[0].Prefix},
					{Protocol: routes[1].Protocol, AFI: 1, SAFI: 1, Prefix: routes[1].Prefix},
				})
				require.NoError(t, err)
			})
			first, err := client.RouteMetrics(ctx, []string{"198.19.230.1"})
			require.NoError(t, err)
			require.Equal(t, []rpc.RouteMetric{{Cost: 107, Resolved: true}}, first.Distances)

			routes[1].AIGP = 200
			_, err = client.RouteInstall(ctx, routes[1:])
			require.NoError(t, err)
			changed, err := client.RouteMetrics(ctx, nil)
			require.NoError(t, err)
			require.Greater(t, changed.Revision, first.Revision, "a recursive AIGP-only change must invalidate subprocess metric caches")
			next, err := client.RouteMetrics(ctx, []string{"198.19.230.1"})
			require.NoError(t, err)
			require.Equal(t, []rpc.RouteMetric{{Cost: 207, Resolved: true}}, next.Distances)

			routes[1].AIGPPresent = false
			_, err = client.RouteInstall(ctx, routes[1:])
			require.NoError(t, err)
			missing, err := client.RouteMetrics(ctx, []string{"198.19.230.1"})
			require.NoError(t, err)
			require.Equal(t, []rpc.RouteMetric{{Cost: 7, Resolved: true, MissingAIGP: true}}, missing.Distances, "a BGP hop without AIGP must remain distinguishable from a valid accumulated metric")
		})
	}
}

// An excessive or malformed batch must be refused through both transports,
// rather than returning a partial metric vector that looks complete.
func TestRouteMetricsRejectsInvalidBatch(t *testing.T) {
	for _, direct := range []bool{false, true} {
		name := "socket"
		if direct {
			name = "direct"
		}
		t.Run(name, func(t *testing.T) {
			client := stateRPCClient(t, direct)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			addresses := make([]string, rpc.RouteMetricsAddressMax+1)
			for i := range addresses {
				addresses[i] = "198.19.239.1"
			}
			_, err := client.RouteMetrics(ctx, addresses[:rpc.RouteMetricsAddressMax])
			require.NoError(t, err, "the maximum-sized batch must remain supported")
			out, err := client.RouteMetrics(ctx, addresses)
			require.Error(t, err)
			require.Nil(t, out)
			out, err = client.RouteMetrics(ctx, []string{"198.19.239.1", "not-an-address"})
			require.Error(t, err)
			require.Nil(t, out)
		})
	}
}
