package l2tp

import (
	"log/slog"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	l2tpevents "codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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
	RegisterL2TPSources()

	src, ok := redistribute.LookupSource("l2tp")
	require.True(t, ok, "l2tp source must be registered")
	require.Equal(t, "l2tp", src.Name)
	require.Equal(t, "l2tp", src.Protocol)
}

// VALIDATES: AC-22 -- IPv4 session-up records the /32 address.
func TestRouteObserverInjectsIPv4(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionIPUp(42, "alice", netip.MustParseAddr("192.0.2.7"))

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(1), injected)
	require.Equal(t, uint64(0), withdrawn)
	require.Equal(t, 1, active)
}

// VALIDATES: AC-23 -- IPv6 session-up records the /128 address.
func TestRouteObserverInjectsIPv6(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionIPUp(43, "bob", netip.MustParseAddr("2001:db8::1"))

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(1), injected)
	require.Equal(t, uint64(0), withdrawn)
	require.Equal(t, 1, active)
}

// VALIDATES: dual-stack subscriber gets one record with both addresses.
func TestRouteObserverTracksBothFamiliesPerSession(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionIPUp(44, "carol", netip.MustParseAddr("192.0.2.7"))
	o.OnSessionIPUp(44, "carol", netip.MustParseAddr("2001:db8::7"))

	injected, _, active := o.Stats()
	require.Equal(t, uint64(2), injected)
	require.Equal(t, 1, active, "one session, two families, still one record")
}

// VALIDATES: AC-24 -- session-down withdraws every family for that SID.
func TestRouteObserverWithdrawsOnDown(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionIPUp(45, "dave", netip.MustParseAddr("192.0.2.8"))
	o.OnSessionIPUp(45, "dave", netip.MustParseAddr("2001:db8::8"))
	o.OnSessionDown(45)

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(2), injected)
	require.Equal(t, uint64(2), withdrawn, "dual-stack subscriber withdraws twice")
	require.Equal(t, 0, active)
}

// VALIDATES: OnSessionDown for an unknown SID is a no-op, not an error.
func TestRouteObserverSessionDownUnknownID(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionDown(9999) // never reported up

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(0), injected)
	require.Equal(t, uint64(0), withdrawn)
	require.Equal(t, 0, active)
}

// VALIDATES: OnSessionIPUp with an invalid address is a no-op.
func TestRouteObserverSkipsInvalidAddr(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default(), nil)

	o.OnSessionIPUp(46, "eve", netip.Addr{})

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

	o.OnSessionIPUp(50, "alice", netip.MustParseAddr("192.0.2.10"))

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

	o.OnSessionIPUp(51, "bob", netip.MustParseAddr("2001:db8::1"))

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

	o.OnSessionIPUp(52, "carol", netip.MustParseAddr("192.0.2.20"))
	o.OnSessionIPUp(52, "carol", netip.MustParseAddr("2001:db8::20"))
	bus.mu.Lock()
	bus.emits = nil // clear add events
	bus.mu.Unlock()

	o.OnSessionDown(52)

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

	o.OnSessionIPUp(53, "dave", netip.MustParseAddr("192.0.2.30"))
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	o.OnSessionDown(53)

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

	o.OnSessionIPUp(54, "eve", netip.MustParseAddr("192.0.2.40"))
	o.OnSessionDown(54)

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

	o.OnSessionIPUp(60, "frank", netip.MustParseAddr("192.0.2.50"))
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	o.OnSessionIPUp(60, "frank", netip.MustParseAddr("192.0.2.51"))

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

	o.OnSessionIPUp(61, "grace", netip.MustParseAddr("192.0.2.60"))
	bus.mu.Lock()
	bus.emits = nil
	bus.mu.Unlock()

	o.OnSessionIPUp(61, "grace", netip.MustParseAddr("192.0.2.60"))

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

	o.OnSessionDown(99)

	require.Empty(t, bus.events())
}
