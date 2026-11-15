// Design: docs/architecture/api/process-protocol.md -- prospective metric queries
package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A prospective query crosses each registered transport and resolves the proposed
// routing table without installing anything. Ordinary queries retain their meaning.
func TestRouteMetricsProspectiveTransportPreservesIndependentReachability(t *testing.T) {
	redistevents.RegisterProtocol("prospective-rpc-test")
	for _, direct := range []bool{false, true} {
		name := "socket"
		if direct {
			name = "direct"
		}
		t.Run(name, func(t *testing.T) {
			client := stateRPCClient(t, direct)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			zero := uint8(0)
			cover := rpc.RouteInstallEntry{Protocol: "prospective-rpc-test", AFI: 1, SAFI: 1,
				Prefix: "198.19.243.0/24", DistanceOverride: &zero}
			_, err := client.RouteInstall(ctx, []rpc.RouteInstallEntry{cover})
			require.NoError(t, err)
			t.Cleanup(func() {
				cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
				defer done()
				_, err := client.RouteRemove(cleanup, []rpc.RouteRemoveEntry{{
					Protocol: cover.Protocol, AFI: 1, SAFI: 1, Prefix: cover.Prefix,
				}})
				require.NoError(t, err)
			})
			addresses := []string{"198.19.243.1"}
			before, err := client.RouteMetrics(ctx, addresses)
			require.NoError(t, err)
			require.True(t, before.Distances[0].Resolved)
			proposal := rpc.RouteInstallEntry{Protocol: cover.Protocol, AFI: 1, SAFI: 1,
				Prefix: "198.19.243.0/25", Instance: 1, NextHop: addresses[0], IsBGP: true}
			loop, err := client.RouteMetricsProspective(ctx, addresses, []rpc.RouteInstallEntry{proposal})
			require.NoError(t, err)
			require.False(t, loop.Distances[0].Resolved, "the proposed /25 captures its own next hop")
			require.Equal(t, before.Revision, loop.Revision, "a preview must not publish a routing change")

			proposal.Prefix = "203.0.113.0/24"
			independent, err := client.RouteMetricsProspective(ctx, addresses, []rpc.RouteInstallEntry{proposal})
			require.NoError(t, err)
			require.True(t, independent.Distances[0].Resolved, "an independent zero-cost next hop stays usable")
			proposal.Prefix = cover.Prefix
			same, err := client.RouteMetricsProspective(ctx, addresses, []rpc.RouteInstallEntry{proposal})
			require.NoError(t, err)
			require.True(t, same.Distances[0].Resolved, "the preferred same-prefix terminal route must survive")

			after, err := client.RouteMetrics(ctx, addresses)
			require.NoError(t, err)
			require.Equal(t, before, after, "the original routing table must remain untouched")
			proposal.Prefix = "invalid"
			_, err = client.RouteMetricsProspective(ctx, addresses, []rpc.RouteInstallEntry{proposal})
			require.Error(t, err, "malformed proposals must not become partial success")
		})
	}
}
