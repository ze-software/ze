package rib

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The RIB has no local routing table or shared igpcost callback here. The engine
// computes real recursive distances and the selected path leaves over the SDK
// socket transport used by subprocesses. Server-side registered dispatch is
// covered independently by TestRouteMetricsTransportResolvesRecursiveChanges.
func TestSocketMetricFeedReselectsRetainedRoutes(t *testing.T) {
	clientSide, engineSide := net.Pipe()
	client := sdk.NewWithConn("bgp-rib", clientSide)
	engine := rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	engineRIB := locrib.NewRIB()
	protocol := redistevents.RegisterProtocol("aigp-remote-test")
	metric := func(addr string, cost uint32) {
		engineRIB.Insert(family.IPv4Unicast, netip.MustParsePrefix(addr+"/32"), locrib.Path{Source: protocol, Metric: cost})
	}
	metric("198.51.100.1", 5)
	metric("198.51.100.2", 20)
	installed := make(chan rpc.RouteInstallEntry, 8)
	serverErrors := make(chan error, 1)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		for req := range engine.Requests() {
			var result any
			var err error
			switch req.Method {
			case rpc.MethodRouteMetrics:
				var input rpc.RouteMetricsInput
				err = json.Unmarshal(req.Params, &input)
				out := &rpc.RouteMetricsOutput{Revision: engineRIB.Revision()}
				for _, text := range input.Addresses {
					addr, parseErr := netip.ParseAddr(text)
					if parseErr != nil {
						err = parseErr
						break
					}
					distance := igpcost.Resolve(engineRIB, addr)
					out.Distances = append(out.Distances, rpc.RouteMetric{Cost: distance.Cost, Resolved: distance.Resolved, MissingAIGP: distance.MissingAIGP})
				}
				result = out
			case rpc.MethodRouteInstall:
				var input rpc.RouteInstallInput
				err = json.Unmarshal(req.Params, &input)
				for _, entry := range input.Routes {
					installed <- entry
				}
				result = &rpc.RouteInstallOutput{Installed: uint32(len(input.Routes))}
			case rpc.MethodRouteRemove:
				result = &rpc.RouteRemoveOutput{}
			default:
				err = fmt.Errorf("unexpected engine call %s", req.Method)
			}
			if err == nil {
				err = engine.SendResult(ctx, req.ID, result)
			}
			if err != nil {
				select {
				case serverErrors <- err:
				default:
				}
				_ = engine.SendError(ctx, req.ID, err.Error())
				return
			}
		}
	}()
	t.Cleanup(func() { cancel(); _ = client.Close(); _ = engine.Close(); <-serverDone })
	r := newRIBManager(client)
	r.setupRemoteRIB()
	pollStopped := make(chan struct{})
	go func() { defer close(pollStopped); r.runAIGPSelection(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-pollStopped
		for _, routes := range r.bgpPeers {
			routes.Release()
		}
	})
	receivedMetric := uint64(10)
	for i, peer := range []string{"192.0.2.1", "192.0.2.2"} {
		attrs := aigpSelectionAttrs([4]byte{198, 51, 100, byte(i + 1)}, &receivedMetric, 100, 1)
		encoded := fmt.Sprintf(`{"type":"bgp","bgp":{"message":{"type":"update","id":%d},"peer":{"address":%q,"remote":{"address":%q,"as":65001},"local":{"as":65000}},"raw":{"attributes":%q,"nlri":{"ipv4/unicast":"180a1400"}},"update":{"nlri":{"ipv4/unicast":[{"action":"add","next-hop":"198.51.100.%d","nlri":["10.20.0.0/24"]}]}}}}`, i+1, peer, peer, hex.EncodeToString(attrs), i+1)
		event, err := parseEvent([]byte(encoded))
		require.NoError(t, err)
		r.dispatch(event)
	}
	awaitPath := func(nextHop string) {
		t.Helper()
		select {
		case entry := <-installed:
			require.Equal(t, "10.20.0.0/24", entry.Prefix)
			require.Equal(t, nextHop, entry.NextHop)
			require.True(t, entry.AIGPPresent)
			require.Equal(t, uint64(10), entry.AIGP, "recursive consumers need received AIGP, not accumulated selection cost")
		case err := <-serverErrors:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal("remote metric change did not publish a selected path")
		}
	}
	awaitPath("198.51.100.1")
	metric("198.51.100.1", 30)
	awaitPath("198.51.100.2")
}
