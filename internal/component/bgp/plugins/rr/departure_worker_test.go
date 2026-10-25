// Design: docs/guide/route-reflection.md -- departure recovery on peer down

package rr

import (
	"context"
	"net"
	"net/netip"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestPeerDownUsesOneDepartureWorker holds the engine inside the first
// departure's recovery request, takes down 49 more sources, then releases the
// engine.
//
// VALIDATES: while a departure is in flight, further session downs start no
// goroutine (the goroutine count grows by fewer than five, not by one per
// down), handleStateDown returns without waiting on the engine, and the
// engine receives the departures one source at a time in the order the
// sources went down.
// PREVENTS: one goroutine per session-down event (ai/rules/goroutine-lifecycle.md),
// which a flapping fabric turns into an unbounded goroutine count, and
// departures racing each other to the engine out of order.
func TestPeerDownUsesOneDepartureWorker(t *testing.T) {
	const downs = 50
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })
	p := sdk.NewWithConn("rr-worker-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = p.Close() })
	entered := make(chan struct{}, downs)
	release := make(chan struct{})
	sources := make(chan string, downs)
	bridge.SetUpdateRouteSel(func(_ context.Context, _ *selector.Selector, _ string, meta map[string]any) (uint32, uint32, error) {
		source, _ := meta["recovery-source"].(string)
		sources <- source
		entered <- struct{}{}
		<-release
		return 0, 1, nil
	})
	bridge.SetReady()
	rr := &routeReflector{plugin: p, withdrawals: make(map[string]map[string]withdrawalInfo)}

	addresses := make([]string, downs)
	for i := range downs {
		addresses[i] = netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)}).String()
		rr.withdrawals[addresses[i]] = map[string]withdrawalInfo{
			"ipv4/unicast|10.0.0.0/24": {Family: family.IPv4Unicast, Prefix: "prefix 10.0.0.0/24"},
		}
	}

	rr.handleStateDown(addresses[0])
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first departure never reached the engine")
	}
	before := runtime.NumGoroutine()
	returned := make(chan struct{})
	go func() {
		for _, address := range addresses[1:] {
			rr.handleStateDown(address)
		}
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("handleStateDown waited on the engine while a departure was in flight")
	}
	if grown := runtime.NumGoroutine() - before; grown >= 5 {
		t.Fatalf("%d session downs grew the goroutine count by %d, want one departure worker", downs-1, grown)
	}

	close(release)
	rr.stopDepartures()
	close(sources)
	var got []string
	for source := range sources {
		got = append(got, source)
	}
	if !slices.Equal(got, addresses) {
		t.Fatalf("departures reached the engine in order %v, want the order the sources went down %v", got, addresses)
	}
}
