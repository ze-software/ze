// Design: docs/architecture/rib/unified-locrib.md -- bounded local capture recovery.

package sysrib

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// An offer may find a full queue and then resume after the consumer drained it
// and checked overflow. Complete that offer's real overflow operation only
// after the worker publishes its queued control. No later input may be needed
// to withdraw the route removed by the interrupted offer.
func TestRouteViewRuntimeLateLocalOverflowWakesWorker(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	rib := locrib.NewRIB()
	source := redistevents.RegisterProtocol("static")
	prefix := netip.MustParsePrefix("198.18.240.0/24")
	control := netip.MustParsePrefix("198.18.241.0/24")
	path := locrib.Path{Source: source, RouteType: routetype.Blackhole}
	rib.Insert(family.IPv4Unicast, prefix, path)
	rib.Insert(family.IPv4Unicast, control, path)
	initial, release := make(chan struct{}), make(chan struct{})
	updated, withdrawn := make(chan struct{}), make(chan struct{})
	var first, update, removal, unblock sync.Once
	releaseInitial := func() { unblock.Do(func() { close(release) }) }
	client := routeViewNativeRuntimeClient(t, ctx, func(call context.Context, method string, payload any) (any, bool, error) {
		if method != rpc.MethodRouteViewPublish {
			return nil, false, nil
		}
		input, ok := payload.(routeViewPublication)
		if !ok {
			return nil, true, fmt.Errorf("native publication received %T", payload)
		}
		for _, entry := range input.Batch.Changes {
			if entry.Prefix == prefix && entry.Action == routeaction.Add {
				first.Do(func() { close(initial) })
				select {
				case <-release:
				case <-call.Done():
					return nil, true, call.Err()
				}
			}
			if entry.Prefix == control && entry.Action == routeaction.Update && entry.RouteType == routetype.Unreachable {
				update.Do(func() { close(updated) })
			}
			if entry.Prefix == prefix && entry.Action == routeaction.Withdraw {
				removal.Do(func() { close(withdrawn) })
			}
		}
		return rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}, true, nil
	})
	client.reader = routeViewClientLocal{rib: rib}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		releaseInitial()
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := client.Close(cleanup); err != nil {
			t.Error(err)
		}
	}()
	select {
	case <-initial:
	case <-ctx.Done():
		t.Fatal("initial admitted local route never reached publication")
	}
	capture := client.worker.local
	for range rpc.RouteViewRecordsMax {
		capture.offer(locrib.Change{Family: family.IPv4Unicast, Prefix: control})
	}
	if len(capture.keys) != rpc.RouteViewRecordsMax {
		t.Fatal("interrupted offer did not observe a full bounded queue")
	}
	// Split the callback at its full-queue branch, without a scheduling hook:
	// authority mutates now, and recordOverflow completes that branch below.
	// Unsubscribe prevents a second delivery of either controlled mutation.
	capture.unsubscribe()
	rib.Remove(family.IPv4Unicast, prefix, source, path.Instance)
	changed := path
	changed.RouteType = routetype.Unreachable
	rib.Insert(family.IPv4Unicast, control, changed)
	releaseInitial()
	select {
	case <-updated:
	case <-ctx.Done():
		t.Fatalf("queued healthy control was not updated: %v", client.Failure())
	}
	if len(capture.keys) != 0 {
		t.Fatal("normal consumer did not drain the queued keys before late overflow")
	}
	capture.recordOverflow(prefixKey{family: family.IPv4Unicast, prefix: prefix})
	select {
	case <-withdrawn:
	case <-ctx.Done():
		t.Fatalf("late overflow left the removed route installed without another input: %v", client.Failure())
	}
	client.rib.mu.RLock()
	_, retained := client.rib.routes[prefixKey{family: family.IPv4Unicast, prefix: control}]
	client.rib.mu.RUnlock()
	if !retained {
		t.Fatal("overflow recovery lost the admitted healthy control")
	}
}
