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

	"encoding/json"
	"fmt"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestPeerDownDepartureWorkersBounded holds the engine inside every
// departure's recovery request, takes down 50 sources, then releases the
// engine.
//
// VALIDATES: further session downs start no goroutine (the goroutine count
// grows by fewer than five, not by one per down), handleStateDown returns
// without waiting on the engine, and a departure the engine holds does not
// hold the next sources' departures behind it: exactly departureWorkers of
// them are in the engine at once, the first sources to go down, and every
// source's departure reaches the engine once after the release.
// PREVENTS: one goroutine per session-down event (ai/rules/goroutine-lifecycle.md),
// which a flapping fabric turns into an unbounded goroutine count, and one
// slow departure serializing every later source's departure behind it.
func TestPeerDownDepartureWorkersBounded(t *testing.T) {
	const downs = 50
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })
	p := sdk.NewWithConn("rr-worker-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = p.Close() })
	entered := make(chan struct{}, downs)
	release := make(chan struct{})
	sources := make(chan string, downs)
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		if result, handled, err := rrFixtureLeaseRPC(method, params); handled {
			return result, err
		}
		var input rpc.UpdateRouteInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, err
		}
		if method != rpc.MethodUpdateRoute {
			return nil, fmt.Errorf("unexpected worker RPC %s", method)
		}
		source, ok := input.Meta["recovery-source"].(string)
		if !ok {
			return nil, fmt.Errorf("departure has no source: %+v", input)
		}
		sources <- source
		entered <- struct{}{}
		<-release
		return json.Marshal(rpc.UpdateRouteOutput{Withdrawn: 1})
	})
	bridge.SetReady()
	rr := &routeReflector{plugin: p, peers: make(map[peerIdentity]*peerState), withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
	rrFixtureStart(t, rr, rrOriginalFixture(t, "203.0.113.254", 900, 900), 0, 7, 1900)
	t.Cleanup(rr.stopDepartures)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	addresses := make([]string, downs)
	for i := range downs {
		addresses[i] = netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)}).String()
		original := rrOriginalFixture(t, addresses[i], uint64(1000+i), uint64(2000+i))
		rrFixtureStart(t, rr, original, uint64(3000+i), 7, uint64(4000+i))
		rr.withdrawals[addresses[i]] = map[withdrawalKey]withdrawalInfo{
			{owner: original.owner, connection: original.connection, route: "ipv4/unicast|10.0.0.0/24"}: {SourceOwner: original.owner, ConnectionID: original.connection, SourceReceipt: uint64(3000 + i), Family: family.IPv4Unicast, Prefix: "prefix 10.0.0.0/24"},
		}
	}

	for _, state := range rr.peers {
		if state.Identity.owner != 900 {
			state.DestinationSession = 0
		}
	}
	rr.handleStateDown(rrOriginalFixture(t, addresses[0], 1000, 2000), 7)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first departure never reached the engine")
	}
	before := runtime.NumGoroutine()
	returned := make(chan struct{})
	go func() {
		for index, address := range addresses[1:] {
			rr.handleStateDown(rrOriginalFixture(t, address, uint64(1001+index), uint64(2001+index)), 7)
		}
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("handleStateDown waited on the engine while a departure was in flight")
	}
	if grown := runtime.NumGoroutine() - before; grown >= 5 {
		t.Fatalf("%d session downs grew the goroutine count by %d, want under 5: downs queue on the already started worker set and spawn nothing", downs-1, grown)
	}

	inEngine := 1
	deadline := time.After(5 * time.Second)
	for inEngine < departureWorkers {
		select {
		case <-entered:
			inEngine++
		case <-deadline:
			t.Fatalf("%d departures in the engine at once, want %d: a held departure serialized the others", inEngine, departureWorkers)
		}
	}
	select {
	case <-entered:
		t.Fatalf("more than %d departures in the engine at once, want the worker bound", departureWorkers)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	rr.stopDepartures()
	close(sources)
	var got []string
	for source := range sources {
		got = append(got, source)
	}
	first := slices.Clone(got[:departureWorkers])
	slices.Sort(first)
	want := slices.Clone(addresses[:departureWorkers])
	slices.Sort(want)
	if !slices.Equal(first, want) {
		t.Fatalf("first departures in the engine %v, want the first sources to go down %v", first, want)
	}
	slices.Sort(got)
	all := slices.Clone(addresses)
	slices.Sort(all)
	if !slices.Equal(got, all) {
		t.Fatalf("departures reached the engine for %v, want each source once %v", got, all)
	}
}

// TestStopDeparturesWithoutDepartureStartsNothing stops the departure workers
// of an owner that never queued a departure, then queues one.
//
// VALIDATES: stopDepartures starts no worker when none was started, and a
// departure queued after it is refused.
// PREVENTS: shutdown starting departure workers only to stop them.
func TestStopDeparturesWithoutDepartureStartsNothing(t *testing.T) {
	rr := &routeReflector{withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
	rr.stopDepartures()
	if rr.departures != nil {
		t.Fatal("stopDepartures started the departure workers with no departure queued")
	}
	if rr.enqueueDeparture(rrOriginalFixture(t, "192.0.2.1", 701, 41), map[withdrawalKey]withdrawalInfo{}) {
		t.Fatal("enqueueDeparture after stopDepartures answered true")
	}
	if rr.departures != nil {
		t.Fatal("enqueueDeparture after stopDepartures started the departure workers")
	}
}
