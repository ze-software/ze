// Design: docs/architecture/rib/unified-locrib.md -- publication order.
// Related: route_view_worker_output.go -- publish, the order both readers share.

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

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// publicationOrderChain is a three-level chain whose two dependents sort, and
// arrive, ahead of the device route they resolve through: first via a gateway
// inside second, second via a gateway inside the device route cover.
type publicationOrderChain struct {
	first, second, cover        netip.Prefix
	firstGateway, secondGateway netip.Addr
}

func newPublicationOrderChain() publicationOrderChain {
	return publicationOrderChain{
		first:         netip.MustParsePrefix("198.18.217.0/24"),
		second:        netip.MustParsePrefix("198.18.218.0/24"),
		cover:         netip.MustParsePrefix("198.18.219.0/24"),
		firstGateway:  netip.MustParseAddr("198.18.218.5"),
		secondGateway: netip.MustParseAddr("198.18.219.7"),
	}
}

// awaitPublicationOrder returns the order in which the targets first reached
// publication, across every batch, and requires each to be an install.
func awaitPublicationOrder(t *testing.T, ctx context.Context, published <-chan outgoingBatch, targets ...netip.Prefix) []netip.Prefix {
	t.Helper()
	wanted := make(map[netip.Prefix]bool, len(targets))
	for _, target := range targets {
		wanted[target] = true
	}
	var order []netip.Prefix
	for len(order) < len(targets) {
		select {
		case batch := <-published:
			for _, change := range batch.Changes {
				if !wanted[change.Prefix] {
					continue
				}
				if change.Action != routeaction.Add {
					t.Fatalf("%s reached publication as %v, want an install: %+v", change.Prefix, change.Action, change)
				}
				wanted[change.Prefix] = false
				order = append(order, change.Prefix)
			}
		case <-ctx.Done():
			t.Fatalf("publication order incomplete: saw %v of %v", order, targets)
		}
	}
	return order
}

func requirePrefixOrder(t *testing.T, got []netip.Prefix, want ...netip.Prefix) {
	t.Helper()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FIB writer receives %v, want %v: a route ahead of its gateway's device route is refused ENETUNREACH", got, want)
		}
	}
}

// Goal: the selected reader (forked placement) publishes a delta page's chain
// cover-first, so a FIB writer that installs entry by entry reaches every
// gateway. Method: complete an empty snapshot, then deliver one delta page
// carrying [first, second, cover] in that explicit order, and read the order
// the daemon's publication receives.
func TestRouteViewSelectedPublishesCoverFirst(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	feed := newRouteViewRuntimeFeed(t, ctx)
	feed.pages <- runtimeSnapshot(1, true)
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	chain := newPublicationOrderChain()
	feed.pages <- rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 4, Phase: rpc.RouteViewPhaseDelta, Complete: true,
		Entries: []rpc.RouteViewEntry{
			runtimeSelected(chain.first, rpc.RouteViewPath{Protocol: "static", Distance: 10, NextHop: chain.firstGateway}),
			runtimeSelected(chain.second, rpc.RouteViewPath{Protocol: "static", Distance: 10, NextHop: chain.secondGateway}),
			runtimeSelected(chain.cover, rpc.RouteViewPath{Protocol: "static", Distance: 10, Interface: "port1"}),
		}}
	order := awaitPublicationOrder(t, ctx, feed.published, chain.first, chain.second, chain.cover)
	requirePrefixOrder(t, order, chain.cover, chain.first, chain.second)
}

// Goal: the local reader (internal placement) publishes one captured batch's
// chain cover-first. Method: hold the worker inside its initial publication,
// insert first, second and cover into the daemon Loc-RIB in that order so the
// worker drains all three keys as one batch in that order, release it, and
// read the order its native publication receives.
func TestRouteViewLocalPublishesCoverFirst(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	local := locrib.Default()
	if local == nil {
		t.Fatal("test process has no daemon authority")
	}
	source := redistevents.RegisterProtocol("static")
	chain := newPublicationOrderChain()
	blocker := netip.MustParsePrefix("198.18.216.0/24")
	routes := []struct {
		prefix netip.Prefix
		path   locrib.Path
	}{
		{chain.first, locrib.Path{Source: source, NextHop: chain.firstGateway}},
		{chain.second, locrib.Path{Source: source, NextHop: chain.secondGateway}},
		{chain.cover, locrib.Path{Source: source, Interface: "port1"}},
	}
	local.Insert(family.IPv4Unicast, blocker, locrib.Path{Source: source, RouteType: routetype.Blackhole})
	t.Cleanup(func() {
		local.Remove(family.IPv4Unicast, blocker, source, 0)
		for _, route := range routes {
			local.Remove(family.IPv4Unicast, route.prefix, source, route.path.Instance)
		}
	})
	published := make(chan outgoingBatch, 64)
	held := make(chan struct{})
	release := make(chan struct{})
	p := publicationOrderLocalPeer(t, ctx, published, blocker, held, release)
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
	case <-held:
	case <-ctx.Done():
		t.Fatal("the initial local publication never carried the blocker route")
	}
	for _, route := range routes {
		local.Insert(family.IPv4Unicast, route.prefix, route.path)
	}
	close(release)
	order := awaitPublicationOrder(t, ctx, published, chain.first, chain.second, chain.cover)
	requirePrefixOrder(t, order, chain.cover, chain.first, chain.second)
}

// publicationOrderLocalPeer answers the local consumer's lease over a direct
// bridge. The native publication that carries blocker closes held and waits
// for release; every other publication is forwarded to published.
func publicationOrderLocalPeer(t *testing.T, ctx context.Context, published chan<- outgoingBatch, blocker netip.Prefix, held, release chan struct{}) *sdk.Plugin {
	t.Helper()
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
			return json.Marshal(rpc.RouteViewOpenOutput{Session: "order-owner", Epoch: "daemon", Snapshot: "local"})
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			return json.Marshal(rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation})
		case rpc.MethodRouteViewClose:
			return json.Marshal(rpc.RouteViewCloseOutput{Session: "order-owner"})
		default:
			return nil, fmt.Errorf("unexpected local order method %q", method)
		}
	})
	var hold sync.Once
	bridge.SetDispatchNative(func(_ context.Context, method string, payload any) (any, bool, error) {
		if method != rpc.MethodRouteViewPublish {
			return nil, false, nil
		}
		input, valid := payload.(routeViewPublication)
		if !valid {
			return nil, true, fmt.Errorf("native publication is not a routing-view value: %T", payload)
		}
		blocking := false
		for i := range input.Batch.Changes {
			if input.Batch.Changes[i].Prefix == blocker {
				blocking = true
			}
		}
		acknowledged := rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}
		if blocking {
			hold.Do(func() { close(held) })
			select {
			case <-release:
			case <-ctx.Done():
				return nil, true, ctx.Err()
			}
			return acknowledged, true, nil
		}
		select {
		case published <- input.Batch:
		case <-ctx.Done():
			return nil, true, ctx.Err()
		}
		return acknowledged, true, nil
	})
	bridge.SetReady()
	return p
}
