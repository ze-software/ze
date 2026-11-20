// Design: docs/architecture/l2tp/subscriber-session-model.md -- actor-captured framed route input
// Related: actor_route_birth_test.go -- actual PAP/IPCP and original route owner

package l2tp

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/redistevents"
)

// TestActorRouteCapturesOriginalIPv6FramedMetadata follows the old IPv6-framed
// metadata case through the actual actor instead of giving routes directly to
// the observer. The admission opens IPCP, then IPv6CP: the IPv4 install adds
// its /32 alone and the IPv6 install adds the IPv6 framed route, each family's
// routes on its own install (AC-4). This is no native-I/O proof.
// MUTATION: dropping the actor's captured framedRoutes loses the IPv6 add,
// replay and withdrawal while the same real PPP peer still answers Echo;
// emitting every framed route on the first install puts the IPv6 route in the
// IPv4 install's adds.
func TestActorRouteCapturesOriginalIPv6FramedMetadata(t *testing.T) {
	captureBirthSIDDraw(t)
	observer := newRecordedActorRoutes(t)
	prefix := netip.MustParsePrefix("2001:db8::/32")
	admission := newActorRouteAdmission(t, observer, actorRouteInput{
		username: "ipv6-framed", address: netip.MustParseAddr("192.0.2.72"),
		framedRoutes: []FramedRoute{{Prefix: prefix, Metric: 5}}, ipv6: true,
	})
	actorRouteEcho(t, admission.wire)
	observer.replay(t, 93)
	admission.reactor.handleLCPDown(admission.wire.beginRestart(t))

	events := observer.routes.events()
	addsIPv4 := actorRouteEntries(t, events[:admission.routesAfterIPv4], redistevents.ActionAdd, 0)
	addsIPv6 := actorRouteEntries(t, events[admission.routesAfterIPv4:admission.routesAfterIPv6], redistevents.ActionAdd, 0)
	adds := actorRouteEntries(t, events, redistevents.ActionAdd, 0)
	replayed := actorRouteEntries(t, events, redistevents.ActionAdd, 93)
	removed := actorRouteEntries(t, events, redistevents.ActionRemove, 0)
	address := netip.MustParsePrefix("192.0.2.72/32")
	require.Len(t, addsIPv4, 1, "the IPv4 install must add its /32 alone")
	require.Equal(t, address, addsIPv4[0].Prefix)
	token := addsIPv4[0].SourceInstance
	require.NotZero(t, token)
	require.Len(t, addsIPv6, 1, "the IPv6 install must add the original IPv6 framed route")
	require.Equal(t, prefix, addsIPv6[0].Prefix)
	require.Equal(t, uint32(5), addsIPv6[0].Metric)
	require.Equal(t, token, addsIPv6[0].SourceInstance)
	for _, entries := range [][]redistevents.RouteChangeEntry{adds, replayed, removed} {
		require.Len(t, entries, 2)
		require.ElementsMatch(t, []netip.Prefix{address, prefix}, []netip.Prefix{entries[0].Prefix, entries[1].Prefix})
		for _, entry := range entries {
			require.Equal(t, token, entry.SourceInstance)
			if entry.Prefix == prefix {
				require.Equal(t, uint32(5), entry.Metric)
			}
		}
	}
}

// TestActorRouteCapturesEveryOriginalFramedRoute preserves the old multiple-
// framed-route metadata question at its new actor owner. One real admission
// supplies two IPv4 and one IPv6 route and opens IPCP, then IPv6CP: the IPv4
// install adds the /32 and both IPv4 routes, the IPv6 install the IPv6 route
// (AC-4), and all survive add/replay/withdrawal with their original metrics
// and token.
// MUTATION: truncating the actor's captured framedRoutes to its first entry
// loses list members even when the direct observer's framed-route tests pass.
func TestActorRouteCapturesEveryOriginalFramedRoute(t *testing.T) {
	captureBirthSIDDraw(t)
	observer := newRecordedActorRoutes(t)
	framed := []FramedRoute{
		{Prefix: netip.MustParsePrefix("10.0.0.0/8"), Metric: 1},
		{Prefix: netip.MustParsePrefix("172.16.0.0/12"), Metric: 2},
		{Prefix: netip.MustParsePrefix("2001:db8::/32"), Metric: 5},
	}
	admission := newActorRouteAdmission(t, observer, actorRouteInput{
		username: "all-framed", address: netip.MustParseAddr("192.0.2.70"), framedRoutes: framed, ipv6: true,
	})
	actorRouteEcho(t, admission.wire)
	observer.replay(t, 94)
	admission.reactor.handleLCPDown(admission.wire.beginRestart(t))

	events := observer.routes.events()
	addsIPv4 := actorRouteEntries(t, events[:admission.routesAfterIPv4], redistevents.ActionAdd, 0)
	addsIPv6 := actorRouteEntries(t, events[admission.routesAfterIPv4:admission.routesAfterIPv6], redistevents.ActionAdd, 0)
	adds := actorRouteEntries(t, events, redistevents.ActionAdd, 0)
	replayed := actorRouteEntries(t, events, redistevents.ActionAdd, 94)
	removed := actorRouteEntries(t, events, redistevents.ActionRemove, 0)
	prefixes := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.70/32"), framed[0].Prefix, framed[1].Prefix, framed[2].Prefix,
	}
	metrics := map[netip.Prefix]uint32{prefixes[0]: 0, framed[0].Prefix: 1, framed[1].Prefix: 2, framed[2].Prefix: 5}
	require.Len(t, addsIPv4, 3, "the IPv4 install must add the /32 and both IPv4 framed routes")
	require.ElementsMatch(t, prefixes[:3], []netip.Prefix{addsIPv4[0].Prefix, addsIPv4[1].Prefix, addsIPv4[2].Prefix})
	token := addsIPv4[0].SourceInstance
	require.NotZero(t, token)
	require.Len(t, addsIPv6, 1, "the IPv6 install must add the IPv6 framed route")
	require.Equal(t, framed[2].Prefix, addsIPv6[0].Prefix)
	for _, entry := range append(addsIPv4, addsIPv6...) {
		require.Equal(t, token, entry.SourceInstance)
		require.Equal(t, metrics[entry.Prefix], entry.Metric)
	}
	for _, entries := range [][]redistevents.RouteChangeEntry{adds, replayed, removed} {
		require.Len(t, entries, 4, "the actor must retain every original profile entry")
		require.ElementsMatch(t, prefixes, []netip.Prefix{entries[0].Prefix, entries[1].Prefix, entries[2].Prefix, entries[3].Prefix})
		for _, entry := range entries {
			require.Equal(t, token, entry.SourceInstance)
			require.Equal(t, metrics[entry.Prefix], entry.Metric)
		}
	}
}
