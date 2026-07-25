// Design: docs/architecture/core-design.md -- kernel redistribute tests

package kernel

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/routewatch"
	kernelevents "github.com/ze-software/ze/internal/plugins/kernel/events"
	"github.com/ze-software/ze/pkg/ze"
)

type capturedBatch struct {
	Protocol redistevents.ProtocolID
	AFI      uint16
	SAFI     uint8
	Entries  []redistevents.RouteChangeEntry
}

type mockBus struct {
	mu      sync.Mutex
	subs    map[string][]func(any)
	batches []capturedBatch
}

func newMockBus() *mockBus {
	return &mockBus{subs: make(map[string][]func(any))}
}

func (m *mockBus) Emit(namespace, eventType string, payload any) (int, error) {
	m.mu.Lock()
	key := namespace + ":" + eventType
	handlers := append([]func(any){}, m.subs[key]...)
	m.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (m *mockBus) Subscribe(namespace, eventType string, handler func(payload any)) func() {
	m.mu.Lock()
	key := namespace + ":" + eventType
	m.subs[key] = append(m.subs[key], handler)
	m.mu.Unlock()
	return func() {}
}

func newCapturingBus() (*mockBus, func()) {
	bus := newMockBus()
	unsub := kernelevents.RouteChange.Subscribe(bus, func(b *redistevents.RouteChangeBatch) {
		batch := capturedBatch{
			Protocol: b.Protocol,
			AFI:      b.AFI,
			SAFI:     b.SAFI,
		}
		batch.Entries = append(batch.Entries, b.Entries...)
		bus.mu.Lock()
		bus.batches = append(bus.batches, batch)
		bus.mu.Unlock()
	})
	return bus, unsub
}

func TestKernelSourceRegistration(t *testing.T) {
	registerKernelSources()
	src, ok := redistribute.LookupSource("kernel")
	require.True(t, ok)
	assert.Equal(t, "kernel", src.Name)
	assert.Equal(t, "kernel", src.Protocol)
}

func TestKernelRouteAdd(t *testing.T) {
	bus, unsub := newCapturingBus()
	defer unsub()

	obs := newRouteObserver(bus)

	obs.handleRouteEvent(routewatch.RouteEvent{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:  netip.MustParseAddr("192.168.1.1"),
		Protocol: 16,
		Metric:   100,
		Action:   routewatch.ActionAdd,
	})

	require.Len(t, bus.batches, 1)
	b := bus.batches[0]
	assert.Equal(t, kernelevents.ProtocolID, b.Protocol)
	require.Len(t, b.Entries, 1)
	assert.Equal(t, redistevents.ActionAdd, b.Entries[0].Action)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), b.Entries[0].Prefix)
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), b.Entries[0].NextHop)
	assert.Equal(t, uint32(100), b.Entries[0].Metric)
}

func TestKernelRouteDelete(t *testing.T) {
	bus, unsub := newCapturingBus()
	defer unsub()

	obs := newRouteObserver(bus)

	obs.handleRouteEvent(routewatch.RouteEvent{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:  netip.MustParseAddr("192.168.1.1"),
		Protocol: 16,
		Action:   routewatch.ActionAdd,
	})

	obs.handleRouteEvent(routewatch.RouteEvent{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Protocol: 16,
		Action:   routewatch.ActionRemove,
	})

	require.Len(t, bus.batches, 2)
	assert.Equal(t, redistevents.ActionRemove, bus.batches[1].Entries[0].Action)
}

func TestKernelRouteFilterKernel(t *testing.T) {
	bus, unsub := newCapturingBus()
	defer unsub()

	obs := newRouteObserver(bus)

	obs.handleRouteEvent(routewatch.RouteEvent{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Protocol: rtprotKernel,
		Action:   routewatch.ActionAdd,
	})

	assert.Empty(t, bus.batches)
}

func TestKernelRouteFilterRedirect(t *testing.T) {
	bus, unsub := newCapturingBus()
	defer unsub()

	obs := newRouteObserver(bus)

	obs.handleRouteEvent(routewatch.RouteEvent{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Protocol: rtprotRedirect,
		Action:   routewatch.ActionAdd,
	})

	assert.Empty(t, bus.batches)
}

func TestKernelRouteSnapshot(t *testing.T) {
	bus, unsub := newCapturingBus()
	defer unsub()

	obs := newRouteObserver(bus)

	prefixes := []string{"10.0.0.0/24", "172.16.0.0/16", "192.168.1.0/24"}
	for _, p := range prefixes {
		obs.handleRouteEvent(routewatch.RouteEvent{
			Prefix:   netip.MustParsePrefix(p),
			Protocol: 16,
			Action:   routewatch.ActionAdd,
		})
	}

	require.Len(t, bus.batches, 3)
	for i, b := range bus.batches {
		assert.Equal(t, redistevents.ActionAdd, b.Entries[0].Action)
		assert.Equal(t, netip.MustParsePrefix(prefixes[i]), b.Entries[0].Prefix)
	}

	obs.mu.Lock()
	assert.Len(t, obs.announced, 3)
	obs.mu.Unlock()
}

func TestKernelShutdownWithdraw(t *testing.T) {
	bus, unsub := newCapturingBus()
	defer unsub()

	obs := newRouteObserver(bus)

	obs.handleRouteEvent(routewatch.RouteEvent{
		Prefix:   netip.MustParsePrefix("10.0.0.0/24"),
		Protocol: 16,
		Action:   routewatch.ActionAdd,
	})
	obs.handleRouteEvent(routewatch.RouteEvent{
		Prefix:   netip.MustParsePrefix("172.16.0.0/16"),
		Protocol: 3,
		Action:   routewatch.ActionAdd,
	})

	require.Len(t, bus.batches, 2)

	obs.withdrawAll()

	withdrawCount := 0
	for _, b := range bus.batches {
		if b.Entries[0].Action == redistevents.ActionRemove {
			withdrawCount++
		}
	}
	assert.Equal(t, 2, withdrawCount)

	obs.mu.Lock()
	assert.Empty(t, obs.announced)
	obs.mu.Unlock()
}

// Verify mockBus implements ze.EventBus.
var _ ze.EventBus = (*mockBus)(nil)
