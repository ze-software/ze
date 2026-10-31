package static

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"

	"github.com/stretchr/testify/require"
)

type staticEmitted struct {
	action redistevents.RouteAction
	prefix string
}

func staticEmits(bus *staticRecordingBus) []staticEmitted {
	var got []staticEmitted
	for _, b := range bus.events() {
		for _, e := range b.Entries {
			got = append(got, staticEmitted{e.Action, e.Prefix.String()})
		}
	}
	return got
}

// VALIDATES: a main-table blackhole or reject static route is a redistribute
// source, the way FRR and BIRD treat one. FRR's zebra_redistribute_check
// (zebra/redistribute.c) selects a route by type and VRF with no next-hop test,
// zsend_redistribute_route carries NEXTHOP_TYPE_BLACKHOLE, and ospfd keeps such
// routes because redistributing them "is only way to summarize routes in ASBR"
// (ospfd/ospf_zebra.c). BIRD's static_announce_rte (proto/static/static.c)
// announces every r->dest, and ospf_rt_notify (proto/ospf/topology.c) exports
// a non-unicast route with a zero forwarding address.
// PREVENTS: the operator's `blackhole { }` aggregate never reaching BGP, OSPF
// or IS-IS through `import static`.
// Method: apply a blackhole and a reject route, replace a forward route by a
// blackhole one, replay, then remove them all, and read the emitted events.
func TestStaticBlackholeAndRejectRedistribute(t *testing.T) {
	bus := &staticRecordingBus{}
	setEventBus(bus)
	t.Cleanup(func() { eventBusPtr.Store(nil) })

	fwd := staticRoute{
		Prefix:   netip.MustParsePrefix("10.0.0.0/8"),
		Action:   actionForward,
		NextHops: []nextHop{{Address: netip.MustParseAddr("1.1.1.1"), Weight: 1}},
	}
	hole := staticRoute{Prefix: netip.MustParsePrefix("192.0.2.0/24"), Action: actionBlackhole}
	rej := staticRoute{Prefix: netip.MustParsePrefix("198.51.100.0/24"), Action: actionReject}

	rm := newTestRouteManager(&mockStaticBackend{})
	require.NoError(t, rm.applyRoutes([]staticRoute{fwd, hole, rej}))
	require.ElementsMatch(t, []staticEmitted{
		{redistevents.ActionAdd, "10.0.0.0/8"},
		{redistevents.ActionAdd, "192.0.2.0/24"},
		{redistevents.ActionAdd, "198.51.100.0/24"},
	}, staticEmits(bus), "blackhole and reject routes are announced like a forward route")

	// A forward route that becomes a blackhole is replaced, never withdrawn first.
	bus.reset()
	fwdHole := fwd
	fwdHole.Action = actionBlackhole
	fwdHole.NextHops = nil
	require.NoError(t, rm.applyRoutes([]staticRoute{fwdHole, hole, rej}))
	require.Equal(t, []staticEmitted{{redistevents.ActionAdd, "10.0.0.0/8"}}, staticEmits(bus),
		"forward to blackhole is an implicit replace")

	bus.reset()
	rm.reemitAll(9)
	require.Len(t, staticEmits(bus), 3, "replay re-emits the blackhole and reject routes")

	bus.reset()
	require.NoError(t, rm.applyRoutes(nil))
	require.ElementsMatch(t, []staticEmitted{
		{redistevents.ActionRemove, "10.0.0.0/8"},
		{redistevents.ActionRemove, "192.0.2.0/24"},
		{redistevents.ActionRemove, "198.51.100.0/24"},
	}, staticEmits(bus), "removing an announced blackhole or reject route withdraws it")
}
