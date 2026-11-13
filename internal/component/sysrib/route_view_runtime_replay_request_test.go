// Design: docs/architecture/rib/unified-locrib.md -- bounded replay requests.
// Related: route_view_worker.go -- requests arriving during an in-flight replay.

package sysrib

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A subscriber joining after delivery still needs a fresh broadcast, even if
// the previous request's final publication acknowledgment remains in flight.
func TestRouteViewRuntimeReplayRetainsRequestAfterDelivery(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	peerCtx, peerCancel := context.WithCancel(context.WithoutCancel(ctx))
	defer peerCancel()
	h := newPublicationResetHarness(t, ctx, "reset")
	delivered := make(chan struct{})
	release := make(chan struct{})
	var first atomic.Bool
	first.Store(true)
	h.plugin = routeViewRuntimePeer(t, peerCtx, func(request *rpc.Request) (any, error) {
		answer, err := h.answer(request)
		if err != nil {
			return nil, err
		}
		if request.Method != rpc.MethodRouteViewPublish {
			return answer, nil
		}
		var input rpc.RouteViewPublishInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		batch, err := routeViewDecodeOutputBatch(input.Batch)
		if err != nil {
			return nil, err
		}
		if batch.ReplayID == replay.Broadcast {
			if first.CompareAndSwap(true, false) {
				close(delivered)
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}
		return answer, nil
	})
	client := h.start(t)
	closed := false
	defer func() {
		if !closed {
			closeRuntimeFeedClient(t, ctx, client)
		}
	}()
	h.await(t, client, h.oldPrefix, 1)
	payload, err := json.Marshal(replay.Request{ReplayID: replay.Broadcast})
	if err != nil {
		t.Fatal(err)
	}
	event, err := json.Marshal(rpc.EventEnvelope{Namespace: sysribevents.Namespace, Event: sysribevents.EventReplayRequest, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.HandleEvent(string(event)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-delivered:
	case <-client.worker.done:
		t.Fatalf("first replay failed before actual delivery: %v", client.Failure())
	case <-ctx.Done():
		t.Fatal("first replay never reached actual delivery before its held acknowledgment")
	}
	late := make(chan sysribevents.BestChangeBatch, 4)
	unsubscribe := sysribevents.BestChange.Subscribe(h.bus, func(batch *sysribevents.BestChangeBatch) {
		if batch.ReplayID != replay.Broadcast {
			return
		}
		select {
		case late <- routeViewCloneOutputBatch(*batch):
		case <-ctx.Done():
		}
	})
	t.Cleanup(unsubscribe)
	h.rib.Insert(family.IPv4Unicast, h.oldPrefix, h.path(2))
	if err := client.HandleEvent(string(event)); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case batch := <-late:
		if len(batch.Changes) != 1 {
			t.Fatalf("late subscriber's replay has %d entries", len(batch.Changes))
		}
		entry := batch.Changes[0]
		if entry.Prefix != h.oldPrefix || entry.Metric != 2 {
			t.Fatalf("late subscriber received stale replay: %+v", entry)
		}
	case <-client.worker.done:
		t.Fatalf("late replay request stopped the worker: %v", client.Failure())
	case <-ctx.Done():
		t.Fatal("request arriving after delivery was lost before the old acknowledgment retired")
	}
	if err := client.Close(ctx); err != nil {
		t.Fatal(err)
	}
	closed = true
}
