// Design: docs/guide/l2tp.md -- independent subscriber route lifecycles
// Related: route_observer.go -- IP assignment, withdrawal and replay

package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/redistevents"
)

// TestRouteObserverKeepsSameSessionIDOnTwoTunnelsApart checks that assigning
// another tunnel's subscriber cannot withdraw the first subscriber's route.
// The observer's actual route-change batches must contain both independent adds.
func TestRouteObserverKeepsSameSessionIDOnTwoTunnelsApart(t *testing.T) {
	bus := &recordingBus{}
	observer := newSubscriberRouteObserver(slog.Default(), bus)

	observer.OnSessionIPUp(fixedRouteFixtureBirth(t, 101), 101, 7, "first", netip.MustParseAddr("192.0.2.10"), nil)
	observer.OnSessionIPUp(fixedRouteFixtureBirth(t, 202), 202, 7, "second", netip.MustParseAddr("192.0.2.20"), nil)

	events := bus.events()
	require.Len(t, events, 2, "a different tunnel's assignment must not withdraw an existing subscriber")
	first := routeIdentityBatch(t, events[0])
	require.Equal(t, redistevents.ActionAdd, first.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("192.0.2.10/32"), first.Entries[0].Prefix)
	second := routeIdentityBatch(t, events[1])
	require.Equal(t, redistevents.ActionAdd, second.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("192.0.2.20/32"), second.Entries[0].Prefix)

	injected, withdrawn, active := observer.Stats()
	require.Equal(t, uint64(2), injected)
	require.Zero(t, withdrawn)
	require.Equal(t, 2, active, "both subscribers must retain their own route record")
}

// TestRouteObserverSessionDownKeepsOtherTunnelReplay checks one subscriber's
// withdrawal and the surviving subscriber's replay as one lifecycle sequence.
// A newly joined redistribute consumer must still receive the survivor's route.
func TestRouteObserverSessionDownKeepsOtherTunnelReplay(t *testing.T) {
	bus := &recordingBus{}
	observer := newSubscriberRouteObserver(slog.Default(), bus)

	observer.OnSessionIPUp(fixedRouteFixtureBirth(t, 101), 101, 7, "first", netip.MustParseAddr("192.0.2.10"), nil)
	observer.OnSessionIPUp(fixedRouteFixtureBirth(t, 202), 202, 7, "second", netip.MustParseAddr("192.0.2.20"), nil)
	beforeDown := len(bus.events())

	observer.OnSessionDown(fixedRouteFixtureBirth(t, 101), 101, 7)
	observer.reemitAll(91)

	events := bus.events()[beforeDown:]
	require.Len(t, events, 2, "withdraw the selected route and replay the other tunnel's surviving route")
	withdrawal := routeIdentityBatch(t, events[0])
	require.Zero(t, withdrawal.ReplayID)
	require.Equal(t, redistevents.ActionRemove, withdrawal.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("192.0.2.10/32"), withdrawal.Entries[0].Prefix)
	replay := routeIdentityBatch(t, events[1])
	require.Equal(t, uint64(91), replay.ReplayID)
	require.Equal(t, redistevents.ActionAdd, replay.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("192.0.2.20/32"), replay.Entries[0].Prefix)

	injected, withdrawn, active := observer.Stats()
	require.Equal(t, uint64(2), injected)
	require.Equal(t, uint64(1), withdrawn)
	require.Equal(t, 1, active)
}

// TestRouteObserverKeepsSameTupleOnTwoListenersApart gives two subscribers the
// same tunnel ID and session ID, as two listeners' reactors can, so only their
// births differ. Both assignments must add a route and keep a record; ending
// the first must withdraw only its route, and a replay must still carry the
// second. MUTATION: a record keyed by TID/SID replaces the first address with
// the second and withdraws both on the first one's end.
func TestRouteObserverKeepsSameTupleOnTwoListenersApart(t *testing.T) {
	bus := &recordingBus{}
	observer := newSubscriberRouteObserver(slog.Default(), bus)
	first, second := fixedRouteFixtureBirth(t, 301), fixedRouteFixtureBirth(t, 302)

	observer.OnSessionIPUp(first, 17, 7, "first", netip.MustParseAddr("192.0.2.31"), nil)
	observer.OnSessionIPUp(second, 17, 7, "second", netip.MustParseAddr("192.0.2.32"), nil)

	events := bus.events()
	require.Len(t, events, 2, "the second listener's assignment must not withdraw the first")
	for index, prefix := range []string{"192.0.2.31/32", "192.0.2.32/32"} {
		batch := routeIdentityBatch(t, events[index])
		require.Equal(t, redistevents.ActionAdd, batch.Entries[0].Action)
		require.Equal(t, netip.MustParsePrefix(prefix), batch.Entries[0].Prefix)
	}
	injected, withdrawn, active := observer.Stats()
	require.Equal(t, uint64(2), injected)
	require.Zero(t, withdrawn)
	require.Equal(t, 2, active, "equal labels must keep two route records")

	observer.OnSessionDown(first, 17, 7)
	observer.reemitAll(93)

	events = bus.events()[2:]
	require.Len(t, events, 2, "withdraw the ended birth's route and replay the survivor's")
	withdrawal := routeIdentityBatch(t, events[0])
	require.Zero(t, withdrawal.ReplayID)
	require.Equal(t, redistevents.ActionRemove, withdrawal.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("192.0.2.31/32"), withdrawal.Entries[0].Prefix)
	replay := routeIdentityBatch(t, events[1])
	require.Equal(t, uint64(93), replay.ReplayID)
	require.Equal(t, redistevents.ActionAdd, replay.Entries[0].Action)
	require.Equal(t, netip.MustParsePrefix("192.0.2.32/32"), replay.Entries[0].Prefix)
	injected, withdrawn, active = observer.Stats()
	require.Equal(t, uint64(2), injected)
	require.Equal(t, uint64(1), withdrawn)
	require.Equal(t, 1, active, "the survivor keeps its route record")
}

func routeIdentityBatch(t *testing.T, event emittedEvent) *redistevents.RouteChangeBatch {
	t.Helper()
	batch, ok := event.payload.(*redistevents.RouteChangeBatch)
	require.True(t, ok, "route observer must publish a route-change batch")
	require.NotNil(t, batch)
	require.Len(t, batch.Entries, 1)
	return batch
}
