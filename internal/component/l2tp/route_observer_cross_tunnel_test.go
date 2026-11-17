// Design: docs/guide/redistribution.md -- full subscriber contribution identity
// Related: route_observer.go -- retained tuple, address/framed replay and removal

package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"

	"github.com/stretchr/testify/require"
)

// TestSubscriberRouteCrossTunnelIdentityAndWrongDown drives two tunnels with the
// same session ID and address/framed prefixes. A foreign tunnel's Down is inert;
// retiring one real tuple removes only its facts and replays the other's full
// dual-stack/framed set with the original source identity and metric.
func TestSubscriberRouteCrossTunnelIdentityAndWrongDown(t *testing.T) {
	bus := &recordingBus{}
	observer := newSubscriberRouteObserver(slog.Default(), bus)
	framed := netip.MustParsePrefix("198.51.100.0/24")
	address4 := netip.MustParseAddr("192.0.2.77")
	for _, tunnel := range []uint16{101, 202} {
		id := l2tpSessionID(tunnel, 7)
		StoreSessionMetadata(id, &AuthMetadata{FramedRoutes: []FramedRoute{{Prefix: framed, Metric: uint32(tunnel)}}})
		t.Cleanup(func() { ClearSessionMetadata(id) })
		observer.OnSessionIPUp(tunnel, 7, "subscriber", address4)
		observer.OnSessionIPUp(tunnel, 7, "subscriber", netip.MustParseAddr("2001:db8::77"))
	}
	_, _, active := observer.Stats()
	require.Equal(t, 2, active)
	first := uint32(101)<<16 | 7
	second := uint32(202)<<16 | 7
	before := len(bus.events())
	observer.OnSessionDown(303, 7)
	require.Len(t, bus.events(), before, "an unknown full tuple withdraws nothing")
	observer.reemitAll(71)
	observer.OnSessionDown(101, 7)
	observer.OnSessionDown(101, 7)
	observer.reemitAll(72)
	replayed := map[uint64]map[uint32]int{71: {}, 72: {}}
	removed := map[uint32]int{}
	for _, emitted := range bus.events() {
		batch, ok := emitted.payload.(*redistevents.RouteChangeBatch)
		require.True(t, ok)
		for _, entry := range batch.Entries {
			require.Contains(t, []uint32{first, second}, entry.SourceInstance)
			if entry.Prefix == framed {
				require.Equal(t, entry.SourceInstance>>16, entry.Metric)
			}
			if batch.ReplayID != 0 {
				replayed[batch.ReplayID][entry.SourceInstance]++
			} else if entry.Action == redistevents.ActionRemove {
				removed[entry.SourceInstance]++
			}
		}
	}
	require.Equal(t, map[uint32]int{first: 3, second: 3}, replayed[71])
	require.Equal(t, map[uint32]int{first: 3}, removed)
	require.Equal(t, map[uint32]int{second: 3}, replayed[72])
	injected, withdrawn, active := observer.Stats()
	require.Equal(t, uint64(4), injected)
	require.Equal(t, uint64(2), withdrawn)
	require.Equal(t, 1, active)
}
