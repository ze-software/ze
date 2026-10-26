// Design: docs/architecture/plugin/rib-storage-design.md -- route server departure recovery

package rs

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestPeerDownDepartureWorkersBounded holds the engine inside every
// departure's recovery request, takes down 50 sources, then releases the
// engine.
//
// VALIDATES: further session downs start no goroutine (the count grows by
// fewer than five, not by one per down), handleStateDown returns without
// waiting on the engine, and a departure the engine holds does not hold the
// next sources' departures behind it: exactly departureWorkers of them are in
// the engine at once, the first sources to go down, and every source's
// departure reaches the engine once after the release.
// PREVENTS: one goroutine per session-down event (ai/rules/goroutine-lifecycle.md),
// and one slow departure serializing every later source's departure behind it.
func TestPeerDownDepartureWorkersBounded(t *testing.T) {
	const downs = 50
	rs := newTestRouteServer(t)
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })
	p := sdk.NewWithConn("rs-worker-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = p.Close() })
	rs.plugin = p
	entered := make(chan struct{}, downs)
	release := make(chan struct{})
	sources := make(chan string, downs)
	bridge.SetDispatchRPC(func(_ context.Context, _ string, raw json.RawMessage) (json.RawMessage, error) {
		var input rpc.UpdateRouteInput
		if err := json.Unmarshal(raw, &input); err != nil {
			return nil, err
		}
		source, _ := input.Meta["recovery-source"].(string)
		sources <- source
		entered <- struct{}{}
		<-release
		return json.RawMessage(`{"announced":0,"withdrawn":1}`), nil
	})
	bridge.SetReady()

	addresses := make([]string, downs)
	for i := range downs {
		addresses[i] = netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)}).String()
		rs.withdrawals[addresses[i]] = map[withdrawalKey]withdrawalEntry{
			{fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}: {},
		}
	}

	rs.handleStateDown(addresses[0])
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first departure never reached the engine")
	}
	before := runtime.NumGoroutine()
	returned := make(chan struct{})
	go func() {
		for _, address := range addresses[1:] {
			rs.handleStateDown(address)
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
	rs.stopDepartures()
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
	rs := newTestRouteServer(t)
	rs.stopDepartures()
	if rs.departures != nil {
		t.Fatal("stopDepartures started the departure workers with no departure queued")
	}
	if rs.enqueueDeparture("192.0.2.1", map[withdrawalKey]withdrawalEntry{}) {
		t.Fatal("enqueueDeparture after stopDepartures answered true")
	}
	if rs.departures != nil {
		t.Fatal("enqueueDeparture after stopDepartures started the departure workers")
	}
}
