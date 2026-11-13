// Design: docs/plugin-development/metrics.md -- current selected-input counters.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

func TestRouteViewSelectedBatchCountsReceivedInput(t *testing.T) {
	previous := sysribMetricsPtr.Load()
	t.Cleanup(func() { sysribMetricsPtr.Store(previous) })
	registry := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(registry)
	state := newSysRIB()
	if _, err := state.applySelectedBatch([]routeViewSelectedChange{nil}); err == nil {
		t.Fatal("malformed selected batch was accepted")
	}
	assertRouteViewMetric(t, registry, "ze_systemrib_events_received_total 0")
	rib := locrib.NewRIB()
	prefix := netip.MustParsePrefix("198.18.255.0/24")
	rib.Insert(family.IPv4Unicast, prefix, locrib.Path{
		Source: redistevents.RegisterProtocol("static"), RouteType: routetype.Blackhole,
	})
	selected := selectedFromAuthority(t, rib, prefix, false)
	batches, err := state.applySelectedBatch([]routeViewSelectedChange{selected})
	if err != nil {
		t.Fatal(err)
	}
	if changes := batches[family.IPv4Unicast]; len(changes) != 1 || changes[0].Action != routeaction.Add || changes[0].RouteType != routetype.Blackhole {
		t.Fatalf("healthy selected input did not produce useful forwarding: %+v", changes)
	}
	assertRouteViewMetric(t, registry, "ze_systemrib_events_received_total 1")
	batches, err = state.applySelectedBatch([]routeViewSelectedChange{selected})
	if err != nil {
		t.Fatal(err)
	}
	if len(batches[family.IPv4Unicast]) != 0 {
		t.Fatal("unchanged accepted input emitted duplicate forwarding")
	}
	assertRouteViewMetric(t, registry, "ze_systemrib_events_received_total 2")
}
