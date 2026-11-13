// Design: docs/architecture/rib/unified-locrib.md -- allocation evidence over the actual consumer.

package sysrib

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// This endpoint uses the registered owner's real handlers and real event bus.
// It substitutes only authenticated process creation and the connected socket;
// the worker, reader, publication evaluator, ACK and output ledger are real.
type routeViewMeasurementEndpoint struct {
	ctx    context.Context
	host   *pluginserver.Server
	owner  *process.Process
	plugin *sdk.Plugin
	client *routeViewClient
}

func newRouteViewMeasurementEndpoint(tb testing.TB, internal bool, wrap func(net.Conn) net.Conn) *routeViewMeasurementEndpoint {
	tb.Helper()
	// The peer MUST remain alive for Close in Cleanup, after testing cancels
	// TB.Context. The bounded independent lifetime ends only after both join.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(tb.Context()), 3*time.Minute)
	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	if err != nil {
		cancel()
		tb.Fatal(err)
	}
	endpoint := &routeViewMeasurementEndpoint{ctx: ctx, host: host, owner: process.NewProcess(plugin.PluginConfig{Name: "measured-system-rib", Run: "rib", Internal: internal})}
	pluginEnd, engineEnd := net.Pipe()
	done := make(chan struct{})
	var engine *rpc.MuxConn
	if internal {
		bridge := rpc.NewDirectBridge()
		endpoint.plugin = sdk.NewWithConn("rib", rpc.NewBridgedConn(pluginEnd, bridge))
		bridge.SetDispatchRPC(func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
			if method == rpc.MethodRouteViewRead {
				return nil, fmt.Errorf("internal measured hot path used JSON method %s", method)
			}
			if method == rpc.MethodRouteViewPublish {
				return nil, fmt.Errorf("internal measured hot path used JSON method %s", method)
			}
			value, err := endpoint.answer(ctx, method, params)
			if err != nil {
				return nil, err
			}
			return json.Marshal(value)
		})
		bridge.SetDispatchNative(func(ctx context.Context, method string, value any) (any, bool, error) {
			if method != rpc.MethodRouteViewPublish {
				return nil, false, nil
			}
			result, err := routeViewPublishNative(endpoint.host, ctx, endpoint.owner, value)
			return result, true, err
		})
		bridge.SetReady()
		close(done)
	} else {
		endpoint.plugin = sdk.NewWithConn("rib", pluginEnd)
		peer := engineEnd
		if wrap != nil {
			peer = wrap(peer)
		}
		engine = rpc.NewMuxConn(rpc.NewConn(peer, peer))
		// The endpoint owns this responder. Cleanup MUST close both connections
		// and join done, even when construction or a measured operation fails.
		go func() {
			defer close(done)
			for {
				select {
				case <-ctx.Done():
					return
				case request, open := <-engine.Requests():
					if !open {
						return
					}
					value, err := endpoint.answer(ctx, request.Method, request.Params)
					if err != nil {
						if engine.SendError(ctx, request.ID, err.Error()) != nil {
							return
						}
						continue
					}
					if engine.SendResult(ctx, request.ID, value) != nil {
						return
					}
				}
			}
		}()
	}
	tb.Cleanup(func() {
		if endpoint.client != nil {
			cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
			if err := endpoint.client.Close(cleanup); err != nil {
				tb.Error(err)
			}
			stop()
		}
		if err := routeViews.cleanup(endpoint.owner); err != nil {
			tb.Error(err)
		}
		_ = endpoint.plugin.Close() //nolint:errcheck // Closing either socket is required even after the peer has already closed it.
		if engine != nil {
			_ = engine.Close()
		} //nolint:errcheck // The SDK may already have closed its peer.
		_ = engineEnd.Close() //nolint:errcheck // The internal bridge still owns the unused socket peer.
		cancel()
		<-done
	})
	return endpoint
}

func (endpoint *routeViewMeasurementEndpoint) answer(ctx context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case rpc.MethodRouteViewOpen:
		return routeViewOpenHandler(endpoint.host, ctx, endpoint.owner, params)
	case rpc.MethodRouteViewRead:
		return routeViewReadHandler(endpoint.host, ctx, endpoint.owner, params)
	case rpc.MethodRouteViewDistances:
		return routeViewDistancesHandler(endpoint.host, ctx, endpoint.owner, params)
	case rpc.MethodRouteViewPublish:
		return routeViewPublishHandler(endpoint.host, ctx, endpoint.owner, params)
	case rpc.MethodRouteViewClose:
		return routeViewCloseHandler(endpoint.host, ctx, endpoint.owner, params)
	default:
		return nil, fmt.Errorf("unexpected measured operation %s", method)
	}
}

// The same preallocated barrier is included in both allocation populations.
// Its task is admitted only after the exact output is observed, so the worker
// MUST finish publication and its ACK before admitting the next mutation.
type routeViewMeasurementBarrier struct{ done chan struct{} }

func (barrier *routeViewMeasurementBarrier) perform(*routeViewWorker) error {
	barrier.done <- struct{}{}
	return nil
}

func (barrier *routeViewMeasurementBarrier) join(tb testing.TB, endpoint *routeViewMeasurementEndpoint) {
	tb.Helper()
	worker, err := endpoint.client.enqueue(endpoint.ctx, barrier)
	if err != nil {
		tb.Fatal(err)
	}
	select {
	case <-barrier.done:
	case <-worker.done:
		tb.Fatalf("measured worker stopped: %v", endpoint.client.Failure())
	case <-endpoint.ctx.Done():
		tb.Fatal(endpoint.ctx.Err())
	}
}

// These figures cover one real Loc-RIB update through the actual consumer,
// owner adapter, delivered event and joined ACK. They include equal fixture
// channel/barrier overhead, and exclude one-time startup and initial snapshot.
// Only B/op and allocs/op are evidence on an unknown-performance host.
func BenchmarkRouteViewWorkerAllocations(b *testing.B) {
	for _, placement := range []struct {
		name     string
		internal bool
	}{
		{name: "InternalNative", internal: true}, {name: "ExternalJSON"},
	} {
		b.Run(placement.name, func(b *testing.B) {
			b.StopTimer()
			endpoint := newRouteViewMeasurementEndpoint(b, placement.internal, nil)
			rib := locrib.Default()
			if rib == nil {
				b.Fatal("measurement has no daemon Loc-RIB")
			}
			prefix := netip.MustParsePrefix("198.19.250.0/24")
			path := locrib.Path{Source: redistevents.RegisterProtocol("static"), RouteType: routetype.Blackhole, Metric: 1}
			observed := make(chan sysribevents.BestChangeEntry, 16)
			unsubscribe := sysribevents.BestChange.Subscribe(endpoint.host, func(batch *sysribevents.BestChangeBatch) {
				for i := range batch.Changes {
					if batch.Changes[i].Prefix == prefix {
						observed <- batch.Changes[i]
					}
				}
			})
			b.Cleanup(unsubscribe)
			b.Cleanup(func() { rib.Remove(family.IPv4Unicast, prefix, path.Source, path.Instance) })
			rib.Insert(family.IPv4Unicast, prefix, path)
			distances, err := parseAdminDistanceConfig("{}")
			if err != nil {
				b.Fatal(err)
			}
			permit, err := parseFIBImportConfig("{}")
			if err != nil {
				b.Fatal(err)
			}
			endpoint.client, err = openRouteViewClient(endpoint.ctx, endpoint.plugin, newSysRIB(), distances, permit)
			if err != nil {
				b.Fatal(err)
			}
			if err := endpoint.client.Start(endpoint.ctx); err != nil {
				b.Fatal(err)
			}
			await := func(metric uint32, action routeaction.Action) {
				select {
				case got := <-observed:
					if got.Metric != metric {
						b.Fatalf("measured output metric=%d want=%d", got.Metric, metric)
					}
					if got.Action != action {
						b.Fatalf("measured output action=%s want=%s", got.Action, action)
					}
					if got.Protocol != "static" {
						b.Fatalf("measured output protocol=%s", got.Protocol)
					}
					if got.RouteType != routetype.Blackhole {
						b.Fatalf("measured output route type=%d", got.RouteType)
					}
				case <-endpoint.ctx.Done():
					b.Fatal(endpoint.ctx.Err())
				}
			}
			barrier := &routeViewMeasurementBarrier{done: make(chan struct{}, 1)}
			await(1, routeaction.Add)
			barrier.join(b, endpoint)
			b.ReportAllocs()
			b.ResetTimer()
			b.StartTimer()
			for index := range b.N {
				path.Metric = uint32(index + 2)
				rib.Insert(family.IPv4Unicast, prefix, path)
				await(path.Metric, routeaction.Update)
				barrier.join(b, endpoint)
			}
			b.StopTimer()
		})
	}
}
