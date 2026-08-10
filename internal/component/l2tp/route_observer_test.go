package l2tp

import (
	"log/slog"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/ze"
)

// recordingBus is a minimal ze.EventBus that records all emitted events.
type recordingBus struct {
	mu    sync.Mutex
	emits []emittedEvent
}

type emittedEvent struct {
	namespace string
	eventType string
	payload   any
}

var _ ze.EventBus = (*recordingBus)(nil)

func (b *recordingBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Deep-copy RouteChangeBatch because the producer releases it after Emit.
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

// VALIDATES: AC-21 -- Subsystem.Start registers the l2tp source.
func TestRegisterL2TPSourcesRegistersSource(t *testing.T) {
	// sync.Once means the registration may already have happened in
	// another test. Call it explicitly, then assert lookup.
	registerL2TPSources()

	src, ok := redistribute.LookupSource("l2tp")
	require.True(t, ok, "l2tp source must be registered")
	require.Equal(t, "l2tp", src.Name)
	require.Equal(t, "l2tp", src.Protocol)
}

// VALIDATES: AC-22 -- IPv4 session-up records the /32 address.
func TestRouteObserverInjectsIPv4(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionIPUp(1, 42, "alice", netip.MustParseAddr("192.0.2.7"))

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(1), injected)
	require.Equal(t, uint64(0), withdrawn)
	require.Equal(t, 1, active)
}

// VALIDATES: AC-23 -- IPv6 session-up records the /128 address.
func TestRouteObserverInjectsIPv6(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionIPUp(1, 43, "bob", netip.MustParseAddr("2001:db8::1"))

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(1), injected)
	require.Equal(t, uint64(0), withdrawn)
	require.Equal(t, 1, active)
}

// VALIDATES: dual-stack subscriber gets one record with both addresses.
func TestRouteObserverTracksBothFamiliesPerSession(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionIPUp(1, 44, "carol", netip.MustParseAddr("192.0.2.7"))
	o.OnSessionIPUp(1, 44, "carol", netip.MustParseAddr("2001:db8::7"))

	injected, _, active := o.Stats()
	require.Equal(t, uint64(2), injected)
	require.Equal(t, 1, active, "one session, two families, still one record")
}

// VALIDATES: AC-24 -- session-down withdraws every family for that SID.
func TestRouteObserverWithdrawsOnDown(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionIPUp(1, 45, "dave", netip.MustParseAddr("192.0.2.8"))
	o.OnSessionIPUp(1, 45, "dave", netip.MustParseAddr("2001:db8::8"))
	o.OnSessionDown(1, 45)

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(2), injected)
	require.Equal(t, uint64(2), withdrawn, "dual-stack subscriber withdraws twice")
	require.Equal(t, 0, active)
}

// VALIDATES: OnSessionDown for an unknown SID is a no-op, not an error.
func TestRouteObserverSessionDownUnknownID(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionDown(1, 9999) // never reported up

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(0), injected)
	require.Equal(t, uint64(0), withdrawn)
	require.Equal(t, 0, active)
}

// VALIDATES: TestProducerEchoesReplayIDOnReplayRequest (l2tp) -- reemitAll
// re-emits the current session addresses as adds tagged with the echoed
// ReplayID; the incremental emit stays ReplayID=0 (AC-6, AC-8).
// PREVENTS: a late-joining peer missing l2tp subscriber routes; a wire change
// to the incremental path.
func TestRouteObserverReemitsReplayID(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	o.OnSessionIPUp(1, 42, "alice", netip.MustParseAddr("192.0.2.7"))
	o.OnSessionIPUp(1, 43, "bob", netip.MustParseAddr("2001:db8::1"))

	// Incremental emits carried ReplayID 0.
	for _, e := range bus.events() {
		if b, ok := e.payload.(*redistevents.RouteChangeBatch); ok {
			require.Equal(t, uint64(0), b.ReplayID, "incremental emit must not set ReplayID")
		}
	}
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	o.reemitAll(88)

	evts := bus.events()
	require.Len(t, evts, 2, "both session addresses re-emitted")
	got := map[string]uint64{}
	for _, e := range evts {
		b, ok := e.payload.(*redistevents.RouteChangeBatch)
		require.True(t, ok)
		require.Len(t, b.Entries, 1)
		require.Equal(t, redistevents.ActionAdd, b.Entries[0].Action)
		got[b.Entries[0].Prefix.String()] = b.ReplayID
	}
	require.Equal(t, uint64(88), got["192.0.2.7/32"], "v4 session route re-emitted with replayID")
	require.Equal(t, uint64(88), got["2001:db8::1/128"], "v6 session route re-emitted with replayID")

	// reemitAll(0) is a no-op.
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()
	o.reemitAll(0)
	require.Empty(t, bus.events(), "reemitAll(0) must be a no-op")
}

// VALIDATES: OnSessionIPUp with an invalid address is a no-op.
func TestRouteObserverSkipsInvalidAddr(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionIPUp(1, 46, "eve", netip.Addr{})

	injected, _, active := o.Stats()
	require.Equal(t, uint64(0), injected)
	require.Equal(t, 0, active)
}

// VALIDATES: the RouteObserver interface is satisfied by
// subscriberRouteObserver. Compile-time proof.
func TestSubscriberRouteObserverSatisfiesInterface(t *testing.T) {
	var _ RouteObserver = (*subscriberRouteObserver)(nil)
}

// VALIDATES: AC-1 -- IPCP-up emits one (l2tp, route-change) batch with
// Action=add, Prefix=<addr>/32, Family=ipv4/unicast.
func TestObserver_OnSessionIPUp_EmitsBatch_IPv4(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	o.OnSessionIPUp(1, 50, "alice", netip.MustParseAddr("192.0.2.10"))

	evts := bus.events()
	require.Len(t, evts, 1)
	require.Equal(t, l2tpevents.Namespace, evts[0].namespace)
	require.Equal(t, redistevents.EventType, evts[0].eventType)

	b, ok := evts[0].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok, "payload must be *RouteChangeBatch")
	require.Equal(t, l2tpevents.ProtocolID, b.Protocol)
	require.Equal(t, uint16(1), b.AFI, "ipv4 AFI")
	require.Equal(t, uint8(1), b.SAFI, "unicast SAFI")
	require.Len(t, b.Entries, 1)
	require.Equal(t, redistevents.ActionAdd, b.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("192.0.2.10/32"), b.Entries[0].Prefix)
}

// VALIDATES: AC-2 -- IPv6CP-up emits one (l2tp, route-change) batch with
// Action=add, Prefix=<addr>/128, Family=ipv6/unicast.
func TestObserver_OnSessionIPUp_EmitsBatch_IPv6(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	o.OnSessionIPUp(1, 51, "bob", netip.MustParseAddr("2001:db8::1"))

	evts := bus.events()
	require.Len(t, evts, 1)

	b, ok := evts[0].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, l2tpevents.ProtocolID, b.Protocol)
	require.Equal(t, uint16(2), b.AFI, "ipv6 AFI")
	require.Equal(t, uint8(1), b.SAFI, "unicast SAFI")
	require.Len(t, b.Entries, 1)
	require.Equal(t, redistevents.ActionAdd, b.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("2001:db8::1/128"), b.Entries[0].Prefix)
}

// VALIDATES: AC-3 -- Session teardown with both families up emits two
// remove-batches, one per family.
func TestObserver_OnSessionDown_EmitsRemoveBatches_PerFamily(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	o.OnSessionIPUp(1, 52, "carol", netip.MustParseAddr("192.0.2.20"))
	o.OnSessionIPUp(1, 52, "carol", netip.MustParseAddr("2001:db8::20"))
	bus.mu.Lock()
	bus.emits = nil // clear add events
	bus.mu.Unlock()

	o.OnSessionDown(1, 52)

	evts := bus.events()
	require.Len(t, evts, 2, "one remove-batch per family")

	// Order: v4 first, v6 second (matches code order).
	b0, ok := evts[0].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, redistevents.ActionRemove, b0.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("192.0.2.20/32"), b0.Entries[0].Prefix)

	b1, ok := evts[1].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, redistevents.ActionRemove, b1.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("2001:db8::20/128"), b1.Entries[0].Prefix)
}

// VALIDATES: AC-4 -- Session teardown with only IPv4 up emits one
// remove-batch for ipv4/unicast; no IPv6 emission.
func TestObserver_OnSessionDown_IPv4Only(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	o.OnSessionIPUp(1, 53, "dave", netip.MustParseAddr("192.0.2.30"))
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	o.OnSessionDown(1, 53)

	evts := bus.events()
	require.Len(t, evts, 1, "only one remove-batch for the one family that was up")
	b, ok := evts[0].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, uint16(1), b.AFI, "ipv4")
	require.Equal(t, redistevents.ActionRemove, b.Entries[0].Action)
}

// VALIDATES: AC-5 -- Observer with nil bus records state, does not panic.
func TestObserver_NilBus_StillTracksState(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionIPUp(1, 54, "eve", netip.MustParseAddr("192.0.2.40"))
	o.OnSessionDown(1, 54)

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(1), injected)
	require.Equal(t, uint64(1), withdrawn)
	require.Equal(t, 0, active)
}

// VALIDATES: NCP renegotiation (same family, different address) emits
// remove for old address before add for new address.
// PREVENTS: Orphaned routes in BGP when IPCP renegotiates a new IP.
func TestObserver_OnSessionIPUp_ReplaceAddr_EmitsRemoveThenAdd(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	o.OnSessionIPUp(1, 60, "frank", netip.MustParseAddr("192.0.2.50"))
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	o.OnSessionIPUp(1, 60, "frank", netip.MustParseAddr("192.0.2.51"))

	evts := bus.events()
	require.Len(t, evts, 2, "remove old + add new")

	// First event: remove old address.
	b0, ok := evts[0].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, redistevents.ActionRemove, b0.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("192.0.2.50/32"), b0.Entries[0].Prefix)

	// Second event: add new address.
	b1, ok := evts[1].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, redistevents.ActionAdd, b1.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("192.0.2.51/32"), b1.Entries[0].Prefix)
}

// VALIDATES: Same address re-announced does not emit spurious remove.
func TestObserver_OnSessionIPUp_SameAddr_NoRemove(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	o.OnSessionIPUp(1, 61, "grace", netip.MustParseAddr("192.0.2.60"))
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	o.OnSessionIPUp(1, 61, "grace", netip.MustParseAddr("192.0.2.60"))

	evts := bus.events()
	require.Len(t, evts, 1, "only add, no remove for same address")
	b, ok := evts[0].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, redistevents.ActionAdd, b.Entries[0].Action)
}

// VALIDATES: session-down with no prior IP-up emits nothing.
func TestObserver_OnSessionDown_NoEmission_IfNothingUp(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	o.OnSessionDown(1, 99)

	require.Empty(t, bus.events())
}

// VALIDATES: AC-6 -- framed routes from RADIUS metadata are emitted
// alongside the subscriber /32 on session-up.
func TestObserver_OnSessionIPUp_EmitsFramedRoutes(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	StoreSessionMetadata(1, 70, &AuthMetadata{
		FramedRoutes: []FramedRoute{
			{Prefix: netip.MustParsePrefix("10.0.0.0/8"), Metric: 1},
			{Prefix: netip.MustParsePrefix("172.16.0.0/12"), Metric: 2},
		},
	})
	defer ClearSessionMetadata(1, 70)

	o.OnSessionIPUp(1, 70, "alice", netip.MustParseAddr("192.0.2.70"))

	evts := bus.events()
	require.Len(t, evts, 2, "subscriber /32 + framed routes batch")

	b1, ok := evts[1].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, uint16(1), b1.AFI)
	require.Len(t, b1.Entries, 2)
	require.Equal(t, redistevents.ActionAdd, b1.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), b1.Entries[0].Prefix)
	require.Equal(t, uint32(1), b1.Entries[0].Metric)
	require.Equal(t, netip.MustParsePrefix("172.16.0.0/12"), b1.Entries[1].Prefix)
	require.Equal(t, uint32(2), b1.Entries[1].Metric)
}

// VALIDATES: AC-7 -- framed routes are withdrawn on session-down.
func TestObserver_OnSessionDown_WithdrawsFramedRoutes(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	StoreSessionMetadata(1, 71, &AuthMetadata{
		FramedRoutes: []FramedRoute{
			{Prefix: netip.MustParsePrefix("10.0.0.0/8"), Metric: 1},
		},
	})
	defer ClearSessionMetadata(1, 71)

	o.OnSessionIPUp(1, 71, "bob", netip.MustParseAddr("192.0.2.71"))
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	o.OnSessionDown(1, 71)

	evts := bus.events()
	require.Len(t, evts, 2, "subscriber /32 remove + framed route remove")

	b1, ok := evts[1].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Len(t, b1.Entries, 1)
	require.Equal(t, redistevents.ActionRemove, b1.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("10.0.0.0/8"), b1.Entries[0].Prefix)
}

// VALIDATES: AC-9 -- IPv6 framed routes emitted in ipv6/unicast batch.
func TestObserver_OnSessionIPUp_EmitsIPv6FramedRoutes(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	StoreSessionMetadata(1, 72, &AuthMetadata{
		FramedRoutes: []FramedRoute{
			{Prefix: netip.MustParsePrefix("2001:db8::/32"), Metric: 5},
		},
	})
	defer ClearSessionMetadata(1, 72)

	o.OnSessionIPUp(1, 72, "carol", netip.MustParseAddr("192.0.2.72"))

	evts := bus.events()
	require.Len(t, evts, 2)

	b1, ok := evts[1].payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok)
	require.Equal(t, uint16(2), b1.AFI, "ipv6")
	require.Len(t, b1.Entries, 1)
	require.Equal(t, redistevents.ActionAdd, b1.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("2001:db8::/32"), b1.Entries[0].Prefix)
	require.Equal(t, uint32(5), b1.Entries[0].Metric)
}

// VALIDATES: dual-stack subscriber does not double-emit framed routes.
func TestObserver_OnSessionIPUp_DualStack_FramedRoutesOnce(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	StoreSessionMetadata(1, 74, &AuthMetadata{
		FramedRoutes: []FramedRoute{
			{Prefix: netip.MustParsePrefix("10.0.0.0/8"), Metric: 1},
		},
	})
	defer ClearSessionMetadata(1, 74)

	o.OnSessionIPUp(1, 74, "eve", netip.MustParseAddr("192.0.2.74"))
	o.OnSessionIPUp(1, 74, "eve", netip.MustParseAddr("2001:db8::74"))

	evts := bus.events()
	framedCount := 0
	for _, e := range evts {
		b, ok := e.payload.(*redistevents.RouteChangeBatch)
		if !ok {
			continue
		}
		for _, entry := range b.Entries {
			if entry.Prefix == netip.MustParsePrefix("10.0.0.0/8") {
				framedCount++
			}
		}
	}
	require.Equal(t, 1, framedCount, "framed route emitted exactly once despite dual-stack")
}

// VALIDATES: no framed route emission when metadata has no routes.
func TestObserver_OnSessionIPUp_NoFramedRoutes(t *testing.T) {
	bus := &recordingBus{}
	o := newSubscriberRouteObserver(slog.Default(), bus)

	o.OnSessionIPUp(1, 73, "dave", netip.MustParseAddr("192.0.2.73"))

	evts := bus.events()
	require.Len(t, evts, 1, "only subscriber /32, no framed routes")
}
