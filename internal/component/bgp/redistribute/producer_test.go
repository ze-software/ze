package redistribute

import (
	"net/netip"
	"sync"
	"testing"

	ribevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/events"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	configredist "codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
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
			Action:  bgptypes.RouteActionAdd,
			Prefix:  netip.MustParsePrefix("2001:db8:5e5::/48"),
			NextHop: netip.MustParseAddr("fd00:1e::4"),
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
			Action: bgptypes.RouteActionWithdraw,
			Prefix: netip.MustParsePrefix("192.0.2.0/24"),
		}},
	})

	got := <-seen
	require.Len(t, got.Entries, 1)
	assert.Equal(t, redistevents.ActionRemove, got.Entries[0].Action)
	assert.Equal(t, netip.MustParsePrefix("192.0.2.0/24"), got.Entries[0].Prefix)
}
