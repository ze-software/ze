// Design: docs/architecture/rib/unified-locrib.md -- broadcast replay admission.
// Related: route_view_worker.go -- reject targeted requests before publication.

package sysrib

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

// Both readers run the real worker, owner adapter, output validator and event
// publication. Invalid IDs must leave forwarding and a pending broadcast alive.
func TestRouteViewRuntimeReplayRejectsNonBroadcastBothPlacements(t *testing.T) {
	for _, placement := range []struct {
		name     string
		internal bool
	}{
		{name: "InternalNative", internal: true},
		{name: "ExternalJSON"},
	} {
		t.Run(placement.name, func(t *testing.T) {
			for _, request := range []struct {
				name string
				id   uint64
			}{
				{name: "zero"},
				{name: "targeted", id: 17},
			} {
				t.Run(request.name, func(t *testing.T) {
					testRouteViewReplayAdmission(t, placement.internal, request.id)
				})
			}
		})
	}
}

func testRouteViewReplayAdmission(t *testing.T, internal bool, invalid uint64) {
	t.Helper()
	endpoint := newRouteViewMeasurementEndpoint(t, internal, nil)
	ctx, cancel := context.WithTimeout(endpoint.ctx, 20*time.Second)
	t.Cleanup(cancel)
	rib := locrib.Default()
	if rib == nil {
		t.Fatal("replay admission test has no daemon authority")
	}
	prefix := netip.MustParsePrefix("198.19.248.0/24")
	path := locrib.Path{Source: redistevents.RegisterProtocol("static"), RouteType: routetype.Blackhole, Metric: 1}
	t.Cleanup(func() { rib.Remove(family.IPv4Unicast, prefix, path.Source, path.Instance) })
	observed := make(chan sysribevents.BestChangeBatch, 32)
	delivered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	var hold atomic.Bool
	unsubscribe := sysribevents.BestChange.Subscribe(endpoint.host, func(batch *sysribevents.BestChangeBatch) {
		for i := range batch.Changes {
			entry := &batch.Changes[i]
			if entry.Prefix != prefix {
				continue
			}
			select {
			case observed <- routeViewCloneOutputBatch(*batch):
			case <-ctx.Done():
				return
			}
			if batch.ReplayID != replay.Broadcast {
				return
			}
			if hold.CompareAndSwap(true, false) {
				close(delivered)
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
			return
		}
	})
	t.Cleanup(unsubscribe)
	rib.Insert(family.IPv4Unicast, prefix, path)
	distances, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	permit, err := parseFIBImportConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	endpoint.client, err = openRouteViewClient(ctx, endpoint.plugin, newSysRIB(), distances, permit)
	if err != nil {
		t.Fatal(err)
	}
	client := endpoint.client
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	worker := client.worker
	await := func(metric uint32, action routeaction.Action, identity uint64) {
		t.Helper()
		for {
			select {
			case batch := <-observed:
				if batch.ReplayID != identity {
					continue
				}
				for i := range batch.Changes {
					entry := &batch.Changes[i]
					if entry.Prefix != prefix {
						continue
					}
					if entry.Metric != metric {
						continue
					}
					if entry.Action != action {
						continue
					}
					if entry.Protocol != "static" {
						t.Fatalf("forwarding control changed protocol: %+v", *entry)
					}
					if entry.RouteType != routetype.Blackhole {
						t.Fatalf("forwarding control changed action: %+v", *entry)
					}
					return
				}
			case <-worker.done:
				t.Fatalf("worker stopped before metric %d replay %d: %v", metric, identity, client.Failure())
			case <-ctx.Done():
				t.Fatalf("forwarding control metric %d replay %d was not delivered: %v", metric, identity, ctx.Err())
			}
		}
	}
	barrier := &routeViewMeasurementBarrier{done: make(chan struct{}, 1)}
	await(1, routeaction.Add, 0)
	barrier.join(t, endpoint)
	refuse := func() {
		t.Helper()
		err := client.HandleEvent(routeViewReplayEvent(t, invalid))
		if err == nil {
			t.Errorf("invalid system replay identity %d was admitted", invalid)
			return
		}
		var failure *opererr.Error
		if !errors.As(err, &failure) {
			t.Errorf("invalid replay refusal is not an operator error: %v", err)
			return
		}
		if failure.Fix() != "ze request reload" {
			t.Errorf("invalid replay fix %q is not the published reload command", failure.Fix())
		}
	}
	refuse()
	client.mu.Lock()
	queued := len(client.replays)
	client.mu.Unlock()
	if queued != 0 {
		t.Errorf("invalid replay retained %d pending identities", queued)
	}
	path.Metric = 2
	rib.Insert(family.IPv4Unicast, prefix, path)
	await(2, routeaction.Update, 0)
	barrier.join(t, endpoint)
	hold.Store(true)
	if err := client.HandleEvent(routeViewReplayEvent(t, replay.Broadcast)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-delivered:
	case <-worker.done:
		t.Fatalf("valid broadcast stopped the worker: %v", client.Failure())
	case <-ctx.Done():
		t.Fatal("valid broadcast never reached actual output delivery")
	}
	refuse()
	if err := client.HandleEvent(routeViewReplayEvent(t, replay.Broadcast)); err != nil {
		t.Fatal(err)
	}
	client.mu.Lock()
	queued, revision := len(client.replays), client.replays[replay.Broadcast]
	client.mu.Unlock()
	if queued != 1 {
		t.Errorf("invalid replay changed the pending broadcast's identity count to %d", queued)
	}
	if revision != 2 {
		t.Errorf("repeated broadcast arrival revision %d, want 2 before acknowledgment", revision)
	}
	path.Metric = 3
	rib.Insert(family.IPv4Unicast, prefix, path)
	releaseOnce.Do(func() { close(release) })
	await(3, routeaction.Add, replay.Broadcast)
	barrier.join(t, endpoint)
	client.mu.Lock()
	queued = len(client.replays)
	client.mu.Unlock()
	if queued != 0 {
		t.Errorf("acknowledged repeated broadcast retained %d pending identities", queued)
	}
	path.Metric = 4
	rib.Insert(family.IPv4Unicast, prefix, path)
	await(4, routeaction.Update, 0)
	barrier.join(t, endpoint)
	if client.worker != worker {
		t.Error("invalid request replaced the healthy worker")
	}
	if err := client.Failure(); err != nil {
		t.Errorf("invalid request retained a worker failure: %v", err)
	}
	findings, err := client.health.findings(routeViewDoctorName)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("invalid request left a healthy control unavailable: %+v", findings)
	}
}
