// Design: docs/architecture/rib/unified-locrib.md -- routing-view runtime consumer.

package sysrib

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// A complete selected batch must reach the projection before any dependent
// route is resolved. A cover-only update must then re-evaluate that dependent.
func TestRouteViewRuntimeProjectionBatchBeforeResolution(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	feed := newRouteViewRuntimeFeed(t, ctx)
	prefix := netip.MustParsePrefix("198.18.212.0/24")
	cover := netip.MustParsePrefix("192.0.2.0/24")
	feed.pages <- runtimeSnapshot(3, true,
		runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, NextHop: netip.MustParseAddr("192.0.2.9")}),
		runtimeSelected(cover, rpc.RouteViewPath{Protocol: "static", Distance: 10, Interface: "port1", Metric: 3}),
	)
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	first := feed.awaitChange(t, ctx, prefix, false)
	if first.Action != routeaction.Add || first.NextHop != netip.MustParseAddr("192.0.2.9") || first.Interface != "port1" {
		t.Fatalf("route resolved before its complete reader batch: %+v", first)
	}
	feed.pages <- rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 4, Phase: rpc.RouteViewPhaseDelta, Complete: true,
		Entries: []rpc.RouteViewEntry{runtimeSelected(cover, rpc.RouteViewPath{Protocol: "static", Distance: 10, Interface: "port2", Metric: 4})}}
	updated := feed.awaitChange(t, ctx, prefix, false)
	if updated.Action != routeaction.Update || updated.Interface != "port2" {
		t.Fatalf("cover update did not cascade without reannouncement: %+v", updated)
	}
}

// A replay arriving before synchronization must be retained, and no output
// may escape from a partial selected snapshot.
func TestRouteViewRuntimeRetainsEarlyReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	feed := newRouteViewRuntimeFeed(t, ctx)
	prefix := netip.MustParsePrefix("198.18.213.0/24")
	feed.pages <- runtimeSnapshot(1, false, runtimeSelected(prefix, rpc.RouteViewPath{Protocol: "static", Distance: 10, RouteType: uint8(routetype.Blackhole)}))
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
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
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-feed.firstRead:
	case <-ctx.Done():
		t.Fatal("consumer never requested its initial snapshot")
	}
	feed.pages <- runtimeSnapshot(2, true)
	change := feed.awaitChange(t, ctx, prefix, true)
	if change.Action != routeaction.Add || change.RouteType != routetype.Blackhole {
		t.Fatalf("early replay was not reconstructed from the complete selection: %+v", change)
	}
}

// Internal output must remain a self-contained native value. A missing native
// publisher cannot silently select the raw JSON fallback and pass this test.
func TestRouteViewRuntimeInternalPublicationUsesNativeValues(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	local := locrib.Default()
	if local == nil {
		t.Fatal("test process has no daemon authority")
	}
	prefix := netip.MustParsePrefix("198.18.214.0/24")
	path := locrib.Path{Source: redistevents.RegisterProtocol("static"), RouteType: routetype.Blackhole}
	local.Insert(family.IPv4Unicast, prefix, path)
	t.Cleanup(func() { local.Remove(family.IPv4Unicast, prefix, path.Source, path.Instance) })
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	p := sdk.NewWithConn("rib", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = p.Close(); _ = engineEnd.Close() })
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		switch method {
		case rpc.MethodRouteViewOpen:
			var input rpc.RouteViewOpenInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			if input.Reader != rpc.RouteViewReaderLocalAuthority {
				return nil, fmt.Errorf("internal consumer did not request its local reader")
			}
			return json.Marshal(rpc.RouteViewOpenOutput{Session: "native-owner", Epoch: "daemon", Snapshot: "local"})
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			return json.Marshal(rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation})
		case rpc.MethodRouteViewClose:
			return json.Marshal(rpc.RouteViewCloseOutput{Session: "native-owner"})
		default:
			return nil, fmt.Errorf("internal hot path used raw method %q", method)
		}
	})
	published := make(chan outgoingChange, 32)
	bridge.SetDispatchNative(func(_ context.Context, method string, payload any) (any, bool, error) {
		if method != rpc.MethodRouteViewPublish {
			return nil, false, nil
		}
		input, value := payload.(routeViewPublication)
		if !value || input.Session != "native-owner" || input.Cursor != 0 || input.Operation == 0 {
			return nil, true, fmt.Errorf("native publication is not a complete owned value: %T", payload)
		}
		for i := range input.Batch.Changes {
			if input.Batch.Changes[i].Prefix == prefix {
				published <- input.Batch.Changes[i]
			}
		}
		return rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}, true, nil
	})
	bridge.SetReady()
	distances, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	permit, err := parseFIBImportConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	client, err := openRouteViewClient(ctx, p, newSysRIB(), distances, permit)
	if err != nil {
		t.Fatal(err)
	}
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case change := <-published:
		if change.Action != routeaction.Add || change.Protocol != "static" || change.RouteType != routetype.Blackhole {
			t.Fatalf("native path changed forwarding metadata: %+v", change)
		}
	case <-ctx.Done():
		t.Fatal("internal selected route never reached native publication")
	}
}

type routeViewRuntimeFeed struct {
	ctx       context.Context
	plugin    *sdk.Plugin
	pages     chan rpc.RouteViewReadOutput
	published chan outgoingBatch
	firstRead chan struct{}
	first     sync.Once
	cursor    uint64
	complete  bool
}

func newRouteViewRuntimeFeed(t *testing.T, ctx context.Context) *routeViewRuntimeFeed {
	t.Helper()
	feed := &routeViewRuntimeFeed{ctx: ctx, pages: make(chan rpc.RouteViewReadOutput, 8), published: make(chan outgoingBatch, 64), firstRead: make(chan struct{})}
	feed.plugin = routeViewRuntimePeer(t, ctx, feed.answer)
	return feed
}

func (feed *routeViewRuntimeFeed) answer(request *rpc.Request) (any, error) {
	switch request.Method {
	case rpc.MethodRouteViewOpen:
		return rpc.RouteViewOpenOutput{Session: "runtime-owner", Epoch: "daemon", Snapshot: "initial"}, nil
	case rpc.MethodRouteViewDistances:
		var input rpc.RouteViewDistancesInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		return rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation, Cursor: feed.cursor}, nil
	case rpc.MethodRouteViewRead:
		var input rpc.RouteViewReadInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		feed.first.Do(func() { close(feed.firstRead) })
		wait := time.NewTimer(time.Duration(input.WaitMillis) * time.Millisecond)
		defer wait.Stop()
		select {
		case <-feed.ctx.Done():
			return nil, feed.ctx.Err()
		case page := <-feed.pages:
			feed.cursor = page.Cursor
			feed.complete = page.Complete
			return page, nil
		case <-wait.C:
			if !feed.complete {
				select {
				case page := <-feed.pages:
					feed.cursor, feed.complete = page.Cursor, page.Complete
					return page, nil
				case <-feed.ctx.Done():
					return nil, feed.ctx.Err()
				}
			}
			return rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: feed.cursor, Phase: rpc.RouteViewPhaseDelta, Complete: true}, nil
		}
	case rpc.MethodRouteViewPublish:
		if !feed.complete {
			return nil, fmt.Errorf("consumer published from a partial selected snapshot")
		}
		var input rpc.RouteViewPublishInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		var batch outgoingBatch
		if err := json.Unmarshal(input.Batch, &batch); err != nil {
			return nil, err
		}
		select {
		case feed.published <- batch:
		case <-feed.ctx.Done():
			return nil, feed.ctx.Err()
		}
		return rpc.RouteViewPublishOutput{Session: input.Session, Operation: input.Operation, Cursor: input.Cursor}, nil
	case rpc.MethodRouteViewClose:
		return rpc.RouteViewCloseOutput{Session: "runtime-owner"}, nil
	default:
		return nil, fmt.Errorf("unexpected runtime fixture method %q", request.Method)
	}
}

func runtimeSelected(prefix netip.Prefix, path rpc.RouteViewPath) rpc.RouteViewEntry {
	return rpc.RouteViewEntry{AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast), Prefix: prefix, Action: rpc.RouteViewActionAdd, Path: path}
}

func runtimeSnapshot(cursor uint64, complete bool, entries ...rpc.RouteViewEntry) rpc.RouteViewReadOutput {
	return rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Snapshot: "initial", Cursor: cursor, Phase: rpc.RouteViewPhaseSnapshot, Complete: complete, Entries: entries}
}

func openRuntimeFeedClient(t *testing.T, ctx context.Context, feed *routeViewRuntimeFeed) *routeViewClient {
	t.Helper()
	distances, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	permit, err := parseFIBImportConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	client, err := openRouteViewClient(ctx, feed.plugin, newSysRIB(), distances, permit)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func closeRuntimeFeedClient(t *testing.T, ctx context.Context, client *routeViewClient) {
	t.Helper()
	if err := client.Close(ctx); err != nil {
		t.Error(err)
	}
}

func (feed *routeViewRuntimeFeed) awaitChange(t *testing.T, ctx context.Context, prefix netip.Prefix, replayed bool) outgoingChange {
	t.Helper()
	for {
		select {
		case batch := <-feed.published:
			if replay.IsReplay(batch.ReplayID) != replayed {
				continue
			}
			for i := range batch.Changes {
				if batch.Changes[i].Prefix == prefix {
					return batch.Changes[i]
				}
			}
		case <-ctx.Done():
			t.Fatalf("selected prefix %s never reached the requested live/replay publication", prefix)
			return outgoingChange{}
		}
	}
}
