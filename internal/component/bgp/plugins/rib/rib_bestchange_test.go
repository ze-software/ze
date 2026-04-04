package rib

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// testBus is a minimal Bus implementation for testing.
type testBus struct {
	mu       sync.Mutex
	events   []ze.Event
	topics   map[string]struct{}
	consumer ze.Consumer
}

func newTestBus() *testBus {
	return &testBus{
		topics: make(map[string]struct{}),
	}
}

func (b *testBus) CreateTopic(name string) (ze.Topic, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.topics[name] = struct{}{}
	return ze.Topic{Name: name}, nil
}

func (b *testBus) Publish(topic string, payload []byte, metadata map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, ze.Event{
		Topic:    topic,
		Payload:  payload,
		Metadata: metadata,
	})
}

func (b *testBus) Subscribe(_ string, _ map[string]string, c ze.Consumer) (ze.Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consumer = c
	return ze.Subscription{ID: 1}, nil
}

func (b *testBus) Unsubscribe(_ ze.Subscription) {}

func (b *testBus) lastEvent() *ze.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.events) == 0 {
		return nil
	}
	return &b.events[len(b.events)-1]
}

func (b *testBus) eventCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

// newTestRIBManagerWithBus creates a RIBManager with test Bus wired in.
func newTestRIBManagerWithBus(bus *testBus) *RIBManager {
	SetBus(bus)
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

// VALIDATES: AC-1 -- BGP UPDATE makes prefix best path change, RIB publishes
// rib/best-change/bgp with action "add" and correct next-hop.
// PREVENTS: Best-path changes going undetected.
func TestRIBBestChangePublish(t *testing.T) {
	bus := newTestBus()
	r := newTestRIBManagerWithBus(bus)

	// Set up peer metadata for eBGP.
	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	// Insert a route from peer.
	family := nlri.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(family, attrs, prefix)

	// Check best-path change under lock.
	r.mu.Lock()
	change := r.checkBestPathChange(family, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change, "should detect new best path")
	assert.Equal(t, bestChangeAdd, change.Action)
	assert.Equal(t, "10.0.0.0/24", change.Prefix)
	assert.Equal(t, "192.168.1.1", change.NextHop)
	assert.Equal(t, 20, change.Priority, "eBGP should have priority 20")

	// Publish and verify Bus event.
	publishBestChanges([]bestChangeEntry{*change}, family.String())

	evt := bus.lastEvent()
	require.NotNil(t, evt)
	assert.Equal(t, bestChangeTopic, evt.Topic)
	assert.Equal(t, "bgp", evt.Metadata["protocol"])
	assert.Equal(t, "ipv4/unicast", evt.Metadata["family"])

	var batch bestChangeBatch
	require.NoError(t, json.Unmarshal(evt.Payload, &batch))
	require.Len(t, batch.Changes, 1)
	assert.Equal(t, "add", batch.Changes[0].Action)
	assert.Equal(t, "10.0.0.0/24", batch.Changes[0].Prefix)
	assert.Equal(t, "192.168.1.1", batch.Changes[0].NextHop)
}

// VALIDATES: AC-3 -- BGP UPDATE does not change best path, no event published.
// PREVENTS: Spurious Bus events when best path is unchanged.
func TestRIBBestChangeNoPublishSameBest(t *testing.T) {
	bus := newTestBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	family := nlri.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(family, attrs, prefix)

	// First check: detects new best.
	r.mu.Lock()
	change1 := r.checkBestPathChange(family, prefix, false)
	r.mu.Unlock()
	require.NotNil(t, change1)

	// Re-insert same route (implicit withdraw + re-add with same attrs).
	r.ribInPool[peerAddr].Insert(family, attrs, prefix)

	// Second check: same best, no change.
	r.mu.Lock()
	change2 := r.checkBestPathChange(family, prefix, false)
	r.mu.Unlock()
	assert.Nil(t, change2, "no change when best path is unchanged")
}

// VALIDATES: AC-2 -- BGP withdraws last route for prefix, RIB publishes
// rib/best-change/bgp with action "withdraw".
// PREVENTS: Withdraw events not being published.
func TestRIBBestChangeWithdraw(t *testing.T) {
	bus := newTestBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	family := nlri.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(family, attrs, prefix)

	// Establish best path.
	r.mu.Lock()
	r.checkBestPathChange(family, prefix, false)
	r.mu.Unlock()

	// Withdraw the route.
	r.ribInPool[peerAddr].Remove(family, prefix)

	r.mu.Lock()
	change := r.checkBestPathChange(family, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change, "should detect withdraw")
	assert.Equal(t, bestChangeWithdraw, change.Action)
	assert.Equal(t, "10.0.0.0/24", change.Prefix)
	assert.Empty(t, change.NextHop, "withdraw should have no next-hop")
}

// VALIDATES: AC-11 -- Peer goes down, all its routes withdrawn. RIB publishes
// single batch event with withdraws for all affected prefixes.
// PREVENTS: Peer-down producing individual events per prefix.
func TestRIBBestChangeBatchPeerDown(t *testing.T) {
	bus := newTestBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	family := nlri.Family{AFI: 1, SAFI: 1}
	prefixes := [][]byte{
		ipv4Prefix(24, 10, 0, 0),
		ipv4Prefix(16, 172, 16),
		ipv4Prefix(8, 192),
	}
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	for _, p := range prefixes {
		r.ribInPool[peerAddr].Insert(family, attrs, p)
	}

	// Establish best paths for all prefixes.
	r.mu.Lock()
	for _, p := range prefixes {
		r.checkBestPathChange(family, p, false)
	}
	r.mu.Unlock()

	// Simulate peer down: withdraw all routes.
	for _, p := range prefixes {
		r.ribInPool[peerAddr].Remove(family, p)
	}

	// Collect all changes in one batch (under lock).
	r.mu.Lock()
	var changes []bestChangeEntry
	for _, p := range prefixes {
		if change := r.checkBestPathChange(family, p, false); change != nil {
			changes = append(changes, *change)
		}
	}
	r.mu.Unlock()

	// Publish as single batch.
	require.Len(t, changes, 3, "should have 3 withdraw changes")
	publishBestChanges(changes, family.String())

	// Verify single Bus event with 3 withdrawals.
	assert.Equal(t, 1, bus.eventCount(), "should be a single batch event")
	evt := bus.lastEvent()
	require.NotNil(t, evt)

	var batch bestChangeBatch
	require.NoError(t, json.Unmarshal(evt.Payload, &batch))
	assert.Len(t, batch.Changes, 3)
	for _, c := range batch.Changes {
		assert.Equal(t, bestChangeWithdraw, c.Action)
	}
}

// VALIDATES: AC-1 (eBGP priority) -- eBGP routes published with priority 20.
// PREVENTS: Wrong admin distance for eBGP routes.
func TestRIBBestChangeEBGPPriority(t *testing.T) {
	bus := newTestBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	family := nlri.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(family, attrs, prefix)

	r.mu.Lock()
	change := r.checkBestPathChange(family, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change)
	assert.Equal(t, 20, change.Priority, "eBGP admin distance should be 20")
}

// VALIDATES: AC-1 (iBGP priority) -- iBGP routes published with priority 200.
// PREVENTS: Wrong admin distance for iBGP routes.
func TestRIBBestChangeIBGPPriority(t *testing.T) {
	bus := newTestBus()
	r := newTestRIBManagerWithBus(bus)

	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65000, LocalASN: 65000} // same AS = iBGP

	family := nlri.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(family, attrs, prefix)

	r.mu.Lock()
	change := r.checkBestPathChange(family, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change)
	assert.Equal(t, 200, change.Priority, "iBGP admin distance should be 200")
}

// VALIDATES: AC-1 -- Best path changes when a better route arrives from another peer.
// PREVENTS: Best-path update events not being detected.
func TestRIBBestChangeUpdate(t *testing.T) {
	bus := newTestBus()
	r := newTestRIBManagerWithBus(bus)

	family := nlri.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)

	// Peer 1: iBGP route.
	peer1 := "192.0.2.1"
	r.peerMeta[peer1] = &PeerMeta{PeerASN: 65000, LocalASN: 65000}
	r.ribInPool[peer1] = storage.NewPeerRIB(peer1)
	r.ribInPool[peer1].Insert(family, makeAttrBytes([4]byte{10, 0, 0, 1}), prefix)

	r.mu.Lock()
	change1 := r.checkBestPathChange(family, prefix, false)
	r.mu.Unlock()
	require.NotNil(t, change1)
	assert.Equal(t, bestChangeAdd, change1.Action)

	// Peer 2: eBGP route (better -- eBGP preferred over iBGP).
	peer2 := "192.0.2.2"
	r.peerMeta[peer2] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}
	r.ribInPool[peer2] = storage.NewPeerRIB(peer2)
	r.ribInPool[peer2].Insert(family, makeAttrBytes([4]byte{10, 0, 0, 2}), prefix)

	r.mu.Lock()
	change2 := r.checkBestPathChange(family, prefix, false)
	r.mu.Unlock()

	require.NotNil(t, change2, "should detect best-path update")
	assert.Equal(t, bestChangeUpdate, change2.Action)
	assert.Equal(t, "10.0.0.2", change2.NextHop, "should switch to eBGP next-hop")
	assert.Equal(t, 20, change2.Priority, "eBGP priority")
}

// VALIDATES: AC-23 -- System RIB subscribes after BGP RIB has computed best paths.
// Protocol RIB replays current best-path table as batch event.
// PREVENTS: Late-starting subscribers missing the initial state.
func TestRIBReplayOnSubscribe(t *testing.T) {
	bus := newTestBus()
	r := newTestRIBManagerWithBus(bus)

	// Set up a peer with a route and establish best path.
	peerAddr := "192.0.2.1"
	r.peerMeta[peerAddr] = &PeerMeta{PeerASN: 65001, LocalASN: 65000}

	family := nlri.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	attrs := makeAttrBytes([4]byte{192, 168, 1, 1})

	r.ribInPool[peerAddr] = storage.NewPeerRIB(peerAddr)
	r.ribInPool[peerAddr].Insert(family, attrs, prefix)

	r.mu.Lock()
	r.checkBestPathChange(family, prefix, false)
	r.mu.Unlock()

	// Clear Bus events from the initial insert.
	bus.mu.Lock()
	bus.events = nil
	bus.mu.Unlock()

	// Now trigger replay (simulating a late subscriber requesting it).
	r.replayBestPaths()

	// Should have published a replay batch.
	evt := bus.lastEvent()
	require.NotNil(t, evt, "should publish replay batch")
	assert.Equal(t, bestChangeTopic, evt.Topic)
	assert.Equal(t, "true", evt.Metadata["replay"])
	assert.Equal(t, "bgp", evt.Metadata["protocol"])

	var batch bestChangeBatch
	require.NoError(t, json.Unmarshal(evt.Payload, &batch))
	require.Len(t, batch.Changes, 1)
	assert.Equal(t, "add", batch.Changes[0].Action)
	assert.Equal(t, "192.168.1.1", batch.Changes[0].NextHop)
}
