package redistribute

import (
	"net/netip"
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/routeaction"

	configredist "codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/ribevents"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBGPSourcesRegisteredAtInit(t *testing.T) {
	for _, name := range []string{"bgp", "ibgp", "ebgp"} {
		src, ok := configredist.LookupSource(name)
		require.True(t, ok, "source %q must be registered before config parsing", name)
		assert.Equal(t, "bgp", src.Protocol)
	}
}

type testEvent struct {
	namespace string
	eventType string
	payload   any
}

type testBus struct {
	mu       sync.Mutex
	events   []testEvent
	handlers map[string][]func(any)
}

func newTestBus() *testBus {
	return &testBus{handlers: make(map[string][]func(any))}
}

func (b *testBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	b.events = append(b.events, testEvent{namespace: namespace, eventType: eventType, payload: payload})
	handlers := append([]func(any){}, b.handlers[namespace+"/"+eventType]...)
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (b *testBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	b.mu.Lock()
	key := namespace + "/" + eventType
	b.handlers[key] = append(b.handlers[key], handler)
	idx := len(b.handlers[key]) - 1
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if idx >= 0 && idx < len(b.handlers[key]) {
			b.handlers[key][idx] = nil
		}
	}
}

func TestBGPProducerBridgeEmitsRouteChange(t *testing.T) {
	bus := newTestBus()
	seen := make(chan *redistevents.RouteChangeBatch, 1)
	unsub := RouteChange.Subscribe(bus, func(b *redistevents.RouteChangeBatch) {
		cp := *b
		cp.Entries = append([]redistevents.RouteChangeEntry(nil), b.Entries...)
		seen <- &cp
	})
	defer unsub()

	EmitBestChange(bus, &ribevents.BestChangeBatch{
		Protocol: "bgp",
		Family:   family.IPv6Unicast,
		Changes: []ribevents.BestChangeEntry{{
			Action:   routeaction.Add,
			Prefix:   netip.MustParsePrefix("2001:db8:5e5::/48"),
			NextHop:  netip.MustParseAddr("fd00:1e::4"),
			Metric:   4242,
			OriginAS: 64512,
		}},
	})

	got := <-seen
	assert.Equal(t, ProtocolID, got.Protocol)
	assert.Equal(t, uint16(family.AFIIPv6), got.AFI)
	assert.Equal(t, uint8(family.SAFIUnicast), got.SAFI)
	require.Len(t, got.Entries, 1)
	assert.Equal(t, redistevents.ActionAdd, got.Entries[0].Action)
	assert.Equal(t, netip.MustParsePrefix("2001:db8:5e5::/48"), got.Entries[0].Prefix)
	assert.Equal(t, netip.MustParseAddr("fd00:1e::4"), got.Entries[0].NextHop)
	// AC-3 / AC-4: the bridge no longer drops Metric or OriginAS.
	assert.Equal(t, uint32(4242), got.Entries[0].Metric, "bridge must carry the source Metric")
	assert.Equal(t, uint32(64512), got.Entries[0].OriginAS, "bridge must carry the source OriginAS")
}

func TestBGPProducerBridgeWithdraw(t *testing.T) {
	bus := newTestBus()
	seen := make(chan *redistevents.RouteChangeBatch, 1)
	unsub := RouteChange.Subscribe(bus, func(b *redistevents.RouteChangeBatch) {
		cp := *b
		cp.Entries = append([]redistevents.RouteChangeEntry(nil), b.Entries...)
		seen <- &cp
	})
	defer unsub()

	EmitBestChange(bus, &ribevents.BestChangeBatch{
		Protocol: "bgp",
		Family:   family.IPv4Unicast,
		Changes: []ribevents.BestChangeEntry{{
			Action: routeaction.Withdraw,
			Prefix: netip.MustParsePrefix("192.0.2.0/24"),
		}},
	})

	got := <-seen
	require.Len(t, got.Entries, 1)
	assert.Equal(t, redistevents.ActionRemove, got.Entries[0].Action)
	assert.Equal(t, netip.MustParsePrefix("192.0.2.0/24"), got.Entries[0].Prefix)
}

// TestBGPProducerBridgeMapsAllActions verifies AC-1: a batch carrying add,
// update and withdraw maps to add, add, remove with every entry emitted -- no
// entry silently dropped. It is a regression guard for the lossless bridge.
func TestBGPProducerBridgeMapsAllActions(t *testing.T) {
	bus := newTestBus()
	seen := make(chan *redistevents.RouteChangeBatch, 1)
	unsub := RouteChange.Subscribe(bus, func(b *redistevents.RouteChangeBatch) {
		cp := *b
		cp.Entries = append([]redistevents.RouteChangeEntry(nil), b.Entries...)
		seen <- &cp
	})
	defer unsub()

	EmitBestChange(bus, &ribevents.BestChangeBatch{
		Protocol: "bgp",
		Family:   family.IPv4Unicast,
		Changes: []ribevents.BestChangeEntry{
			{Action: routeaction.Add, Prefix: netip.MustParsePrefix("192.0.2.0/24")},
			{Action: routeaction.Update, Prefix: netip.MustParsePrefix("198.51.100.0/24")},
			{Action: routeaction.Withdraw, Prefix: netip.MustParsePrefix("203.0.113.0/24")},
		},
	})

	got := <-seen
	require.Len(t, got.Entries, 3, "all three actions must survive: none silently dropped")
	assert.Equal(t, redistevents.ActionAdd, got.Entries[0].Action, "add -> ActionAdd")
	assert.Equal(t, redistevents.ActionAdd, got.Entries[1].Action, "update -> ActionAdd")
	assert.Equal(t, redistevents.ActionRemove, got.Entries[2].Action, "withdraw -> ActionRemove")
}

// TestBGPProducerBridgeUnknownActionLogged verifies AC-2: an entry whose action
// is outside add/update/withdraw is skipped AND counted (no silent discard),
// while valid entries in the same batch still flow through.
func TestBGPProducerBridgeUnknownActionLogged(t *testing.T) {
	bus := newTestBus()
	seen := make(chan *redistevents.RouteChangeBatch, 1)
	unsub := RouteChange.Subscribe(bus, func(b *redistevents.RouteChangeBatch) {
		cp := *b
		cp.Entries = append([]redistevents.RouteChangeEntry(nil), b.Entries...)
		seen <- &cp
	})
	defer unsub()

	before := unknownActionSkips.Load()

	EmitBestChange(bus, &ribevents.BestChangeBatch{
		Protocol: "bgp",
		Family:   family.IPv4Unicast,
		Changes: []ribevents.BestChangeEntry{
			{Action: routeaction.Add, Prefix: netip.MustParsePrefix("192.0.2.0/24")},
			// A hypothetical future enumerant the bridge does not map (AC-2).
			{Action: routeaction.Action(99), Prefix: netip.MustParsePrefix("198.51.100.0/24")},
		},
	})

	got := <-seen
	require.Len(t, got.Entries, 1, "unknown-action entry must be skipped, valid entry kept")
	assert.Equal(t, netip.MustParsePrefix("192.0.2.0/24"), got.Entries[0].Prefix)
	assert.Equal(t, uint64(1), unknownActionSkips.Load()-before, "the skipped entry must be counted, not silently dropped")
}
