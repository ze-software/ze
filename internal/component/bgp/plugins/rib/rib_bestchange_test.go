package rib

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ribevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/events"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/locrib"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// testEvent records one event emitted on the in-memory test EventBus.
type testEvent struct {
	Namespace string
	EventType string
	Payload   any
}

// testEventBus is a minimal ze.EventBus implementation for unit tests.
// It records every Emit so test assertions can inspect the namespace,
// event-type, and payload. Subscribe stores handlers per-key and
// dispatches them synchronously on Emit.
type testEventBus struct {
	mu       sync.Mutex
	events   []testEvent
	handlers map[string][]func(any)
}

func newTestEventBus() *testEventBus {
	return &testEventBus{
		handlers: make(map[string][]func(any)),
	}
}

func (b *testEventBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	b.events = append(b.events, testEvent{Namespace: namespace, EventType: eventType, Payload: payload})
	hs := append([]func(any){}, b.handlers[namespace+"/"+eventType]...)
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return 0, nil
}

func (b *testEventBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	if handler == nil {
		return func() {}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	key := namespace + "/" + eventType
	b.handlers[key] = append(b.handlers[key], handler)
	return func() {}
}

func (b *testEventBus) lastEvent() *testEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.events) == 0 {
		return nil
	}
	return &b.events[len(b.events)-1]
}

func (b *testEventBus) eventCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

// newTestRIBManagerWithBus creates a RIBManager with the test EventBus wired in.
// Name is preserved from the original test for traceability.
func newTestRIBManagerWithBus(eb *testEventBus) *RIBManager {
	SetEventBus(eb)
	return NewRIBManager(nil)
}

// makeAttrBytes builds minimal attribute wire bytes for testing.
// ORIGIN(IGP) + NEXT_HOP(nhIP).
func makeAttrBytes(nhIP [4]byte) []byte {
	// ORIGIN: flags=0x40, type=1, len=1, value=0 (IGP)
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	// NEXT_HOP: flags=0x40, type=3, len=4, value=nhIP
	nextHop := []byte{0x40, 0x03, 0x04, nhIP[0], nhIP[1], nhIP[2], nhIP[3]}
	return append(origin, nextHop...)
}

// ipv4Prefix builds wire-format NLRI bytes for an IPv4 prefix.
// Example: ipv4Prefix(24, 10, 0, 0) produces 10.0.0.0/24.
func ipv4Prefix(prefLen byte, octets ...byte) []byte {
	result := []byte{prefLen}
	return append(result, octets...)
}

// VALIDATES: AC-1 -- BGP UPDATE makes prefix best path change, RIB emits
// (rib, best-change) with action "add" and correct next-hop.
// PREVENTS: Best-path changes going undetected.
func TestRIBBestChangePublish(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	// Set up peer metadata for eBGP.
	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	// Insert a route from peer.
	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	// Check best-path change under lock.
	change, ok := r.checkBestPathChange(fam, prefix, false, nil)

	require.True(t, ok, "should detect new best path")
	assert.Equal(t, ribevents.BestChangeAdd, change.Action)
	assert.Equal(t, "10.0.0.0/24", change.Prefix)
	assert.Equal(t, "192.168.1.1", change.NextHop)
	assert.Equal(t, 20, change.Priority, "eBGP should have priority 20")

	// Publish and verify the EventBus event.
	publishBestChanges([]bestChangeEntry{change}, fam.String())

	evt := bus.lastEvent()
	require.NotNil(t, evt)
	assert.Equal(t, "bgp-rib", evt.Namespace)
	assert.Equal(t, ribevents.EventBestChange, evt.EventType)

	batchPtr, ok := evt.Payload.(*bestChangeBatch)
	require.True(t, ok, "expected *bestChangeBatch payload, got %T", evt.Payload)
	batch := *batchPtr
	assert.Equal(t, "bgp", batch.Protocol)
	assert.Equal(t, "ipv4/unicast", batch.Family)
	require.Len(t, batch.Changes, 1)
	assert.Equal(t, ribevents.BestChangeAdd, batch.Changes[0].Action)
	assert.Equal(t, "10.0.0.0/24", batch.Changes[0].Prefix)
	assert.Equal(t, "192.168.1.1", batch.Changes[0].NextHop)
}

// TestPurgeBestPrevForPeer validates that a peer-down purge drops every
// bestPrev record belonging to the departing peer, mirrors the
// withdrawals into locrib, and emits a best-change Withdraw event per
// purged prefix. Records belonging to a different peer must survive.
//
// VALIDATES: bestprev-purge on peer-DOWN; cross-protocol consumers see
// the withdrawal immediately instead of waiting for a later UPDATE per
// prefix.
// PREVENTS: a peer-DOWN handler regression that leaves stale best-path
// records referencing a departed peer -- the "show bgp rib best"
// staleness reported at the TODO(bestprev-purge) removal.
func TestPurgeBestPrevForPeer(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	fam := family.Family{AFI: 1, SAFI: 1}
	leavingPeer := "192.0.2.1"
	survivingPeer := "192.0.2.2"

	// Seed two bestPrev records for the leaving peer, plus one for the
	// surviving peer. Go through checkBestPathChange so the interner
	// learns both peers naturally.
	r.peerUp[leavingPeer] = true
	r.peerUp[survivingPeer] = true
	r.peerMeta[leavingPeer] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}
	r.peerMeta[survivingPeer] = &PeerMeta{PeerASN: 65002, LocalASN: 65000}
	r.ribInPool[leavingPeer] = storage.NewPeerRIB(leavingPeer)
	r.ribInPool[survivingPeer] = storage.NewPeerRIB(survivingPeer)

	leavingPrefixes := [][]byte{
		ipv4Prefix(24, 10, 0, 0), // 10.0.0.0/24
		ipv4Prefix(24, 10, 1, 0), // 10.1.0.0/24
		ipv4Prefix(24, 10, 2, 0), // 10.2.0.0/24
	}
	survivingPrefix := ipv4Prefix(24, 10, 3, 0) // 10.3.0.0/24

	attrsLeaving := makeAttrBytes([4]byte{192, 168, 1, 1})
	attrsSurviving := makeAttrBytes([4]byte{192, 168, 2, 2})

	for _, p := range leavingPrefixes {
		r.ribInPool[leavingPeer].Insert(fam, attrsLeaving, p)
		_, ok := r.checkBestPathChange(fam, p, false, nil)
		require.True(t, ok, "seed checkBestPathChange must record the prefix")
	}
	r.ribInPool[survivingPeer].Insert(fam, attrsSurviving, survivingPrefix)
	_, ok := r.checkBestPathChange(fam, survivingPrefix, false, nil)
	require.True(t, ok, "seed surviving prefix must record")

	// Sanity: four bestPrev records total across all shards for fam.
	depthsBefore := r.bestPrev.shardDepth(fam)
	totalBefore := 0
	for _, d := range depthsBefore {
		totalBefore += d
	}
	require.Equal(t, 4, totalBefore, "expected four bestPrev records before purge")

	// Trigger DOWN for the leaving peer. Use handleStructuredState so
	// the whole DOWN flow runs (ribInPool delete + peerMeta delete +
	// purge under peerMu).
	r.handleStructuredState(&rpc.StructuredEvent{
		PeerAddress: leavingPeer,
		State:       "down",
	})

	// The surviving peer's one record must remain; the leaving peer's
	// three records must be gone. Total: one.
	depthsAfter := r.bestPrev.shardDepth(fam)
	totalAfter := 0
	for _, d := range depthsAfter {
		totalAfter += d
	}
	assert.Equal(t, 1, totalAfter, "only the surviving peer's record should remain")

	// Every purged prefix must have surfaced on the EventBus as a
	// Withdraw. The surviving prefix must NOT appear.
	require.GreaterOrEqual(t, bus.eventCount(), 1, "expected at least one best-change event")
	var withdrawnPrefixes []string
	bus.mu.Lock()
	for i := range bus.events {
		batch, ok := bus.events[i].Payload.(*bestChangeBatch)
		if !ok {
			continue
		}
		for _, c := range batch.Changes {
			if c.Action == ribevents.BestChangeWithdraw {
				withdrawnPrefixes = append(withdrawnPrefixes, c.Prefix)
			}
		}
	}
	bus.mu.Unlock()
	assert.ElementsMatch(t,
		[]string{"10.0.0.0/24", "10.1.0.0/24", "10.2.0.0/24"},
		withdrawnPrefixes,
		"purge must emit a Withdraw for every leaving-peer prefix",
	)
	assert.NotContains(t, withdrawnPrefixes, "10.3.0.0/24", "surviving prefix must not be withdrawn")
}

// TestPurgeBestPrevForPeerUnknown validates the no-op path when the peer
// was never interned (no records could reference it).
func TestPurgeBestPrevForPeerUnknown(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	// No seed -- interner is empty.
	r.purgeBestPrevForPeer("203.0.113.99")

	assert.Zero(t, bus.eventCount(), "no events for a peer never interned")
}

// VALIDATES: AC-3 -- BGP UPDATE does not change best path, no event published.
// PREVENTS: Spurious EventBus events when best path is unchanged.
func TestRIBBestChangeNoPublishSameBest(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	// First check: detects new best.
	_, ok1 := r.checkBestPathChange(fam, prefix, false, nil)
	require.True(t, ok1)

	// Re-insert same route (implicit withdraw + re-add with same attrs).
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	// Second check: same best, no change.
	_, ok2 := r.checkBestPathChange(fam, prefix, false, nil)
	assert.False(t, ok2, "no change when best path is unchanged")
}

// VALIDATES: AC-2 -- BGP withdraws last route for prefix, RIB emits
// (rib, best-change) with action "withdraw".
// PREVENTS: Withdraw events not being published.
func TestRIBBestChangeWithdraw(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	// Establish best path.
	r.checkBestPathChange(fam, prefix, false, nil)

	// Withdraw the route.
	r.ribInPool[peerAddr].Remove(fam, prefix)

	change, ok := r.checkBestPathChange(fam, prefix, false, nil)

	require.True(t, ok, "should detect withdraw")
	assert.Equal(t, ribevents.BestChangeWithdraw, change.Action)
	assert.Equal(t, "10.0.0.0/24", change.Prefix)
	assert.Empty(t, change.NextHop, "withdraw should have no next-hop")
}

// VALIDATES: AC-11 -- Peer goes down, all its routes withdrawn. RIB emits
// single batch event with withdraws for all affected prefixes.
// PREVENTS: Peer-down producing individual events per prefix.
func TestRIBBestChangeBatchPeerDown(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	fam := family.Family{AFI: 1, SAFI: 1}
	prefixes := [][]byte{
		ipv4Prefix(24, 10, 0, 0),
		ipv4Prefix(16, 172, 16),
		ipv4Prefix(8, 192),
	}
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	for _, p := range prefixes {
		r.ribInPool[peerAddr].Insert(fam, attrs, p)
	}

	// Establish best paths for all prefixes.
	for _, p := range prefixes {
		r.checkBestPathChange(fam, p, false, nil)
	}

	// Simulate peer down: withdraw all routes.
	for _, p := range prefixes {
		r.ribInPool[peerAddr].Remove(fam, p)
	}

	// Collect all changes in one batch (under lock).
	var changes []bestChangeEntry
	for _, p := range prefixes {
		if change, ok := r.checkBestPathChange(fam, p, false, nil); ok {
			changes = append(changes, change)
		}
	}

	// Publish as single batch.
	require.Len(t, changes, 3, "should have 3 withdraw changes")
	publishBestChanges(changes, fam.String())

	// Verify single event with 3 withdrawals.
	assert.Equal(t, 1, bus.eventCount(), "should be a single batch event")
	evt := bus.lastEvent()
	require.NotNil(t, evt)

	batchPtr, ok := evt.Payload.(*bestChangeBatch)
	require.True(t, ok, "expected *bestChangeBatch payload, got %T", evt.Payload)
	batch := *batchPtr
	assert.Len(t, batch.Changes, 3)
	for _, c := range batch.Changes {
		assert.Equal(t, ribevents.BestChangeWithdraw, c.Action)
	}
}

// VALIDATES: AC-6 -- eBGP route has protocol-type "ebgp" in best-change entry.
// PREVENTS: sysrib unable to distinguish eBGP from iBGP for admin distance.
func TestRIBBestChangeEBGPMetadata(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	change, ok := r.checkBestPathChange(fam, prefix, false, nil)

	require.True(t, ok)
	assert.Equal(t, "ebgp", change.ProtocolType, "eBGP route must have protocol-type 'ebgp'")

	// Verify it survives JSON round-trip (sysrib reads this from payload).
	data, err := json.Marshal(change)
	require.NoError(t, err)
	var decoded bestChangeEntry
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "ebgp", decoded.ProtocolType)
}

// VALIDATES: AC-7 -- iBGP route has protocol-type "ibgp" in best-change entry.
// PREVENTS: sysrib unable to distinguish iBGP from eBGP for admin distance.
func TestRIBBestChangeIBGPMetadata(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65000, LocalASN: 65000} // same AS = iBGP

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	change, ok := r.checkBestPathChange(fam, prefix, false, nil)

	require.True(t, ok)
	assert.Equal(t, "ibgp", change.ProtocolType, "iBGP route must have protocol-type 'ibgp'")
}

// VALIDATES: AC-1 (eBGP priority) -- eBGP routes published with priority 20.
// PREVENTS: Wrong admin distance for eBGP routes.
func TestRIBBestChangeEBGPPriority(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	change, ok := r.checkBestPathChange(fam, prefix, false, nil)

	require.True(t, ok)
	assert.Equal(t, 20, change.Priority, "eBGP admin distance should be 20")
}

// VALIDATES: AC-1 (iBGP priority) -- iBGP routes published with priority 200.
// PREVENTS: Wrong admin distance for iBGP routes.
func TestRIBBestChangeIBGPPriority(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65000, LocalASN: 65000} // same AS = iBGP

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	change, ok := r.checkBestPathChange(fam, prefix, false, nil)

	require.True(t, ok)
	assert.Equal(t, 200, change.Priority, "iBGP admin distance should be 200")
}

// VALIDATES: AC-1 -- Best path changes when a better route arrives from another peer.
// PREVENTS: Best-path update events not being detected.
func TestRIBBestChangeUpdate(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)

	// Peer 1: iBGP route.
	peer1 := "192.0.2.1"
	r.peerMeta[peer1] = &PeerMeta{PeerASN: 65000, LocalASN: 65000}
	r.ribInPool[peer1] = storage.NewPeerRIB(peer1)
	r.ribInPool[peer1].Insert(fam, makeAttrBytes([4]byte{10, 0, 0, 1}), prefix)

	change1, ok1 := r.checkBestPathChange(fam, prefix, false, nil)
	require.True(t, ok1)
	assert.Equal(t, ribevents.BestChangeAdd, change1.Action)

	// Peer 2: eBGP route (better -- eBGP preferred over iBGP).
	peer2 := "192.0.2.2"
	r.peerMeta[peer2] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}
	r.ribInPool[peer2] = storage.NewPeerRIB(peer2)
	r.ribInPool[peer2].Insert(fam, makeAttrBytes([4]byte{10, 0, 0, 2}), prefix)

	change2, ok2 := r.checkBestPathChange(fam, prefix, false, nil)

	require.True(t, ok2, "should detect best-path update")
	assert.Equal(t, ribevents.BestChangeUpdate, change2.Action)
	assert.Equal(t, "10.0.0.2", change2.NextHop, "should switch to eBGP next-hop")
	assert.Equal(t, 20, change2.Priority, "eBGP priority")
	_ = bus
}

// VALIDATES: AC-23 -- System RIB subscribes after BGP RIB has computed best paths.
// Protocol RIB replays current best-path table as batch event.
// PREVENTS: Late-starting subscribers missing the initial state.
func TestRIBReplayOnSubscribe(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)

	// Set up a peer with a route and establish best path.
	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	r.checkBestPathChange(fam, prefix, false, nil)

	// Clear emitted events from the initial insert.
	bus.mu.Lock()
	bus.events = nil
	bus.mu.Unlock()

	// Now trigger replay (simulating a late subscriber requesting it).
	r.replayBestPaths()

	// Should have published a replay batch.
	evt := bus.lastEvent()
	require.NotNil(t, evt, "should publish replay batch")
	assert.Equal(t, "bgp-rib", evt.Namespace)
	assert.Equal(t, ribevents.EventBestChange, evt.EventType)

	batchPtr, ok := evt.Payload.(*bestChangeBatch)
	require.True(t, ok, "expected *bestChangeBatch payload, got %T", evt.Payload)
	batch := *batchPtr
	assert.Equal(t, "bgp", batch.Protocol)
	assert.True(t, batch.Replay, "replay batch must have Replay=true")
	require.Len(t, batch.Changes, 1)
	assert.Equal(t, ribevents.BestChangeAdd, batch.Changes[0].Action)
	assert.Equal(t, "192.168.1.1", batch.Changes[0].NextHop)
}

// VALIDATES: AC-4 -- packed bestPathRecord round-trips through pack/unpack.
// PREVENTS: Bit-layout regressions that silently corrupt stored records.
func TestBestPathRecordPackUnpack(t *testing.T) {
	cases := []struct {
		name                                 string
		metricIdx, peerIdx, nhIdx, flagsBits uint16
	}{
		{"all-zero", 0, 0, 0, 0},
		{"all-max", 0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF},
		{"typical-ebgp", 3, 42, 17, flagEBGP},
		{"typical-ibgp", 0, 7, 0, 0},
		{"distinct-fields", 0x1234, 0x5678, 0x9ABC, 0xDEF0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := packBestPath(tc.metricIdx, tc.peerIdx, tc.nhIdx, tc.flagsBits)
			assert.Equal(t, tc.metricIdx, rec.MetricIdx(), "metric idx round-trip")
			assert.Equal(t, tc.peerIdx, rec.PeerIdx(), "peer idx round-trip")
			assert.Equal(t, tc.nhIdx, rec.NextHopIdx(), "next-hop idx round-trip")
			assert.Equal(t, tc.flagsBits, rec.Flags(), "flags round-trip")
			assert.Equal(t, tc.flagsBits&flagEBGP != 0, rec.IsEBGP(), "IsEBGP matches flag bit 0")
		})
	}
}

// VALIDATES: AC-4 -- packed bestPathRecord equality is a single uint64 compare.
// PREVENTS: Same-best short-circuit breaking when fields differ.
func TestBestPathRecordEquality(t *testing.T) {
	base := packBestPath(3, 42, 17, flagEBGP)
	same := packBestPath(3, 42, 17, flagEBGP)
	assert.Equal(t, base, same, "identical fields compare equal")

	diffMetric := packBestPath(4, 42, 17, flagEBGP)
	diffPeer := packBestPath(3, 43, 17, flagEBGP)
	diffNH := packBestPath(3, 42, 18, flagEBGP)
	diffFlags := packBestPath(3, 42, 17, 0)
	assert.NotEqual(t, base, diffMetric, "metric differs")
	assert.NotEqual(t, base, diffPeer, "peer differs")
	assert.NotEqual(t, base, diffNH, "next-hop differs")
	assert.NotEqual(t, base, diffFlags, "flags differ (ebgp vs ibgp)")
}

// VALIDATES: AC-6 -- interner dedupes on repeat and grows the reverse table
// only on first sighting.
// PREVENTS: Reverse table bloating on every checkBestPathChange call.
func TestBestPrevInternerDedup(t *testing.T) {
	ir := newBestPrevInterner()

	p1, ok := ir.internPeer("192.0.2.1")
	require.True(t, ok)
	p2, ok := ir.internPeer("192.0.2.2")
	require.True(t, ok)
	p1dup, ok := ir.internPeer("192.0.2.1")
	require.True(t, ok)

	assert.NotEqual(t, p1, p2, "distinct peers get distinct indices")
	assert.Equal(t, p1, p1dup, "repeat returns the same index")
	assert.Len(t, ir.peers, 2, "reverse table grows only on new values")

	nh := netip.MustParseAddr("10.0.0.1")
	nhIdx, ok := ir.internNextHop(nh)
	require.True(t, ok)
	nhDupIdx, ok := ir.internNextHop(nh)
	require.True(t, ok)
	assert.Equal(t, nhIdx, nhDupIdx)
	assert.Len(t, ir.nextHops, 1)

	m1, ok := ir.internMetric(100)
	require.True(t, ok)
	m2, ok := ir.internMetric(200)
	require.True(t, ok)
	mDup, ok := ir.internMetric(100)
	require.True(t, ok)
	assert.NotEqual(t, m1, m2)
	assert.Equal(t, m1, mDup)
	assert.Len(t, ir.metrics, 2)
}

// VALIDATES: AC-6 -- reverse-table lookups return the originally interned value.
// PREVENTS: Emission path emitting stale or wrong values.
func TestBestPrevInternerReverse(t *testing.T) {
	ir := newBestPrevInterner()

	peers := []string{"192.0.2.1", "192.0.2.2", "198.51.100.7"}
	peerIdxs := make([]uint16, len(peers))
	for i, p := range peers {
		idx, ok := ir.internPeer(p)
		require.True(t, ok)
		peerIdxs[i] = idx
	}
	for i, idx := range peerIdxs {
		assert.Equal(t, peers[i], ir.peers[idx])
	}

	nhs := []netip.Addr{
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("2001:db8::1"),
		{}, // zero / invalid -- must round-trip via interner
	}
	nhIdxs := make([]uint16, len(nhs))
	for i, nh := range nhs {
		idx, ok := ir.internNextHop(nh)
		require.True(t, ok)
		nhIdxs[i] = idx
	}
	for i, idx := range nhIdxs {
		assert.Equal(t, nhs[i], ir.nextHops[idx])
	}

	metrics := []uint32{0, 100, 42, 1<<31 - 1}
	metricIdxs := make([]uint16, len(metrics))
	for i, m := range metrics {
		idx, ok := ir.internMetric(m)
		require.True(t, ok)
		metricIdxs[i] = idx
	}
	for i, idx := range metricIdxs {
		assert.Equal(t, metrics[i], ir.metrics[idx])
	}
}

// VALIDATES: AC-7 -- an interner saturated at 65536 entries returns (0, false)
// without panicking; subsequent checkBestPathChange calls return (zero, false)
// for the affected record. No panic is permitted in this path.
// PREVENTS: Architectural-unreachable overflow becoming a crash.
func TestBestPrevInternerOverflow(t *testing.T) {
	// Part 1: bare interner overflow on each table, no RIB involved.
	t.Run("bare-intern-overflow", func(t *testing.T) {
		ir := newBestPrevInterner()
		for i := range internerCap {
			_, ok := ir.internMetric(uint32(i))
			require.True(t, ok, "insertion %d within cap must succeed", i)
		}
		assert.Len(t, ir.metrics, internerCap)

		// Re-inserting an already-known value still succeeds (forward map hit).
		idx, ok := ir.internMetric(0)
		require.True(t, ok, "dedup hit bypasses the cap check")
		assert.Equal(t, uint16(0), idx)

		// A brand-new value must be rejected without panic.
		require.NotPanics(t, func() {
			_, ok := ir.internMetric(0xFFFFFFFF)
			assert.False(t, ok, "overflow returns (_, false)")
		})
	})

	// Part 2: checkBestPathChange drops a record when its peer cannot be
	// interned. Saturate the peer table with synthetic keys, then introduce
	// one more distinct peer and confirm the call returns (zero, false)
	// without panicking.
	t.Run("checkBestPathChange-drops-on-overflow", func(t *testing.T) {
		bus := newTestEventBus()
		r := newTestRIBManagerWithBus(bus)

		// Saturate the peer reverse table with distinct synthetic keys.
		for i := range internerCap {
			_, ok := r.bestPathInterner.internPeer(syntheticPeerKey(i))
			require.True(t, ok)
		}

		peerAddr := "192.0.2.1"
		r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}
		fam := family.Family{AFI: 1, SAFI: 1}
		prefix := ipv4Prefix(24, 10, 0, 0)
		attrs := makeAttrBytes([4]byte{192, 168, 1, 1})
		r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
		r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

		require.NotPanics(t, func() {
			entry, ok := r.checkBestPathChange(fam, prefix, false, nil)
			assert.False(t, ok, "overflow returns (zero, false)")
			assert.Equal(t, bestChangeEntry{}, entry)
		})
	})

	// Part 3: saturation logs an slog.Error exactly once per table. Repeated
	// saturated calls on the same table are silent -- this prevents the
	// per-UPDATE log flood a deployed-at-cap daemon would otherwise produce.
	t.Run("overflow-logs-once-per-table", func(t *testing.T) {
		var logBuf bytes.Buffer
		prior := logger()
		h := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelError})
		SetLogger(slog.New(h))
		defer SetLogger(prior)

		ir := newBestPrevInterner()
		for i := range internerCap {
			if _, ok := ir.internMetric(uint32(i)); !ok {
				t.Fatalf("fill %d: metric interner unexpectedly rejected within cap", i)
			}
		}

		// First overflow on metrics: MUST emit one log line.
		_, ok := ir.internMetric(0xFFFFFFFF)
		require.False(t, ok)
		assert.Equal(t, 1, strings.Count(logBuf.String(), "best-path interner saturated"),
			"first saturation logs once")
		assert.Contains(t, logBuf.String(), "table=metrics")

		// Second overflow on the same table: MUST NOT emit another log line.
		_, ok = ir.internMetric(0xFFFFFFFE)
		require.False(t, ok)
		assert.Equal(t, 1, strings.Count(logBuf.String(), "best-path interner saturated"),
			"repeat saturation on same table is silent")

		// Overflow on a different table: MUST emit its own log line.
		for i := range internerCap {
			if _, ok := ir.internPeer(syntheticPeerKey(i)); !ok {
				t.Fatalf("fill %d: peer interner unexpectedly rejected within cap", i)
			}
		}
		_, ok = ir.internPeer("203.0.113.99")
		require.False(t, ok)
		assert.Equal(t, 2, strings.Count(logBuf.String(), "best-path interner saturated"),
			"each table logs its own first saturation")
		assert.Contains(t, logBuf.String(), "table=peers")
	})
}

// syntheticPeerKey builds a guaranteed-unique string for the overflow test
// without relying on net-parseable IP syntax -- the interner stores strings
// verbatim, so any distinct input drives a new index.
func syntheticPeerKey(i int) string {
	const hex = "0123456789abcdef"
	return string([]byte{
		hex[(i>>12)&0xF], hex[(i>>8)&0xF], hex[(i>>4)&0xF], hex[i&0xF],
	})
}

// VALIDATES: AC-5 -- resolve rebuilds BestChangeEntry payload fields from the
// packed record plus interner reverse tables.
// PREVENTS: Emission path regressions producing wrong priority / protocol-type.
func TestBestPathResolve(t *testing.T) {
	ir := newBestPrevInterner()
	peerIdx, _ := ir.internPeer("192.0.2.1")
	nhIdx, _ := ir.internNextHop(netip.MustParseAddr("10.0.0.1"))
	metricIdx, _ := ir.internMetric(500)

	ebgpRec := packBestPath(metricIdx, peerIdx, nhIdx, flagEBGP)
	ebgpEntry := ebgpRec.resolve(ir, ribevents.BestChangeAdd, "10.0.0.0/24")
	assert.Equal(t, ribevents.BestChangeAdd, ebgpEntry.Action)
	assert.Equal(t, "10.0.0.0/24", ebgpEntry.Prefix)
	assert.Equal(t, "10.0.0.1", ebgpEntry.NextHop)
	assert.Equal(t, 20, ebgpEntry.Priority, "eBGP records resolve to priority 20")
	assert.Equal(t, uint32(500), ebgpEntry.Metric)
	assert.Equal(t, "ebgp", ebgpEntry.ProtocolType)

	ibgpRec := packBestPath(metricIdx, peerIdx, nhIdx, 0)
	ibgpEntry := ibgpRec.resolve(ir, ribevents.BestChangeUpdate, "10.0.0.0/24")
	assert.Equal(t, ribevents.BestChangeUpdate, ibgpEntry.Action)
	assert.Equal(t, 200, ibgpEntry.Priority, "iBGP records resolve to priority 200")
	assert.Equal(t, "ibgp", ibgpEntry.ProtocolType)

	// Zero next-hop round-trips to empty string via nextHopString.
	zeroNHIdx, _ := ir.internNextHop(netip.Addr{})
	zeroRec := packBestPath(metricIdx, peerIdx, zeroNHIdx, 0)
	zeroEntry := zeroRec.resolve(ir, ribevents.BestChangeWithdraw, "")
	assert.Empty(t, zeroEntry.NextHop)
}

// TestLocRIBMirror validates that BGP best-path changes are mirrored into
// the shared cross-protocol Loc-RIB (Phase 3b of plan/design-rib-unified.md).
//
// VALIDATES: SetLocRIB + checkBestPathChange publish the BGP best as a
// locrib.Path; withdrawal removes it.
// PREVENTS: Silent drift between BGP's internal best-path state and the
// unified Loc-RIB that non-BGP consumers observe.
func TestLocRIBMirror(t *testing.T) {
	r := newTestRIBManager(t)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	_, ok := r.checkBestPathChange(fam, prefix, false, nil)
	require.True(t, ok)

	best, found := loc.Best(fam, netip.MustParsePrefix("10.0.0.0/24"))
	require.True(t, found, "Loc-RIB must carry the BGP best after checkBestPathChange")
	assert.Equal(t, uint8(20), best.AdminDistance, "eBGP AdminDistance")
	assert.Equal(t, "192.168.1.1", best.NextHop.String())

	// Withdraw: remove the route from the adj-rib-in, then re-run bestpath.
	r.ribInPool[peerAddr].Remove(fam, prefix)
	_, ok = r.checkBestPathChange(fam, prefix, false, nil)
	require.True(t, ok)

	_, found = loc.Best(fam, netip.MustParsePrefix("10.0.0.0/24"))
	assert.False(t, found, "Loc-RIB must drop the prefix after BGP withdraws its best")
}

// TestLocRIBMirrorPropagatesForwardHandle validates that when rib passes a
// non-nil ForwardHandle to checkBestPathChange, the resulting locrib Change
// dispatched to subscribers carries that handle.
//
// VALIDATES: design-rib-rs-fastpath.md -- producer side is wired.
// PREVENTS: a subscriber seeing nil Forward when rib supplied a handle,
// which would regress the feature back to "library only" state.
func TestLocRIBMirrorPropagatesForwardHandle(t *testing.T) {
	r := newTestRIBManager(t)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)

	var seen []locrib.Change
	loc.OnChange(func(c locrib.Change) { seen = append(seen, c) })

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	// Simulate the wire buffer as handleReceivedStructured would see it.
	wire := []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02, 0x03}
	forward := newForwardHandle(wire)
	require.NotNil(t, forward, "non-empty wire bytes must produce a handle")

	_, ok := r.checkBestPathChange(fam, prefix, false, forward)
	require.True(t, ok)

	require.Len(t, seen, 1, "BGP best-change must produce exactly one locrib Change")
	assert.Equal(t, locrib.ChangeAdd, seen[0].Kind)
	assert.Same(t, forward, seen[0].Forward, "Change must carry the handle rib passed in")

	// End-to-end: subscriber AddRef inside the handler, then reads the
	// retained bytes via the ForwardBytes optional interface. Proves
	// the locrib interface + rib implementation compose.
	seen[0].Forward.AddRef()
	reader, ok := seen[0].Forward.(locrib.ForwardBytes)
	require.True(t, ok, "rib handle must satisfy locrib.ForwardBytes")
	assert.Equal(t, wire, reader.Bytes(), "subscriber reads the retained wire payload")
	seen[0].Forward.Release()
}

// TestForwardHandleEmptyBytes documents that an empty wire buffer
// yields a nil ForwardHandle; rib never dispatches a non-nil handle
// pointing at zero bytes.
//
// VALIDATES: newForwardHandle "no wire payload" short-circuit.
// PREVENTS: a subscriber AddRef-ing on a handle whose materialized
// copy would be empty.
func TestForwardHandleEmptyBytes(t *testing.T) {
	assert.Nil(t, newForwardHandle(nil))
	assert.Nil(t, newForwardHandle([]byte{}))
	assert.NotNil(t, newForwardHandle([]byte{1}))
}

// TestForwardHandleRefcount validates the AddRef / Release contract:
// N AddRefs + N Releases drive the refcount back to zero, and Release
// beyond AddRef is permitted (goes negative, observable).
//
// VALIDATES: ribForwardHandle refcount semantics.
// PREVENTS: silent drift between the refcount value and subscriber
// lifetime; catches a future change that moves AddRef logic around.
func TestForwardHandleRefcount(t *testing.T) {
	h, ok := newForwardHandle([]byte{0xaa, 0xbb}).(*ribForwardHandle)
	require.True(t, ok, "newForwardHandle must return *ribForwardHandle")
	assert.Zero(t, h.refs.Load(), "fresh handle starts at zero refs")

	h.AddRef()
	h.AddRef()
	h.AddRef()
	assert.Equal(t, int32(3), h.refs.Load())

	h.Release()
	h.Release()
	h.Release()
	assert.Zero(t, h.refs.Load(), "N AddRefs + N Releases returns to zero")

	// Unbalanced Release is not defended against -- documenting current
	// behavior so a future guard change updates this assertion.
	h.Release()
	assert.Equal(t, int32(-1), h.refs.Load(), "over-Release goes negative silently")
}

// TestForwardHandleBytesLazyCopy validates that the first AddRef copies
// source into an owned buffer, Bytes returns that copy, and the source
// slice is not aliased into buf.
//
// VALIDATES: ForwardBytes contract -- subscribers can read the wire
// bytes after AddRef without relying on the unsafe source slice.
// PREVENTS: regressions that silently alias source (reused after the
// producing handler returns) instead of copying.
func TestForwardHandleBytesLazyCopy(t *testing.T) {
	source := []byte{0x01, 0x02, 0x03, 0x04}
	h, ok := newForwardHandle(source).(*ribForwardHandle)
	require.True(t, ok)

	// Before AddRef, Bytes returns nil.
	assert.Nil(t, h.Bytes(), "Bytes before AddRef must be nil")

	h.AddRef()
	got := h.Bytes()
	require.NotNil(t, got)
	assert.Equal(t, source, got, "Bytes returns the copied payload")

	// Mutate source to prove buf is not aliased.
	source[0] = 0xff
	assert.Equal(t, byte(0x01), h.Bytes()[0], "buf must not alias source")

	// Second AddRef does NOT re-copy (sync.Once).
	h.AddRef()
	assert.Same(t, &got[0], &h.Bytes()[0], "subsequent AddRef reuses buf")

	h.Release()
	h.Release()
	// Bytes after full Release still returns the copy (GC decides
	// lifetime; Release does not zero buf).
	assert.Equal(t, byte(0x01), h.Bytes()[0])
}

// TestObserveForwardHandlesLogsOnNonNil captures the observer's log
// output via a buffer-backed slog handler and verifies the observer
// emits "forward-handle observed" only when Change.Forward is non-nil.
// Asserts on the log message itself -- the observer's only externally
// visible effect.
//
// VALIDATES: observeForwardHandles actually logs the expected line.
// PREVENTS: regression where the observer silently no-ops (would have
// passed the previous counter-only assertion that did not look at log
// output).
func TestObserveForwardHandlesLogsOnNonNil(t *testing.T) {
	var buf bytes.Buffer
	prev := logger()
	SetLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer SetLogger(prev)

	loc := locrib.NewRIB()
	unsub := observeForwardHandles(loc)
	defer unsub()

	p := locrib.Path{
		Source:        bgpProtocolID,
		Instance:      1,
		NextHop:       netip.MustParseAddr("192.0.2.1"),
		AdminDistance: 20,
	}

	// Insert WITHOUT a forward handle (non-BGP producer case): no log.
	loc.Insert(famUnicast(), netip.MustParsePrefix("10.0.0.0/24"), p)
	assert.NotContains(t, buf.String(), "forward-handle observed",
		"observer must stay silent when Change.Forward is nil")

	// InsertForward WITH a handle (BGP producer case): single log line.
	p2 := p
	p2.Instance = 2
	loc.InsertForward(famUnicast(), netip.MustParsePrefix("10.1.0.0/24"), p2,
		newForwardHandle([]byte{0x01, 0x02}))
	out := buf.String()
	assert.Contains(t, out, "forward-handle observed")
	assert.Contains(t, out, "prefix=10.1.0.0/24")
	assert.Contains(t, out, "kind=add")
}

// TestObserveForwardHandlesQuietWhenDebugOff validates the Enabled()
// gate: when the package logger filters out debug, the observer does
// nothing, and none of the Change fields are string-formatted.
//
// VALIDATES: debug-off path has zero formatting cost.
// PREVENTS: accidental removal of the Enabled() pre-check.
func TestObserveForwardHandlesQuietWhenDebugOff(t *testing.T) {
	var buf bytes.Buffer
	prev := logger()
	// Level higher than debug -- observer must skip entirely.
	SetLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	defer SetLogger(prev)

	loc := locrib.NewRIB()
	unsub := observeForwardHandles(loc)
	defer unsub()

	loc.InsertForward(famUnicast(), netip.MustParsePrefix("10.2.0.0/24"),
		locrib.Path{Source: bgpProtocolID, Instance: 1, NextHop: netip.MustParseAddr("192.0.2.1"), AdminDistance: 20},
		newForwardHandle([]byte{0xaa}))
	assert.Empty(t, buf.String(), "observer must not emit at info level")
}

// famUnicast returns ipv4 unicast for the observer test without
// pulling in a full family-registry setup.
func famUnicast() family.Family { return family.Family{AFI: 1, SAFI: 1} }

// TestForwardHandleConcurrentAddRef validates that parallel AddRefs
// all see the same materialized buf and the final refcount matches
// the number of AddRefs.
//
// VALIDATES: sync.Once copy is visible to every goroutine that
// AddRef-ed.
// PREVENTS: data race on buf under concurrent AddRef.
func TestForwardHandleConcurrentAddRef(t *testing.T) {
	source := make([]byte, 256)
	for i := range source {
		source[i] = byte(i)
	}
	h, ok := newForwardHandle(source).(*ribForwardHandle)
	require.True(t, ok)

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			h.AddRef()
			got := h.Bytes()
			assert.Equal(t, source, got)
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(goroutines), h.refs.Load())
}
