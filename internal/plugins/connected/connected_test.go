// Design: docs/architecture/core-design.md -- connected route tests

package connected

import (
	"encoding/json"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/redistevents"
	connectedevents "github.com/ze-software/ze/internal/plugins/connected/events"
)

type recordingBus struct {
	mu    sync.Mutex
	emits []emittedEvent
}

type emittedEvent struct {
	namespace string
	eventType string
	payload   any
}

func (b *recordingBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if batch, ok := payload.(*redistevents.RouteChangeBatch); ok && batch != nil {
		cp := *batch
		cp.Entries = make([]redistevents.RouteChangeEntry, len(batch.Entries))
		copy(cp.Entries, batch.Entries)
		payload = &cp
	}
	b.emits = append(b.emits, emittedEvent{namespace, eventType, payload})
	return 0, nil
}

func (b *recordingBus) Subscribe(_, _ string, _ func(payload any)) func() {
	return func() {}
}

func (b *recordingBus) events() []emittedEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]emittedEvent, len(b.emits))
	copy(out, b.emits)
	return out
}

func makePayload(addr string, prefixLen int) []byte {
	p := addrPayload{
		Name:         "eth0",
		Unit:         0,
		Address:      addr,
		PrefixLength: prefixLen,
		Family:       "ipv4",
	}
	data, _ := json.Marshal(p)
	return data
}

// VALIDATES: AC-1 -- connected registers as redistribute source.
func TestConnectedRegistersSource(t *testing.T) {
	registerConnectedSources()
	src, ok := redistribute.LookupSource("connected")
	require.True(t, ok)
	require.Equal(t, "connected", src.Name)
	require.Equal(t, "connected", src.Protocol)
}

// VALIDATES: AC-2 -- addr-added emits RouteChangeBatch with network prefix.
func TestConnectedEmitAdd(t *testing.T) {
	bus := &recordingBus{}
	obs := newRouteObserver(bus)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))

	evts := bus.events()
	require.Len(t, evts, 1)
	require.Equal(t, connectedevents.Namespace, evts[0].namespace)
	require.Equal(t, redistevents.EventType, evts[0].eventType)

	b, ok := evts[0].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, connectedevents.ProtocolID, b.Protocol)
	require.Equal(t, uint16(1), b.AFI)
	require.Equal(t, uint8(1), b.SAFI)
	require.Len(t, b.Entries, 1)
	require.Equal(t, redistevents.ActionAdd, b.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), b.Entries[0].Prefix)
}

// VALIDATES: AC-3 -- addr-removed emits ActionRemove.
func TestConnectedEmitRemove(t *testing.T) {
	bus := &recordingBus{}
	obs := newRouteObserver(bus)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	obs.handleAddrRemoved(makePayload("10.0.0.1", 24))

	evts := bus.events()
	require.Len(t, evts, 1)
	b, ok := evts[0].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, redistevents.ActionRemove, b.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), b.Entries[0].Prefix)
}

// VALIDATES: AC-4 -- host address 10.0.0.1/24 emits network 10.0.0.0/24.
func TestConnectedNetworkPrefix(t *testing.T) {
	bus := &recordingBus{}
	obs := newRouteObserver(bus)

	obs.handleAddrAdded(makePayload("192.168.1.100", 16))

	evts := bus.events()
	require.Len(t, evts, 1)
	b, ok := evts[0].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, netip.MustParsePrefix("192.168.0.0/16"), b.Entries[0].Prefix)
}

// VALIDATES: AC-10 -- nil bus does not panic.
func TestConnectedNilBus(t *testing.T) {
	obs := newRouteObserver(nil)
	obs.handleAddrAdded(makePayload("10.0.0.1", 24))
	obs.handleAddrRemoved(makePayload("10.0.0.1", 24))
}

// VALIDATES: IPv6 connected route.
func TestConnectedIPv6(t *testing.T) {
	bus := &recordingBus{}
	obs := newRouteObserver(bus)

	p := addrPayload{
		Name:         "eth0",
		Unit:         0,
		Address:      "2001:db8::1",
		PrefixLength: 48,
		Family:       "ipv6",
	}
	data, _ := json.Marshal(p)
	obs.handleAddrAdded(data)

	evts := bus.events()
	require.Len(t, evts, 1)
	b, ok := evts[0].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, uint16(2), b.AFI, "ipv6")
	require.Equal(t, netip.MustParsePrefix("2001:db8::/48"), b.Entries[0].Prefix)
}

// VALIDATES: multiple addresses on same prefix emit only once.
func TestConnectedDuplicatePrefix(t *testing.T) {
	bus := &recordingBus{}
	obs := newRouteObserver(bus)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))
	obs.handleAddrAdded(makePayload("10.0.0.2", 24))

	evts := bus.events()
	require.Len(t, evts, 1, "second addr on same prefix should not emit")
}

// VALIDATES: prefix not withdrawn until all addresses removed.
func TestConnectedRemoveOnlyAfterAllAddrs(t *testing.T) {
	bus := &recordingBus{}
	obs := newRouteObserver(bus)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))
	obs.handleAddrAdded(makePayload("10.0.0.2", 24))
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	obs.handleAddrRemoved(makePayload("10.0.0.1", 24))
	require.Empty(t, bus.events(), "still one addr on prefix, no remove")

	obs.handleAddrRemoved(makePayload("10.0.0.2", 24))
	evts := bus.events()
	require.Len(t, evts, 1)
	b, ok := evts[0].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, redistevents.ActionRemove, b.Entries[0].Action)
}

func TestConnectedMalformedPayload(t *testing.T) {
	bus := &recordingBus{}
	obs := newRouteObserver(bus)

	obs.handleAddrAdded([]byte("not json"))
	require.Empty(t, bus.events())

	obs.handleAddrAdded(42)
	require.Empty(t, bus.events())
}

// VALIDATES: TestProducerEchoesReplayIDOnReplayRequest (connected) -- reemitAll
// re-emits the current prefix set as adds tagged with the echoed ReplayID; the
// incremental emit stays ReplayID=0 (AC-6, AC-8).
// PREVENTS: a late-joining peer missing connected routes; a wire change to the
// incremental path.
func TestConnectedReemitsReplayID(t *testing.T) {
	bus := &recordingBus{}
	obs := newRouteObserver(bus)

	obs.handleAddrAdded(makePayload("10.0.0.1", 24))
	obs.handleAddrAdded(makePayload("192.168.1.1", 16))

	// Incremental emits carry ReplayID 0 (no behavior change).
	for _, e := range bus.events() {
		b, ok := e.payload.(*redistevents.RouteChangeBatch)
		require.True(t, ok)
		require.Equal(t, uint64(0), b.ReplayID, "incremental emit must not set ReplayID")
	}
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	obs.reemitAll(77)

	evts := bus.events()
	require.Len(t, evts, 2, "both current prefixes re-emitted")
	prefixes := map[string]bool{}
	for _, e := range evts {
		b, ok := e.payload.(*redistevents.RouteChangeBatch)
		require.True(t, ok)
		require.Equal(t, uint64(77), b.ReplayID, "re-emit echoes the replayID")
		require.Len(t, b.Entries, 1)
		require.Equal(t, redistevents.ActionAdd, b.Entries[0].Action)
		prefixes[b.Entries[0].Prefix.String()] = true
	}
	require.True(t, prefixes["10.0.0.0/24"])
	require.True(t, prefixes["192.168.0.0/16"])

	// A replayID of 0 is a no-op guard (never re-emit as incremental).
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()
	obs.reemitAll(0)
	require.Empty(t, bus.events(), "reemitAll(0) must be a no-op")
}

// VALIDATES: an interface address FRR's zebra never redistributes (loopback,
// IPv4 and IPv6 link-local, class D, and the zero network BIRD rejects as an
// invalid address) reaches the Loc-RIB but never the redistribution feed, on
// add, on remove and on a late-join replay.
// PREVENTS: `redistribute { destination isis|ospf|bgp { import connected } }`
// exporting 127.0.0.0/8 from `lo`, or fe80::/64 from every link, to a peer.
// Method: drive each address through the observer with a recording bus and a
// recording Loc-RIB sink, then read both; 10.0.0.1/24 is the routable control.
func TestConnectedNeverRedistributesUnroutablePrefixes(t *testing.T) {
	cases := []struct {
		addr      string
		prefixLen int
	}{
		{"127.0.0.1", 8},
		{"::1", 128},
		{"169.254.10.1", 16},
		{"fe80::1", 64},
		{"224.0.0.5", 24},
		{"0.1.2.3", 8},
	}
	for _, tc := range cases {
		t.Run(tc.addr, func(t *testing.T) {
			bus := &recordingBus{}
			sink := &recordingSink{}
			obs := newRouteObserver(bus)
			obs.setLocRIB(nil, sink)

			obs.handleAddrAdded(makePayload(tc.addr, tc.prefixLen))
			require.Len(t, sink.inserted, 1, "the kernel holds the connected route, so the Loc-RIB still carries it")
			require.Empty(t, bus.events(), "an unroutable prefix must never reach the redistribution feed")

			obs.reemitAll(9)
			require.Empty(t, bus.events(), "a late-join replay must not export it either")

			obs.handleAddrRemoved(makePayload(tc.addr, tc.prefixLen))
			require.Len(t, sink.removed, 1)
			require.Empty(t, bus.events(), "its removal emits nothing, because its add emitted nothing")
		})
	}

	bus := &recordingBus{}
	obs := newRouteObserver(bus)
	obs.handleAddrAdded(makePayload("10.0.0.1", 24))
	obs.reemitAll(9)
	require.Len(t, bus.events(), 2, "a routable prefix is emitted and replayed")
}

// lockProbeBus runs probe for every replay batch emitted on it, at the moment
// of the emit, and records nothing else.
type lockProbeBus struct {
	probe func()
}

func (b *lockProbeBus) Emit(_, _ string, payload any) (int, error) {
	if batch, ok := payload.(*redistevents.RouteChangeBatch); ok && batch != nil && batch.ReplayID != 0 {
		b.probe()
	}
	return 0, nil
}

func (b *lockProbeBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

// TestConnectedReplayEmitsUnderEmitLock proves every replay emit holds emitMu
// exclusive, so an incremental Remove of a snapshotted prefix waits until the
// replay's Add is published.
//
// VALIDATES: reemitAll holds emitMu from its snapshot to its last emit; the
// bus probes it with TryRLock at each replay emit.
// PREVENTS: a replay Add reaching the orchestrator after the Remove of the
// same prefix, which revives a removed connected route.
func TestConnectedReplayEmitsUnderEmitLock(t *testing.T) {
	bus := &lockProbeBus{}
	obs := newRouteObserver(bus)
	unordered, replayed := 0, 0
	bus.probe = func() {
		replayed++
		if obs.emitMu.TryRLock() {
			unordered++
			obs.emitMu.RUnlock()
		}
	}
	obs.handleAddrAdded(makePayload("10.0.0.1", 24))
	obs.handleAddrAdded(makePayload("192.168.1.1", 16))

	obs.reemitAll(5)

	require.Equal(t, 2, replayed)
	require.Zero(t, unordered, "an incremental emit could interleave with the replay")
}
