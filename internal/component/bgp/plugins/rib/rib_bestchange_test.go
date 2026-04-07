package rib

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// testEvent records one event emitted on the in-memory test EventBus.
type testEvent struct {
	Namespace string
	EventType string
	Payload   string
}

// testEventBus is a minimal ze.EventBus implementation for unit tests.
// It records every Emit so test assertions can inspect the namespace,
// event-type, and payload. Subscribe stores handlers per-key and
// dispatches them synchronously on Emit.
type testEventBus struct {
	mu       sync.Mutex
	events   []testEvent
	handlers map[string][]func(string)
}

func newTestEventBus() *testEventBus {
	return &testEventBus{
		handlers: make(map[string][]func(string)),
	}
}

func (b *testEventBus) Emit(namespace, eventType, payload string) (int, error) {
	b.mu.Lock()
	b.events = append(b.events, testEvent{Namespace: namespace, EventType: eventType, Payload: payload})
	hs := append([]func(string){}, b.handlers[namespace+"/"+eventType]...)
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return 0, nil
}

func (b *testEventBus) Subscribe(namespace, eventType string, handler func(string)) func() {
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
	return &RIBManager{
		ribInPool:     make(map[string]*storage.PeerRIB),
		ribOut:        make(map[string]map[string]map[string]*Route),
		peerUp:        make(map[string]bool),
		peerMeta:      make(map[string]*PeerMeta),
		retainedPeers: make(map[string]bool),
		grState:       make(map[string]*peerGRState),
		bestPrev:      make(map[bestPathKey]*bestPathRecord),
	}
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
	r.mu.Lock()
	change := r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change, "should detect new best path")
	assert.Equal(t, bestChangeAdd, change.Action)
	assert.Equal(t, "10.0.0.0/24", change.Prefix)
	assert.Equal(t, "192.168.1.1", change.NextHop)
	assert.Equal(t, 20, change.Priority, "eBGP should have priority 20")

	// Publish and verify the EventBus event.
	publishBestChanges([]bestChangeEntry{*change}, fam.String())

	evt := bus.lastEvent()
	require.NotNil(t, evt)
	assert.Equal(t, plugin.NamespaceRIB, evt.Namespace)
	assert.Equal(t, plugin.EventBestChange, evt.EventType)

	var batch bestChangeBatch
	require.NoError(t, json.Unmarshal([]byte(evt.Payload), &batch))
	assert.Equal(t, "bgp", batch.Protocol)
	assert.Equal(t, "ipv4/unicast", batch.Family)
	require.Len(t, batch.Changes, 1)
	assert.Equal(t, "add", batch.Changes[0].Action)
	assert.Equal(t, "10.0.0.0/24", batch.Changes[0].Prefix)
	assert.Equal(t, "192.168.1.1", batch.Changes[0].NextHop)
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
	r.mu.Lock()
	change1 := r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()
	require.NotNil(t, change1)

	// Re-insert same route (implicit withdraw + re-add with same attrs).
	r.ribInPool[peerAddr].Insert(fam, attrs, prefix)

	// Second check: same best, no change.
	r.mu.Lock()
	change2 := r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()
	assert.Nil(t, change2, "no change when best path is unchanged")
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
	r.mu.Lock()
	r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()

	// Withdraw the route.
	r.ribInPool[peerAddr].Remove(fam, prefix)

	r.mu.Lock()
	change := r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change, "should detect withdraw")
	assert.Equal(t, bestChangeWithdraw, change.Action)
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
	r.mu.Lock()
	for _, p := range prefixes {
		r.checkBestPathChange(fam, p, false)
	}
	r.mu.Unlock()

	// Simulate peer down: withdraw all routes.
	for _, p := range prefixes {
		r.ribInPool[peerAddr].Remove(fam, p)
	}

	// Collect all changes in one batch (under lock).
	r.mu.Lock()
	var changes []bestChangeEntry
	for _, p := range prefixes {
		if change := r.checkBestPathChange(fam, p, false); change != nil {
			changes = append(changes, *change)
		}
	}
	r.mu.Unlock()

	// Publish as single batch.
	require.Len(t, changes, 3, "should have 3 withdraw changes")
	publishBestChanges(changes, fam.String())

	// Verify single event with 3 withdrawals.
	assert.Equal(t, 1, bus.eventCount(), "should be a single batch event")
	evt := bus.lastEvent()
	require.NotNil(t, evt)

	var batch bestChangeBatch
	require.NoError(t, json.Unmarshal([]byte(evt.Payload), &batch))
	assert.Len(t, batch.Changes, 3)
	for _, c := range batch.Changes {
		assert.Equal(t, bestChangeWithdraw, c.Action)
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

	r.mu.Lock()
	change := r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change)
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

	r.mu.Lock()
	change := r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change)
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

	r.mu.Lock()
	change := r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change)
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

	r.mu.Lock()
	change := r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change)
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

	r.mu.Lock()
	change1 := r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()
	require.NotNil(t, change1)
	assert.Equal(t, bestChangeAdd, change1.Action)

	// Peer 2: eBGP route (better -- eBGP preferred over iBGP).
	peer2 := "192.0.2.2"
	r.peerMeta[peer2] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}
	r.ribInPool[peer2] = storage.NewPeerRIB(peer2)
	r.ribInPool[peer2].Insert(fam, makeAttrBytes([4]byte{10, 0, 0, 2}), prefix)

	r.mu.Lock()
	change2 := r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change2, "should detect best-path update")
	assert.Equal(t, bestChangeUpdate, change2.Action)
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

	r.mu.Lock()
	r.checkBestPathChange(fam, prefix, false)
	r.mu.Unlock()

	// Clear emitted events from the initial insert.
	bus.mu.Lock()
	bus.events = nil
	bus.mu.Unlock()

	// Now trigger replay (simulating a late subscriber requesting it).
	r.replayBestPaths()

	// Should have published a replay batch.
	evt := bus.lastEvent()
	require.NotNil(t, evt, "should publish replay batch")
	assert.Equal(t, plugin.NamespaceRIB, evt.Namespace)
	assert.Equal(t, plugin.EventBestChange, evt.EventType)

	var batch bestChangeBatch
	require.NoError(t, json.Unmarshal([]byte(evt.Payload), &batch))
	assert.Equal(t, "bgp", batch.Protocol)
	assert.True(t, batch.Replay, "replay batch must have Replay=true")
	require.Len(t, batch.Changes, 1)
	assert.Equal(t, "add", batch.Changes[0].Action)
	assert.Equal(t, "192.168.1.1", batch.Changes[0].NextHop)
}
