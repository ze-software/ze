package sysrib

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// testBus is a minimal Bus implementation for testing.
type testBus struct {
	mu     sync.Mutex
	events []ze.Event
}

func newTestBus() *testBus {
	return &testBus{}
}

func (b *testBus) CreateTopic(_ string) (ze.Topic, error) { return ze.Topic{}, nil }

func (b *testBus) Publish(topic string, payload []byte, metadata map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, ze.Event{Topic: topic, Payload: payload, Metadata: metadata})
}

func (b *testBus) Subscribe(_ string, _ map[string]string, _ ze.Consumer) (ze.Subscription, error) {
	return ze.Subscription{}, nil
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

func makeEvent(protocol, family string, changes []incomingChange) ze.Event {
	batch := incomingBatch{Changes: changes}
	payload, _ := json.Marshal(batch)
	return ze.Event{
		Topic:   "rib/best-change/" + protocol,
		Payload: payload,
		Metadata: map[string]string{
			"protocol": protocol,
			"family":   family,
		},
	}
}

// VALIDATES: AC-4 -- System RIB receives rib/best-change/bgp (eBGP priority 20),
// installs as system best if no lower-priority route exists.
// PREVENTS: System RIB not selecting correct winner.
func TestSysRIBSelectByPriority(t *testing.T) {
	bus := newTestBus()
	setBus(bus)
	s := newSysRIB()

	// eBGP route arrives with priority 20.
	event := makeEvent("bgp", "ipv4/unicast", []incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Priority: 20},
	})
	changes := s.processEvent(event)

	require.Len(t, changes, 1)
	assert.Equal(t, "add", changes[0].Action)
	assert.Equal(t, "10.0.0.0/24", changes[0].Prefix)
	assert.Equal(t, "192.168.1.1", changes[0].NextHop)
	assert.Equal(t, "bgp", changes[0].Protocol)
}

// VALIDATES: AC-5 -- System RIB has static (priority 10) and eBGP (priority 20)
// for same prefix. Static wins.
// PREVENTS: Higher-priority (lower number) route not winning.
func TestSysRIBStaticWinsOverBGP(t *testing.T) {
	bus := newTestBus()
	setBus(bus)
	s := newSysRIB()

	// BGP route first.
	bgpEvent := makeEvent("bgp", "ipv4/unicast", []incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Priority: 20},
	})
	s.processEvent(bgpEvent)

	// Static route arrives with lower priority (wins).
	staticEvent := makeEvent("static", "ipv4/unicast", []incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "10.0.0.1", Priority: 10},
	})
	changes := s.processEvent(staticEvent)

	require.Len(t, changes, 1)
	assert.Equal(t, "update", changes[0].Action)
	assert.Equal(t, "10.0.0.1", changes[0].NextHop, "static next-hop should win")
	assert.Equal(t, "static", changes[0].Protocol)
}

// VALIDATES: AC-6 -- Static route withdrawn, BGP route still exists.
// BGP becomes system best with action "update".
// PREVENTS: Fallback to remaining protocol not working.
func TestSysRIBFallback(t *testing.T) {
	bus := newTestBus()
	setBus(bus)
	s := newSysRIB()

	// Install both routes.
	s.processEvent(makeEvent("bgp", "ipv4/unicast", []incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Priority: 20},
	}))
	s.processEvent(makeEvent("static", "ipv4/unicast", []incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "10.0.0.1", Priority: 10},
	}))

	// Withdraw static.
	changes := s.processEvent(makeEvent("static", "ipv4/unicast", []incomingChange{
		{Action: "withdraw", Prefix: "10.0.0.0/24"},
	}))

	require.Len(t, changes, 1)
	assert.Equal(t, "update", changes[0].Action)
	assert.Equal(t, "192.168.1.1", changes[0].NextHop, "BGP should become system best")
	assert.Equal(t, "bgp", changes[0].Protocol)
}

// VALIDATES: AC-7 -- All routes withdrawn for prefix. System RIB publishes
// sysrib/best-change with action "withdraw".
// PREVENTS: Stale entries remaining in system RIB.
func TestSysRIBWithdrawAll(t *testing.T) {
	bus := newTestBus()
	setBus(bus)
	s := newSysRIB()

	// Install and then withdraw using IPv6 family.
	s.processEvent(makeEvent("bgp", "ipv6/unicast", []incomingChange{
		{Action: "add", Prefix: "2001:db8::/32", NextHop: "fe80::1", Priority: 20},
	}))

	changes := s.processEvent(makeEvent("bgp", "ipv6/unicast", []incomingChange{
		{Action: "withdraw", Prefix: "2001:db8::/32"},
	}))

	require.Len(t, changes, 1)
	assert.Equal(t, "withdraw", changes[0].Action)
	assert.Equal(t, "2001:db8::/32", changes[0].Prefix)
}

// VALIDATES: AC-4 -- System RIB publishes sysrib/best-change on system best change.
// PREVENTS: Bus events not being published.
func TestSysRIBPublishChange(t *testing.T) {
	bus := newTestBus()
	setBus(bus)
	s := newSysRIB()

	event := makeEvent("bgp", "ipv4/unicast", []incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Priority: 20},
	})
	changes := s.processEvent(event)
	require.Len(t, changes, 1)

	publishChanges(changes, "ipv4/unicast")

	evt := bus.lastEvent()
	require.NotNil(t, evt)
	assert.Equal(t, sysribTopic, evt.Topic)
	assert.Equal(t, "ipv4/unicast", evt.Metadata["family"])

	var batch outgoingBatch
	require.NoError(t, json.Unmarshal(evt.Payload, &batch))
	require.Len(t, batch.Changes, 1)
	assert.Equal(t, "add", batch.Changes[0].Action)
	assert.Equal(t, "bgp", batch.Changes[0].Protocol)
}

// VALIDATES: AC-4 -- No change event when same route is re-announced.
// PREVENTS: Spurious system RIB events.
func TestSysRIBNoChangeOnSameRoute(t *testing.T) {
	s := newSysRIB()

	event := makeEvent("bgp", "ipv4/unicast", []incomingChange{
		{Action: "add", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Priority: 20},
	})
	changes1 := s.processEvent(event)
	require.Len(t, changes1, 1)

	// Same route again (update with identical data).
	changes2 := s.processEvent(makeEvent("bgp", "ipv4/unicast", []incomingChange{
		{Action: "update", Prefix: "10.0.0.0/24", NextHop: "192.168.1.1", Priority: 20},
	}))
	assert.Empty(t, changes2, "no change when same route is re-announced")
}
