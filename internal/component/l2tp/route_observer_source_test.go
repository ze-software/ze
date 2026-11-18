// Design: docs/guide/redistribution.md -- session-owned subscriber contributions
// Related: route_observer.go -- address/framed add, replay and removal publishers

package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"

	"github.com/stretchr/testify/require"
)

// TestSubscriberRouteSourceInstanceSurvivesReplayAndRemove gives two sessions
// the same address and framed prefix, then checks each actual lifecycle event
// retains the original contribution token and one session's removal leaves the
// other's replay intact.
func TestSubscriberRouteSourceInstanceSurvivesReplayAndRemove(t *testing.T) {
	bus := &recordingBus{}
	observer := newSubscriberRouteObserver(slog.Default(), bus)
	address := netip.MustParseAddr("192.0.2.77")
	framed := netip.MustParsePrefix("198.51.100.0/24")
	var tokens [2]uint32
	for index, session := range []uint16{701, 702} {
		before := len(bus.events())
		observer.OnSessionIPUp(fixedRouteFixtureBirth(t, uint64(session)), 17, session, "subscriber", address, []FramedRoute{{Prefix: framed, Metric: 9}})
		batch := routeIdentityBatch(t, bus.events()[before])
		tokens[index] = batch.Entries[0].SourceInstance
	}
	require.NotZero(t, tokens[0])
	require.NotZero(t, tokens[1])
	require.NotEqual(t, tokens[0], tokens[1])
	observer.reemitAll(53)
	observer.OnSessionDown(fixedRouteFixtureBirth(t, 701), 17, 701)
	observer.reemitAll(54)
	normal := map[redistevents.RouteAction]map[uint32]int{
		redistevents.ActionAdd: {}, redistevents.ActionRemove: {},
	}
	replays := map[uint64]map[uint32]int{53: {}, 54: {}}
	for _, emitted := range bus.events() {
		batch, ok := emitted.payload.(*redistevents.RouteChangeBatch)
		require.True(t, ok)
		for _, entry := range batch.Entries {
			if entry.Prefix == framed {
				require.Equal(t, uint32(9), entry.Metric)
			} else {
				require.Equal(t, prefixForAddr(address), entry.Prefix)
			}
			if batch.ReplayID == 0 {
				normal[entry.Action][entry.SourceInstance]++
			} else {
				replays[batch.ReplayID][entry.SourceInstance]++
			}
		}
	}
	first, second := tokens[0], tokens[1]
	require.Equal(t, map[uint32]int{first: 2, second: 2}, normal[redistevents.ActionAdd])
	require.Equal(t, map[uint32]int{first: 2}, normal[redistevents.ActionRemove])
	require.Equal(t, map[uint32]int{first: 2, second: 2}, replays[53])
	require.Equal(t, map[uint32]int{second: 2}, replays[54])
}
