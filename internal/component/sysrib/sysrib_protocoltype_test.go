package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// TestBGPProtocolTypeFromPath checks the selected-path protocol classification.
//
// VALIDATES: the selected adapter reports eBGP/iBGP from the path's IsEBGP
// flag (matching the live event-bus ProtocolType produced by the BGP RIB) for
// ANY admin distance, and Unspecified for non-BGP sources.
// PREVENTS: an operator rib/distance override (eBGP distance != 20 or
// iBGP != 200) silently demoting a BGP route to BGPProtocolUnspecified at
// replay, which dropped sysrib's per-type distance override and made the
// startup-replayed best differ from the live-path classification.
func TestBGPProtocolTypeFromPath(t *testing.T) {
	redistevents.ResetForTest()
	bgpID := redistevents.RegisterProtocol("bgp")
	connID := redistevents.RegisterProtocol("connected")

	tests := []struct {
		name string
		path locrib.Path
		want routeaction.ProtocolType
	}{
		{"ebgp default distance", locrib.Path{Source: bgpID, IsEBGP: true, AdminDistance: 20}, routeaction.ProtocolEBGP},
		{"ibgp default distance", locrib.Path{Source: bgpID, IsEBGP: false, AdminDistance: 200}, routeaction.ProtocolIBGP},
		// The bug: operator-overridden distances must still classify correctly.
		{"ebgp overridden distance", locrib.Path{Source: bgpID, IsEBGP: true, AdminDistance: 40}, routeaction.ProtocolEBGP},
		{"ibgp overridden distance", locrib.Path{Source: bgpID, IsEBGP: false, AdminDistance: 150}, routeaction.ProtocolIBGP},
		{"non-bgp source", locrib.Path{Source: connID, AdminDistance: 0}, routeaction.ProtocolUnspecified},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newCandidateForwardingFixture(t, fixtureRouteOverride)
			path := tt.path
			path.IsBGP = path.Source == bgpID
			path.DistanceOverride, path.HasDistanceOverride = path.AdminDistance, true
			prefix := netip.MustParsePrefix("198.18.252.0/24")
			fixture.rib.Insert(family.IPv4Unicast, prefix, path)
			selected := selectedFromAuthority(t, fixture.rib, prefix, false)
			value, ok := selected.(routeViewSelectedAdd)
			if !ok {
				t.Fatalf("selected adapter returned %T", selected)
			}
			want := tt.want.String()
			if tt.want == routeaction.ProtocolUnspecified {
				want = redistevents.ProtocolName(tt.path.Source)
			}
			if value.route.protocolType != want || value.route.priority != int(tt.path.AdminDistance) {
				t.Fatalf("selected classification/rank = %s/%d, want %s/%d", value.route.protocolType, value.route.priority, want, tt.path.AdminDistance)
			}
		})
	}
}

// TestSysRIBReplayClassifiesOverriddenAdminDistance exercises the full replay
// path (authority selection -> selected adapter -> current forwarding/replay), the
// end-to-end behavior the unit test above isolates.
//
// VALIDATES: a startup-replayed eBGP best whose admin distance is away from the
// default 20 is still classified eBGP, and keeps the distance the Loc-RIB
// ranked it at, because the Loc-RIB owns distance and re-ranks on every reload.
// PREVENTS: the regression where the replay path derived the class from the
// AdminDistance: a distance of 40 was neither 20 nor 200, so the route fell back
// to the generic "bgp" type; and sysrib replacing the Loc-RIB's rank with a
// table lookup, which would erase a static route's own override.
func TestSysRIBReplayClassifiesOverriddenAdminDistance(t *testing.T) {
	redistevents.ResetForTest()
	bgpID := redistevents.RegisterProtocol("bgp")

	s := newCandidateForwardingFixture(t, fixtureRouteOverride)
	s.acknowledgeDistances(map[string]int{"ebgp": 30, "ibgp": 200})

	pfx := netip.MustParsePrefix("10.0.0.0/24")
	s.rib.Insert(family.IPv4Unicast, pfx, locrib.Path{
		Source: bgpID, IsBGP: true, IsEBGP: true,
		DistanceOverride: 40, HasDistanceOverride: true,
		NextHop: netip.MustParseAddr("192.168.1.1"),
	})
	s.forwardSelected(map[prefixKey]struct{}{{family: family.IPv4Unicast, prefix: pfx}: {}})
	replayed := s.replayBatches(&replay.Request{ReplayID: replay.Broadcast})
	require.Len(t, replayed, 1)
	require.Len(t, replayed[0].Changes, 1)
	assert.Equal(t, "bgp", replayed[0].Changes[0].Protocol)
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), replayed[0].Changes[0].NextHop)

	key := prefixKey{family: family.IPv4Unicast, prefix: pfx}
	s.mu.RLock()
	route := s.routes[key]["bgp"]
	s.mu.RUnlock()

	require.NotNil(t, route)
	assert.Equal(t, "ebgp", route.protocolType,
		"a replayed eBGP best must be classified ebgp whatever its distance")
	assert.Equal(t, 40, route.priority,
		"a Loc-RIB change keeps the distance the Loc-RIB ranked it at")
}
