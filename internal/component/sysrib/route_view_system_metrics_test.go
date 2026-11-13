// Design: docs/plugin-development/metrics.md -- placement-specific system RIB metrics.

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
	"github.com/ze-software/ze/internal/component/plugin/registry"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Internal runners inject the daemon registry. External CLI runners invoke the
// same declaration hook with a publisher and pass ConnWithMetrics to the SDK.
// Exercise those actual injection forms, rather than assuming a bare SDK
// constructor invoked a hook which belongs to its runner.
func TestRouteViewRuntimePreservesSystemRIBMetrics(t *testing.T) {
	for _, internal := range []bool{true, false} {
		t.Run(map[bool]string{true: "native", false: "selected-feed"}[internal], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			previous := sysribMetricsPtr.Load()
			t.Cleanup(func() { sysribMetricsPtr.Store(previous) })
			declaration := registry.Lookup(configRootRIB)
			if declaration == nil || declaration.ConfigureMetrics == nil {
				t.Fatal("system RIB has no registered metrics injection hook")
			}
			rib := locrib.NewRIB()
			bus := newTestEventBus()
			var client *routeViewClient
			var daemonMetrics *metrics.PrometheusRegistry
			var published <-chan rpc.MetricsPublishInput
			if internal {
				daemonMetrics = metrics.NewPrometheusRegistry()
				declaration.ConfigureMetrics(daemonMetrics)
				client = openCurrentSysRIB(t, ctx, rib, newSysRIB(), bus)
			} else {
				publisher := sdk.NewMetricsPublisher()
				declaration.ConfigureMetrics(publisher)
				client, published = openSystemMetricsFeed(t, ctx, rib, bus, publisher)
			}
			prefix := netip.MustParsePrefix("198.18.255.128/25")
			path := locrib.Path{Source: redistevents.RegisterProtocol("static"), RouteType: routetype.Blackhole}
			rib.Insert(family.IPv4Unicast, prefix, path)
			if err := client.Start(ctx); err != nil {
				t.Fatal(err)
			}
			await := func(action routeaction.Action) {
				t.Helper()
				if _, ok := waitForEntry(t, bus, func(entry sysribevents.BestChangeEntry) bool {
					return entry.Prefix == prefix && entry.Action == action
				}); !ok {
					t.Fatalf("actual worker did not publish %s: %+v", action, bestChangeEntries(bus))
				}
			}
			check := func(received int, action routeaction.Action, best int) {
				t.Helper()
				if internal {
					assertRouteViewMetric(t, daemonMetrics, fmt.Sprintf("ze_systemrib_events_received_total %d", received))
					assertRouteViewMetric(t, daemonMetrics, fmt.Sprintf("ze_systemrib_routes_best %d", best))
					assertRouteViewMetric(t, daemonMetrics, fmt.Sprintf("ze_systemrib_route_changes_total{action=%q} 1", action.String()))
					return
				}
				if err := client.plugin.FlushMetrics(ctx); err != nil {
					t.Fatal(err)
				}
				var input rpc.MetricsPublishInput
				select {
				case input = <-published:
				case <-ctx.Done():
					t.Fatal("actual external SDK did not publish its injected metrics")
				}
				if received == 1 {
					wanted := map[string]rpc.MetricKind{"ze_systemrib_events_received_total": rpc.MetricCounter,
						"ze_systemrib_routes_best": rpc.MetricGauge, "ze_systemrib_route_changes_total": rpc.MetricCounterVec}
					for _, row := range input.Declare {
						kind, wantedRow := wanted[row.Name]
						if !wantedRow {
							continue
						}
						if row.Kind != kind {
							t.Fatalf("existing metric %s changed kind: %+v", row.Name, row)
						}
						if kind == rpc.MetricCounterVec {
							if len(row.Labels) != 1 || row.Labels[0] != "action" {
								t.Fatalf("route changes lost their bounded action label: %+v", row)
							}
						} else if len(row.Labels) != 0 {
							t.Fatalf("existing scalar metric gained labels: %+v", row)
						}
						delete(wanted, row.Name)
					}
					if len(wanted) != 0 {
						t.Fatalf("external runner/SDK omitted existing metric declarations: %v", wanted)
					}
				}
				// Counters publish deltas, whereas gauges publish their current value.
				assertRouteViewPublishedGauge(t, input, "ze_systemrib_events_received_total", 1)
				assertRouteViewPublishedGauge(t, input, "ze_systemrib_routes_best", float64(best))
				for _, update := range input.Update {
					if update.Name == "ze_systemrib_route_changes_total" && len(update.Values) == 1 && update.Values[0] == action.String() && update.Value == 1 {
						return
					}
				}
				t.Fatalf("external SDK lost the %s route-change delta: %+v", action, input)
			}
			await(routeaction.Add)
			check(1, routeaction.Add, 1)
			path.RouteType = routetype.Unreachable
			rib.Insert(family.IPv4Unicast, prefix, path)
			await(routeaction.Update)
			check(2, routeaction.Update, 1)
			rib.Remove(family.IPv4Unicast, prefix, path.Source, path.Instance)
			await(routeaction.Withdraw)
			check(3, routeaction.Withdraw, 0)
		})
	}
}

// openSystemMetricsFeed runs one joined peer serving the real private authority.
// Cleanup MUST close the client before its publisher/SDK connection and peer.
func openSystemMetricsFeed(t *testing.T, ctx context.Context, rib *locrib.RIB, bus *testEventBus, publisher *sdk.MetricsPublisher) (*routeViewClient, <-chan rpc.MetricsPublishInput) {
	t.Helper()
	peer, stop := context.WithCancel(context.WithoutCancel(ctx))
	pluginEnd, engineEnd := net.Pipe()
	p := sdk.NewWithConn(configRootRIB, sdk.ConnWithMetrics(pluginEnd, publisher))
	engine := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	var authority routeViewAuthority
	owner := process.NewProcess(plugin.PluginConfig{Name: "metrics-owner", Run: configRootRIB})
	published := make(chan rpc.MetricsPublishInput, 8)
	done := make(chan struct{})
	answer := func(request *rpc.Request) (any, error) {
		switch request.Method {
		case rpc.MethodRouteViewOpen:
			var input rpc.RouteViewOpenInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, err
			}
			return authority.open(peer, owner, rib, input)
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, err
			}
			return authority.applyDistances(peer, owner, input)
		case rpc.MethodRouteViewRead:
			var input rpc.RouteViewReadInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, err
			}
			return authority.read(peer, owner, input)
		case rpc.MethodRouteViewPublish:
			var input rpc.RouteViewPublishInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, err
			}
			batch, err := routeViewDecodeOutputBatch(input.Batch)
			if err != nil {
				return nil, err
			}
			return authority.publish(peer, owner, routeViewPublication{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation, Batch: batch}, bus)
		case rpc.MethodRouteViewClose:
			var input rpc.RouteViewCloseInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, err
			}
			return authority.close(owner, input.Session)
		case rpc.MethodMetricsPublish:
			var input rpc.MetricsPublishInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, err
			}
			if err := rpc.ValidateMetricsPublish(&input); err != nil {
				return nil, err
			}
			select {
			case published <- input:
			case <-peer.Done():
				return nil, peer.Err()
			}
			return rpc.MetricsPublishOutput{}, nil
		default:
			return nil, fmt.Errorf("unexpected authentic metrics fixture method %q", request.Method)
		}
	}
	go func() {
		defer close(done)
		for {
			select {
			case <-peer.Done():
				return
			case request, open := <-engine.Requests():
				if !open {
					return
				}
				output, err := answer(request)
				if err != nil {
					if send := engine.SendError(peer, request.ID, err.Error()); send != nil {
						return
					}
				} else if send := engine.SendResult(peer, request.ID, output); send != nil {
					return
				}
			}
		}
	}()
	var client *routeViewClient
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if client != nil {
			if err := client.Close(cleanup); err != nil {
				t.Error(err)
			}
			if err := client.Failure(); err != nil {
				t.Error(err)
			}
		}
		_ = p.Close()
		_ = engine.Close()
		stop()
		select {
		case <-done:
		case <-cleanup.Done():
			t.Error("authentic external metrics peer did not join")
		}
		if err := authority.cleanup(owner); err != nil {
			t.Error(err)
		}
	})
	distances, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	permit, err := parseFIBImportConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	client, err = openRouteViewClient(ctx, p, newSysRIB(), distances, permit)
	if err != nil {
		t.Fatal(err)
	}
	return client, published
}
