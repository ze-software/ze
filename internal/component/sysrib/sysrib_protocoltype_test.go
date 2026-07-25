package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// TestBGPProtocolTypeFromPath checks the replay-path protocol-type derivation.
//
// VALIDATES: bgpProtocolTypeFromPath reports eBGP/iBGP from the path's IsEBGP
// flag (matching the live event-bus ProtocolType produced by the BGP RIB) for
// ANY admin distance, and Unspecified for non-BGP sources.
// PREVENTS: an operator bgp/admin-distance override (eBGP distance != 20 or
// iBGP != 200) silently demoting a BGP route to BGPProtocolUnspecified at
// replay, which dropped sysrib's per-type admin-distance override and made the
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
			if got := bgpProtocolTypeFromPath(tt.path); got != tt.want {
				t.Fatalf("bgpProtocolTypeFromPath(%+v) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestSysRIBReplayClassifiesOverriddenAdminDistance exercises the full replay
// path (changeToBatch -> bgpProtocolTypeFromPath -> processEvent ->
// effectivePriority), the end-to-end behavior the unit test above isolates.
//
// VALIDATES: a startup-replayed eBGP best whose admin distance was overridden
// away from the default 20 is still classified eBGP, so a sysrib-level "ebgp"
// admin-distance override applies to it.
// PREVENTS: the regression where the replay path derived the class from the
// (operator-overridable) AdminDistance: a distance of 40 was neither 20 nor 200,
// so the route fell back to the generic "bgp" type and missed the per-type
// override, disagreeing with the live event-bus classification.
func TestSysRIBReplayClassifiesOverriddenAdminDistance(t *testing.T) {
	redistevents.ResetForTest()
	bgpID := redistevents.RegisterProtocol("bgp")

	s := newSysRIB()
	s.adminDist = map[string]int{"ebgp": 30, "ibgp": 200}

	pfx := netip.MustParsePrefix("10.0.0.0/24")
	batch := changeToBatch(locrib.Change{
		Family: family.IPv4Unicast,
		Prefix: pfx,
		Kind:   locrib.ChangeAdd,
		Best: locrib.Path{
			Source:        bgpID,
			IsEBGP:        true,
			AdminDistance: 40, // operator override, not the default eBGP 20
			NextHop:       netip.MustParseAddr("192.168.1.1"),
		},
	})
	require.NotNil(t, batch)
	s.processEvent(batch)

	key := prefixKey{family: family.IPv4Unicast, prefix: pfx}
	s.mu.RLock()
	route := s.routes[key]["bgp"]
	s.mu.RUnlock()

	require.NotNil(t, route)
	assert.Equal(t, 30, route.priority,
		"overridden-distance eBGP replay must still receive the ebgp admin-distance override")
}
