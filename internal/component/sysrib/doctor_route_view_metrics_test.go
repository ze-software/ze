// Design: docs/plugin-development/metrics.md -- measured routing-view SDK publication.
package sysrib

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRouteViewDoctorRuntimePublishesSDKMetrics(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	published := make(chan rpc.MetricsPublishInput, 8)
	feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		if request.Method != rpc.MethodMetricsPublish {
			return nil, false, nil
		}
		var input rpc.MetricsPublishInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, true, err
		}
		if err := rpc.ValidateMetricsPublish(&input); err != nil {
			return nil, true, err
		}
		published <- input
		return rpc.MetricsPublishOutput{}, true, nil
	})
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if client.health == nil {
		t.Fatal("consumer has no runtime health monitor")
	}
	flush := func() rpc.MetricsPublishInput {
		t.Helper()
		if err := feed.plugin.FlushMetrics(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case input := <-published:
			return input
		case <-ctx.Done():
			t.Fatal(ctx.Err())
			return rpc.MetricsPublishInput{}
		}
	}
	initial := flush()
	if len(initial.Declare) != 3 {
		t.Fatalf("routing-view SDK metric declarations = %v", initial.Declare)
	}
	for _, declaration := range initial.Declare {
		if len(declaration.Labels) != 0 {
			t.Fatalf("routing-view metric has variable labels: %+v", declaration)
		}
	}
	assertRouteViewPublishedGauge(t, initial, "ze_rib_view_ready", 0)
	feed.pages <- runtimeSnapshot(2, true, runtimeSelected(netip.MustParsePrefix("198.18.241.0/24"), rpc.RouteViewPath{Protocol: "static", Distance: 10, RouteType: uint8(routetype.Blackhole)}))
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	awaitRouteViewDoctor(t, ctx, client.health, "")
	healthy := flush()
	if len(healthy.Declare) != 0 {
		t.Fatalf("healthy update redeclared metrics: %v", healthy.Declare)
	}
	assertRouteViewPublishedGauge(t, healthy, "ze_rib_view_ready", 1)
	assertRouteViewPublishedGauge(t, healthy, "ze_rib_view_pending_prefixes", 0)
}

func assertRouteViewPublishedGauge(t *testing.T, input rpc.MetricsPublishInput, name string, want float64) {
	t.Helper()
	for _, update := range input.Update {
		if update.Name != name {
			continue
		}
		if len(update.Values) != 0 {
			t.Fatalf("metric %s has variable label values", name)
		}
		if update.Value != want {
			t.Fatalf("metric %s = %v, want %v", name, update.Value, want)
		}
		return
	}
	t.Fatalf("metric %s absent from actual SDK publish %+v", name, input)
}
