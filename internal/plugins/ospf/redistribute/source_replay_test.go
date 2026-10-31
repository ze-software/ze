package ospfredistribute

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
	ospfredistevents "github.com/ze-software/ze/internal/plugins/ospf/redistribute/events"
	"github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/pkg/ze"
)

// The tests in this file judge the OSPF producer's answer to the redistribute
// orchestrator's ReplayRequest. The orchestrator's reload and rollback
// reconciliation, like its late-join replay, emits that request and judges
// only the routes the producers send back, so a producer that ignores it
// leaves a widened import chain with nothing to admit until the next SPF
// delta. Each test drives the Source through a synchronous bus, the way the
// engine bus delivers to in-process subscribers.

// replayBus is a synchronous ze.EventBus that records every route-change
// batch in emit order and delivers to its subscribers on the emitting
// goroutine. Safe for concurrent use, except that onEmit MUST be set before
// the first Emit it should see.
type replayBus struct {
	mu      sync.Mutex
	batches []recordedBatch
	subs    map[string][]func(any)
	// onEmit, when set, runs on the emitting goroutine for every route-change
	// batch, outside the bus's own lock, so a test can probe what the producer
	// holds at the moment of the emit.
	onEmit func(*redistevents.RouteChangeBatch)
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
	if batch, ok := payload.(*redistevents.RouteChangeBatch); ok && b.onEmit != nil {
		b.onEmit(batch)
	}
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

// TestOSPFSourceAnswersReloadReplay proves the OSPF Source answers the
// ReplayRequest a reload's reconciliation emits with its current SPF route
// set, tagged with the request's ReplayID.
//
// VALIDATES: a route an SPF delta added and no later delta removed is
// re-emitted as an add carrying its next hop and metric; a route a later
// delta removed is absent; the batch carries the ospf ProtocolID, IPv4 and the
// echoed ReplayID; an unsubscribed Source answers nothing.
// PREVENTS: BGP importing from ospf getting nothing after a reload widens the
// chain, until the next SPF delta happens to announce the route again.
func TestOSPFSourceAnswersReloadReplay(t *testing.T) {
	bus := newReplayBus()
	source := NewSource(bus)
	unsubscribe := source.SubscribeReplay()

	source.OnSPFChange(spf.RouteDelta{Added: []spf.RouteEntry{
		extRoute("10.10.0.0/24", 10, "10.0.0.2", spf.RouteIntraArea),
		extRoute("10.20.0.0/24", 25, "10.0.0.3", spf.RouteInterArea),
	}})
	source.OnSPFChange(spf.RouteDelta{Removed: []netip.Prefix{netip.MustParsePrefix("10.20.0.0/24")}})
	requestReplay(t, bus, 42)

	got := bus.recorded()
	require.Len(t, got, 3, "two incremental batches, then one replay answer")
	answer := got[2]
	assert.Equal(t, uint64(42), answer.replayID, "the answer echoes the request's ReplayID")
	assert.Equal(t, ospfredistevents.ProtocolID, answer.protocol)
	assert.Equal(t, uint16(family.AFIIPv4), answer.afi)
	require.Len(t, answer.entries, 1, "only the route no delta removed is replayed")
	entry := answer.entries[0]
	assert.Equal(t, redistevents.ActionAdd, entry.Action)
	assert.Equal(t, netip.MustParsePrefix("10.10.0.0/24"), entry.Prefix)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), entry.NextHop)
	assert.Equal(t, uint32(10), entry.Metric)

	unsubscribe()
	requestReplay(t, bus, 43)
	assert.Len(t, bus.recorded(), 3, "an unsubscribed Source answers no replay")
}

// TestOSPFSourceReplayWithNoRouteEmitsNothing proves an empty route set
// answers a replay with no batch, and a zero ReplayID is ignored.
//
// VALIDATES: Replay emits nothing when SPF holds no route, and nothing for
// the token 0, which marks an incremental batch rather than a request.
// PREVENTS: an empty or zero-tagged batch the orchestrator would misroute.
func TestOSPFSourceReplayWithNoRouteEmitsNothing(t *testing.T) {
	bus := newReplayBus()
	source := NewSource(bus)
	source.SubscribeReplay()
	requestReplay(t, bus, 1)
	assert.Empty(t, bus.recorded(), "no route, no replay batch")

	source.OnSPFChange(spf.RouteDelta{Added: []spf.RouteEntry{extRoute("10.10.0.0/24", 10, "10.0.0.2", spf.RouteIntraArea)}})
	source.Replay(0)
	assert.Len(t, bus.recorded(), 1, "a zero ReplayID is not a request")
}

// TestOSPFSourceReplayNeverRevivesARemovedRoute proves a replay racing SPF
// deltas always carries the route set the deltas emitted before it.
//
// VALIDATES: with one goroutine adding and removing a prefix through
// OnSPFChange and another answering replays, every replay batch, read in emit
// order, equals the set the incremental batches before it built.
// PREVENTS: a replay that snapshots the set, loses the race to a delta that
// removes a route, then emits the stale add after that removal, reviving a
// route OSPF no longer has. Run under -race. This is the model check: it
// catches an out-of-lock emit only when the scheduler hits the gap, so
// TestOSPFSourceReplayEmitsUnderLock is the deterministic discriminator.
func TestOSPFSourceReplayNeverRevivesARemovedRoute(t *testing.T) {
	const rounds = 2000
	bus := newReplayBus()
	source := NewSource(bus)
	source.SubscribeReplay()
	stable := extRoute("10.10.0.0/24", 10, "10.0.0.2", spf.RouteIntraArea)
	flapping := extRoute("10.20.0.0/24", 20, "10.0.0.3", spf.RouteIntraArea)
	source.OnSPFChange(spf.RouteDelta{Added: []spf.RouteEntry{stable}})

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

// TestOSPFSourceReplayEmitsUnderLock proves Replay emits its batch while
// holding mu, the lock every SPF delta records and emits under, so no delta
// can remove a route between the replay's read of the set and its emit.
//
// VALIDATES: at the moment of every replay emit, the bus probes mu with
// TryLock and fails to take it; the probe ran once per replay.
// PREVENTS: a replay that snapshots under mu and emits after releasing it.
// TestOSPFSourceReplayNeverRevivesARemovedRoute catches that only when the
// scheduler interleaves a delta in the gap; this probe catches it on every run.
func TestOSPFSourceReplayEmitsUnderLock(t *testing.T) {
	bus := newReplayBus()
	source := NewSource(bus)
	source.OnSPFChange(spf.RouteDelta{Added: []spf.RouteEntry{
		extRoute("10.10.0.0/24", 10, "10.0.0.2", spf.RouteIntraArea),
		extRoute("10.20.0.0/24", 20, "10.0.0.3", spf.RouteInterArea),
	}})
	probed, unlocked := 0, 0
	bus.onEmit = func(batch *redistevents.RouteChangeBatch) {
		if batch.ReplayID == 0 {
			return
		}
		probed++
		if source.mu.TryLock() {
			unlocked++
			source.mu.Unlock()
		}
	}

	source.Replay(7)

	require.Equal(t, 1, probed, "the replay emitted one batch")
	require.Zero(t, unlocked, "the replay emit holds the lock the SPF deltas hold")
}
