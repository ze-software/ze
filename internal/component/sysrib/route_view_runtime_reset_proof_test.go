// Design: docs/architecture/rib/unified-locrib.md -- same-worker reset recovery.
// Related: route_view_runtime_publication_reset_test.go -- real authority harness.

package sysrib

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRouteViewRuntimeMalformedPublicationResetProofIsFatal(t *testing.T) {
	for _, mode := range []string{"missing-operation", "wrong-operation", "wrong-session", "wrong-epoch", "wrong-phase", "complete", "entries", "snapshot"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			h := newPublicationResetHarness(t, ctx, "reset")
			h.plugin = routeViewRuntimePeer(t, ctx, func(request *rpc.Request) (any, error) {
				answer, err := h.answer(request)
				if err != nil {
					return nil, err
				}
				page, ok := answer.(rpc.RouteViewReadOutput)
				if !ok {
					return answer, nil
				}
				if page.Phase != rpc.RouteViewPhaseReset {
					return answer, nil
				}
				encoded, err := json.Marshal(page)
				if err != nil {
					return nil, err
				}
				var raw map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &raw); err != nil {
					return nil, err
				}
				raw["rejected-publication"] = json.RawMessage(`2`)
				switch mode {
				case "missing-operation":
					delete(raw, "rejected-publication")
				case "wrong-operation":
					raw["rejected-publication"] = json.RawMessage(`3`)
				case "wrong-session":
					raw["session"] = json.RawMessage(`"another-lease"`)
				case "wrong-epoch":
					raw["epoch"] = json.RawMessage(`"another-daemon"`)
				case "wrong-phase":
					raw["phase"] = json.RawMessage(`"delta"`)
				case "complete":
					raw["complete"] = json.RawMessage(`true`)
				case "entries":
					raw["entries"] = json.RawMessage(`[{"afi":1,"safi":1,"prefix":"198.18.249.0/24","action":"withdraw"}]`)
				case "snapshot":
					raw["snapshot"] = json.RawMessage(`"unrelated-snapshot"`)
				}
				encoded, err = json.Marshal(raw)
				return json.RawMessage(encoded), err
			})
			client := h.start(t)
			t.Cleanup(func() { closeRuntimeFeedClient(t, ctx, client) })
			h.await(t, client, h.oldPrefix, routeaction.Add, 1)
			h.arm.Store(true)
			h.rib.Insert(family.IPv4Unicast, h.oldPrefix, h.path(2))
			h.awaitInterleaving(t, client)
			select {
			case <-client.worker.done:
			case <-ctx.Done():
				t.Fatal("malformed reset proof did not terminate the old worker")
			}
			if client.Failure() == nil {
				t.Fatal("malformed proof became a successful stop")
			}
			if h.opens.Load() != 1 {
				t.Fatalf("malformed proof acquired %d consumer leases", h.opens.Load())
			}
		})
	}
}

// A refused replay must retain its correlation identity through reconstruction,
// then answer from current authority rather than the pre-fence cache.
func TestRouteViewRuntimeFencedReplayRetainsRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	h := newPublicationResetHarness(t, ctx, "reset")
	var armed atomic.Bool
	h.plugin = routeViewRuntimePeer(t, ctx, func(request *rpc.Request) (any, error) {
		if request.Method == rpc.MethodRouteViewPublish {
			var input rpc.RouteViewPublishInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, err
			}
			batch, err := routeViewDecodeOutputBatch(input.Batch)
			if err != nil {
				return nil, err
			}
			if batch.ReplayID == replay.Broadcast {
				if armed.CompareAndSwap(true, false) {
					if err := h.interleave(input.Session); err != nil {
						return nil, err
					}
					close(h.interleaving)
				}
			}
		}
		return h.answer(request)
	})
	replayed := make(chan sysribevents.BestChangeBatch, 4)
	sysribevents.BestChange.Subscribe(h.bus, func(batch *sysribevents.BestChangeBatch) {
		if batch.ReplayID == replay.Broadcast {
			select {
			case replayed <- routeViewCloneOutputBatch(*batch):
			case <-ctx.Done():
			}
		}
	})
	client := h.start(t)
	closed := false
	defer func() {
		if !closed {
			closeRuntimeFeedClient(t, ctx, client)
		}
	}()
	h.await(t, client, h.oldPrefix, routeaction.Add, 1)
	worker := client.worker
	armed.Store(true)
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
	h.awaitInterleaving(t, client)
	h.await(t, client, h.freshPrefix, routeaction.Add, 3)
	select {
	case batch := <-replayed:
		if len(batch.Changes) != 1 {
			t.Fatalf("fresh replay has %d entries", len(batch.Changes))
		}
		entry := batch.Changes[0]
		if entry.Prefix != h.freshPrefix || entry.Metric != 3 {
			t.Fatalf("replay restored stale authority: %+v", entry)
		}
	case <-client.worker.done:
		t.Fatalf("refused replay killed the worker: %v", client.Failure())
	case <-ctx.Done():
		t.Fatal("reconstruction dropped the original replay identity")
	}
	if client.worker != worker {
		t.Fatal("replay recovery replaced the worker")
	}
	if h.refused.Load() != 1 || h.resetReads.Load() != 1 || h.opens.Load() != 2 {
		t.Fatalf("replay did not perform one scoped reset: refused=%d reset=%d open=%d", h.refused.Load(), h.resetReads.Load(), h.opens.Load())
	}
	if err := client.Close(ctx); err != nil {
		t.Fatal(err)
	}
	closed = true
}
