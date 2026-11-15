// Design: docs/architecture/route-selection.md -- bounded prospective metric queries
package rib

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// A live engine socket that withholds one response must not hold an election
// forever or mark the route reachable. A later answered query must remain usable:
// failure is not a cached negative result. The deadline measures the actual
// response bound, rather than allowing time for an asynchronous state to settle.
func TestProspectiveMetricUnansweredRequestIsBounded(t *testing.T) {
	clientSide, engineSide := net.Pipe()
	client := sdk.NewWithConn("bgp-rib", clientSide)
	engine := rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))
	// Keep the observation bound independent of the producer constant: raising
	// or removing that bound must redden this test instead of extending its wait.
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	requestSeen := make(chan string, 1)
	serverResult := make(chan error, 1)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		first, err := prospectiveMetricRequest(ctx, engine)
		if err != nil {
			serverResult <- err
			return
		}
		requestSeen <- first.Method
		// Deliberately leave the first request unanswered while the connection
		// stays open. The second request proves the failed result was not cached.
		second, err := prospectiveMetricRequest(ctx, engine)
		if err != nil {
			serverResult <- err
			return
		}
		if second.Method != rpc.MethodRouteMetrics {
			serverResult <- fmt.Errorf("retry used %s instead of route-metrics", second.Method)
			return
		}
		serverResult <- engine.SendResult(ctx, second.ID, &rpc.RouteMetricsOutput{Distances: []rpc.RouteMetric{{Resolved: true}}})
	}()
	remote := &remoteRIB{client: client}
	path := locrib.Path{Source: bgpProtocolID, Instance: bgpLocRIBInstance, IsBGP: true,
		NextHop: netip.MustParseAddr("198.51.100.1")}
	prefix := netip.MustParsePrefix("10.48.0.0/24")
	result := make(chan igpcost.Distance, 1)
	callerDone := make(chan struct{})
	go func() {
		defer close(callerDone)
		result <- remote.prospectiveDistance(family.IPv4Unicast, prefix, path)
	}()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, client.Close())
		require.NoError(t, engine.Close())
		<-callerDone
		<-serverDone
	})
	select {
	case method := <-requestSeen:
		require.Equal(t, rpc.MethodRouteMetrics, method)
	case err := <-serverResult:
		t.Fatalf("engine failed before the unanswered query: %v", err)
	case <-ctx.Done():
		t.Fatal("the prospective query never reached the live engine connection")
	}
	select {
	case distance := <-result:
		require.False(t, distance.Resolved, "an unanswered lookup supplies no reachability")
	case <-ctx.Done():
		t.Fatal("an unanswered engine request held prospective admission without a response bound")
	}
	retry := remote.prospectiveDistance(family.IPv4Unicast, prefix, path)
	require.True(t, retry.Resolved, "an answered retry must not reuse the failed lookup")
	require.NoError(t, <-serverResult)
}

func prospectiveMetricRequest(ctx context.Context, engine *rpc.MuxConn) (*rpc.Request, error) {
	select {
	case request, open := <-engine.Requests():
		if !open {
			return nil, io.EOF
		}
		return request, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
