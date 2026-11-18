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
// the observer. Its IPCP address stays IPv4; this is no IPv6CP or native-I/O proof.
// MUTATION: dropping the actor's captured framedRoutes loses the IPv6 add,
// replay and withdrawal while the same real PPP peer still answers Echo.
func TestActorRouteCapturesOriginalIPv6FramedMetadata(t *testing.T) {
	captureBirthSIDDraw(t)
	bus := &recordingBus{}
	observer := newSubscriberRouteObserver(nil, bus)
	prefix := netip.MustParsePrefix("2001:db8::/32")
	admission := newActorRouteAdmission(t, observer, actorRouteInput{
		username: "ipv6-framed", address: netip.MustParseAddr("192.0.2.72"),
		framedRoutes: []FramedRoute{{Prefix: prefix, Metric: 5}},
	})
	actorRouteEcho(t, admission.wire)
	observer.reemitAll(93)
	admission.reactor.handleLCPDown(admission.wire.beginRestart(t))

	adds := actorRouteEntries(t, bus.events(), redistevents.ActionAdd, 0)
	replayed := actorRouteEntries(t, bus.events(), redistevents.ActionAdd, 93)
	removed := actorRouteEntries(t, bus.events(), redistevents.ActionRemove, 0)
	require.Len(t, adds, 2, "actor must capture the original IPv6 framed route beside its IPCP address")
	token := adds[0].SourceInstance
	require.NotZero(t, token)
	address := netip.MustParsePrefix("192.0.2.72/32")
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
// supplies two IPv4 and one IPv6 route; all survive add/replay/withdrawal with
// their original metrics and token, beside its useful IPv4 IPCP address.
// MUTATION: truncating the actor's captured framedRoutes to its first entry
// loses list members even when the direct observer's framed-route tests pass.
func TestActorRouteCapturesEveryOriginalFramedRoute(t *testing.T) {
	captureBirthSIDDraw(t)
	bus := &recordingBus{}
	observer := newSubscriberRouteObserver(nil, bus)
	framed := []FramedRoute{
		{Prefix: netip.MustParsePrefix("10.0.0.0/8"), Metric: 1},
		{Prefix: netip.MustParsePrefix("172.16.0.0/12"), Metric: 2},
		{Prefix: netip.MustParsePrefix("2001:db8::/32"), Metric: 5},
	}
	admission := newActorRouteAdmission(t, observer, actorRouteInput{
		username: "all-framed", address: netip.MustParseAddr("192.0.2.70"), framedRoutes: framed,
	})
	actorRouteEcho(t, admission.wire)
	observer.reemitAll(94)
	admission.reactor.handleLCPDown(admission.wire.beginRestart(t))

	adds := actorRouteEntries(t, bus.events(), redistevents.ActionAdd, 0)
	replayed := actorRouteEntries(t, bus.events(), redistevents.ActionAdd, 94)
	removed := actorRouteEntries(t, bus.events(), redistevents.ActionRemove, 0)
	require.Len(t, adds, 4)
	token := adds[0].SourceInstance
	require.NotZero(t, token)
	prefixes := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.70/32"), framed[0].Prefix, framed[1].Prefix, framed[2].Prefix,
	}
	metrics := map[netip.Prefix]uint32{prefixes[0]: 0, framed[0].Prefix: 1, framed[1].Prefix: 2, framed[2].Prefix: 5}
	for _, entries := range [][]redistevents.RouteChangeEntry{adds, replayed, removed} {
		require.Len(t, entries, 4, "the actor must retain every original profile entry")
		require.ElementsMatch(t, prefixes, []netip.Prefix{entries[0].Prefix, entries[1].Prefix, entries[2].Prefix, entries[3].Prefix})
		for _, entry := range entries {
			require.Equal(t, token, entry.SourceInstance)
			require.Equal(t, metrics[entry.Prefix], entry.Metric)
		}
	}
}
