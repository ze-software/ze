package isisredistribute

import (
	"maps"
	"net/netip"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	isisredistevents "github.com/ze-software/ze/internal/plugins/isis/redistribute/events"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/pkg/ze"
)

// The tests in this file judge the IS-IS producer's answer to the redistribute
// orchestrator's ReplayRequest. The orchestrator's reload and rollback
// reconciliation, like its late-join replay, emits that request and judges
// only the routes the producers send back, so a producer that ignores it
// leaves a widened import chain with nothing to admit until the next SPF
// delta. Each test drives the Source through a synchronous bus, the way the
// engine bus delivers to in-process subscribers.

// replayBus is a synchronous ze.EventBus that records every route-change
// batch in emit order and delivers to its subscribers on the emitting
// goroutine. Safe for concurrent use.
type replayBus struct {
	mu      sync.Mutex
	batches []recordedBatch
	subs    map[string][]func(any)
}

// recordedBatch is a deep copy of one emitted batch: the producer recycles
// the pooled batch as soon as Emit returns.
type recordedBatch struct {
	protocol redistevents.ProtocolID
	afi      uint16
	replayID uint64
	entries  []redistevents.RouteChangeEntry
}

var _ ze.EventBus = (*replayBus)(nil)

func newReplayBus() *replayBus { return &replayBus{subs: map[string][]func(any){}} }

func (b *replayBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	if batch, ok := payload.(*redistevents.RouteChangeBatch); ok {
		b.batches = append(b.batches, recordedBatch{
			protocol: batch.Protocol,
			afi:      batch.AFI,
			replayID: batch.ReplayID,
			entries:  append([]redistevents.RouteChangeEntry(nil), batch.Entries...),
		})
	}
	handlers := append([]func(any){}, b.subs[namespace+"/"+eventType]...)
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return 0, nil
}

func (b *replayBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := namespace + "/" + eventType
	b.subs[key] = append(b.subs[key], handler)
	index := len(b.subs[key]) - 1
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.subs[key][index] = func(any) {}
	}
}

func (b *replayBus) recorded() []recordedBatch {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]recordedBatch(nil), b.batches...)
}

// requestReplay emits a ReplayRequest the way the orchestrator's fire does.
func requestReplay(t *testing.T, bus ze.EventBus, replayID uint64) {
	t.Helper()
	_, err := redistevents.ReplayRequestEvent.Emit(bus, &redistevents.ReplayRequest{ReplayID: replayID})
	require.NoError(t, err)
}

// entrySet maps a batch's entries by prefix.
func entrySet(entries []redistevents.RouteChangeEntry) map[netip.Prefix]redistevents.RouteChangeEntry {
	out := make(map[netip.Prefix]redistevents.RouteChangeEntry, len(entries))
	for _, e := range entries {
		out[e.Prefix] = e
	}
	return out
}

// isisRoute is an IS-IS SPF route through nextHop at metric.
func isisRoute(prefix string, metric uint64, nextHop string) spf.RouteEntry {
	return spf.RouteEntry{
		Prefix:   netip.MustParsePrefix(prefix),
		Metric:   metric,
		NextHops: []spf.NextHop{{Addr: netip.MustParseAddr(nextHop)}},
	}
}

// TestISISSourceAnswersReloadReplay proves the IS-IS Source answers the
// ReplayRequest a reload's reconciliation emits with its current SPF route
// set, one batch per address family, tagged with the request's ReplayID.
//
// VALIDATES: IPv4 and IPv6 routes an SPF delta added and no later delta
// removed are re-emitted as adds carrying their next hop and metric, IPv4
// first; a route a later delta removed is absent, and so is one whose next
// hop the protocol suite rejected (emitted as a remove); each batch carries
// the isis ProtocolID and the echoed ReplayID; an unsubscribed Source answers
// nothing.
// PREVENTS: BGP importing from isis getting nothing after a reload widens the
// chain, until the next SPF delta happens to announce the route again.
func TestISISSourceAnswersReloadReplay(t *testing.T) {
	bus := newReplayBus()
	source := NewSource(bus)
	unsubscribe := source.SubscribeReplay()

	rejected := isisRoute("10.30.0.0/24", 30, "10.0.0.4")
	rejected.NextHops[0].Unsupported = true
	source.OnSPFChange(spf.RouteDelta{Added: []spf.RouteEntry{
		isisRoute("10.10.0.0/24", 10, "10.0.0.2"),
		isisRoute("10.20.0.0/24", 20, "10.0.0.3"),
	}})
	source.OnSPFChange(spf.RouteDelta{
		Changed: []spf.RouteEntry{rejected},
		Removed: []netip.Prefix{netip.MustParsePrefix("10.20.0.0/24")},
	})
	source.OnSPFChangeV6(spf.RouteDelta{Added: []spf.RouteEntry{isisRoute("2001:db8:1::/64", 15, "fe80::2")}})
	requestReplay(t, bus, 42)

	got := bus.recorded()
	require.Len(t, got, 5, "three incremental batches, then one replay answer per family")
	v4, v6 := got[3], got[4]
	for _, answer := range []recordedBatch{v4, v6} {
		assert.Equal(t, uint64(42), answer.replayID, "the answer echoes the request's ReplayID")
		assert.Equal(t, isisredistevents.ProtocolID, answer.protocol)
		require.Len(t, answer.entries, 1, "only the route no delta removed is replayed")
		assert.Equal(t, redistevents.ActionAdd, answer.entries[0].Action)
	}
	assert.Equal(t, uint16(family.AFIIPv4), v4.afi)
	assert.Equal(t, netip.MustParsePrefix("10.10.0.0/24"), v4.entries[0].Prefix)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), v4.entries[0].NextHop)
	assert.Equal(t, uint32(10), v4.entries[0].Metric)
	assert.Equal(t, uint16(family.AFIIPv6), v6.afi)
	assert.Equal(t, netip.MustParsePrefix("2001:db8:1::/64"), v6.entries[0].Prefix)
	assert.Equal(t, uint32(15), v6.entries[0].Metric)

	unsubscribe()
	requestReplay(t, bus, 43)
	assert.Len(t, bus.recorded(), 5, "an unsubscribed Source answers no replay")
}

// TestISISSourceReplayWithNoRouteEmitsNothing proves an empty route set
// answers a replay with no batch, and a zero ReplayID is ignored.
//
// VALIDATES: Replay emits nothing when SPF holds no route, and nothing for
// the token 0, which marks an incremental batch rather than a request.
// PREVENTS: an empty or zero-tagged batch the orchestrator would misroute.
func TestISISSourceReplayWithNoRouteEmitsNothing(t *testing.T) {
	bus := newReplayBus()
	source := NewSource(bus)
	source.SubscribeReplay()
	requestReplay(t, bus, 1)
	assert.Empty(t, bus.recorded(), "no route, no replay batch")

	source.OnSPFChange(spf.RouteDelta{Added: []spf.RouteEntry{isisRoute("10.10.0.0/24", 10, "10.0.0.2")}})
	source.Replay(0)
	assert.Len(t, bus.recorded(), 1, "a zero ReplayID is not a request")
}

// TestISISSourceReplayNeverRevivesARemovedRoute proves a replay racing SPF
// deltas always carries the route set the deltas emitted before it.
//
// VALIDATES: with one goroutine adding and removing a prefix through
// OnSPFChange and another answering replays, every replay batch, read in emit
// order, equals the set the incremental batches before it built.
// PREVENTS: a replay that snapshots the set, loses the race to a delta that
// removes a route, then emits the stale add after that removal, reviving a
// route IS-IS no longer has. Run under -race.
func TestISISSourceReplayNeverRevivesARemovedRoute(t *testing.T) {
	const rounds = 2000
	bus := newReplayBus()
	source := NewSource(bus)
	source.SubscribeReplay()
	flapping := isisRoute("10.20.0.0/24", 20, "10.0.0.3")
	source.OnSPFChange(spf.RouteDelta{Added: []spf.RouteEntry{isisRoute("10.10.0.0/24", 10, "10.0.0.2")}})

	var wg sync.WaitGroup
	wg.Go(func() {
		for range rounds {
			source.OnSPFChange(spf.RouteDelta{Added: []spf.RouteEntry{flapping}})
			source.OnSPFChange(spf.RouteDelta{Removed: []netip.Prefix{flapping.Prefix}})
			runtime.Gosched()
		}
	})
	wg.Go(func() {
		for i := range rounds {
			source.Replay(uint64(i + 1))
			runtime.Gosched()
		}
	})
	wg.Wait()

	model := map[netip.Prefix]redistevents.RouteChangeEntry{}
	replays := 0
	for _, batch := range bus.recorded() {
		if batch.replayID != 0 {
			replays++
			require.True(t, maps.Equal(model, entrySet(batch.entries)),
				"replay %d carries %v, the deltas before it built %v", batch.replayID, entrySet(batch.entries), model)
			continue
		}
		recordEntries(model, batch.entries)
	}
	assert.Equal(t, rounds, replays, "every replay was answered")
}
