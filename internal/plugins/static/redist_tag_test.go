package static

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"

	"github.com/stretchr/testify/require"
)

// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-1 -- emitRouteChangeID
// copies staticRoute.Tag into the RouteChangeEntry it emits.
// PREVENTS: the tag leaf going back to being stored and never published, which is
// the defect this spec exists to remove.
// TestStaticEmitCarriesRouteTag proves the `tag` leaf of a static route reaches the
// redistribute bus: the emitted RouteChangeEntry carries the configured value, and a
// route with no tag emits zero. Method: apply two routes through the route manager
// with a recording bus and read the entries back.
func TestStaticEmitCarriesRouteTag(t *testing.T) {
	bus := &staticRecordingBus{}
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })

	rm := newRouteManager(&mockStaticBackend{})
	require.NoError(t, rm.applyRoutes([]staticRoute{
		{
			Prefix:   netip.MustParsePrefix("10.0.0.0/8"),
			Action:   actionForward,
			Tag:      4242,
			NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
		},
		{
			Prefix:   netip.MustParsePrefix("192.0.2.0/24"),
			Action:   actionForward,
			NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
		},
	}))

	tags := map[string]uint32{}
	for _, b := range bus.events() {
		for _, e := range b.Entries {
			require.Equal(t, redistevents.ActionAdd, e.Action)
			tags[e.Prefix.String()] = e.Tag
		}
	}
	require.Equal(t, uint32(4242), tags["10.0.0.0/8"], "the tag leaf must reach the redistribute bus")
	require.Equal(t, uint32(0), tags["192.0.2.0/24"], "a route with no tag emits zero")
}
