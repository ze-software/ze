// VALIDATES: RFC 3101 Section 2.5 scope checks and external route comparison.
// PREVENTS: source preference overriding E1/E2 metrics, or NSSA LSAs using
// another area's ASBR or forwarding-address reachability.
package spf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func type7LSA(t *testing.T, network, mask, adv string, metric uint32, propagate bool) packet.LSA {
	t.Helper()
	var opts types.Options
	if propagate {
		opts = opts.Set(types.OptionNP)
	}
	return packet.LSA{
		Header: packet.LSAHeader{
			Options:           opts,
			Type:              types.LSTypeNSSA,
			LinkStateID:       testLSID(t, network),
			AdvertisingRouter: testRID(t, adv),
			Sequence:          types.InitialSequenceNumber,
		},
		External: &packet.ExternalLSA{
			NetworkMask:    testIP(t, mask),
			ExternalType2:  true,
			Metric:         metric,
			ForwardingAddr: testIP(t, "192.168.0.5"),
		},
	}
}

func TestOSPFNSSAPreference(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	nssa := areaID(t, "0.0.0.5")
	fa := "192.168.0.5"
	for _, tc := range []struct {
		name   string
		metric uint32
		want   string
	}{
		{"lower metric wins", 100, "2.2.2.2"},
		{"equivalent LSA prefers P bit", 1, "3.3.3.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := ospflsdb.New(nil)
			require.True(t, db.Install(types.BackboneArea, externalLSA(t, "10.40.0.0", "2.2.2.2", true, 1, fa)))
			require.True(t, db.Install(nssa, type7LSA(t, "10.40.0.0", "255.255.255.0", "3.3.3.3", tc.metric, true)))
			require.True(t, db.Install(nssa, type7LSA(t, "10.40.0.0", "255.255.255.0", "4.4.4.4", 1, false)))
			routesToFA := []RouteEntry{
				{AreaID: types.BackboneArea, Prefix: netip.MustParsePrefix("192.168.0.0/24"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}}},
				{AreaID: nssa, Prefix: netip.MustParsePrefix("192.168.0.0/24"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.8")}}},
			}
			border := []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 3, "10.0.0.9"), nssaASBR(t, nssa, "3.3.3.3"), nssaASBR(t, nssa, "4.4.4.4")}
			routes := ComputeExternal(ExternalInput{Source: db, Root: root, BorderRouters: border, Routes: routesToFA, NSSAAreas: []types.AreaID{nssa}})
			require.Len(t, routes, 1)
			assert.Equal(t, netip.MustParsePrefix("10.40.0.0/24"), routes[0].Prefix)
			assert.Equal(t, testRID(t, tc.want), routes[0].Origin)
			assert.Equal(t, uint64(1), routes[0].Metric)
		})
	}
}

func nssaASBR(t *testing.T, area types.AreaID, router string) BorderRouterEntry {
	t.Helper()
	entry := asbrBorder(t, router, 3, "10.0.0.9")
	entry.AreaID = area
	return entry
}

// TestOSPFNSSABorderRouterDefaultPBit verifies the P-bit install gate for a
// Type-7 default received by an NSSA border router.
func TestOSPFNSSABorderRouterDefaultPBit(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	nssa := areaID(t, "0.0.0.5")
	routeTable := []RouteEntry{{
		AreaID:   nssa,
		Prefix:   netip.MustParsePrefix("192.168.0.0/24"),
		Metric:   3,
		Type:     RouteIntraArea,
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}},
	}}

	t.Run("P-bit set", func(t *testing.T) {
		db := ospflsdb.New(nil)
		require.True(t, db.Install(nssa, type7LSA(t, "0.0.0.0", "0.0.0.0", "3.3.3.3", 10, true)))

		// RFC requirement: RFC3101-2.4-4 positive -- an NSSA border router
		// can install a received Type-7 default whose P-bit is set.
		// RFC requirement: RFC3101-2.5-1 negative -- a regular NSSA does
		// not suppress Type-7 defaults when summary import is enabled.
		routes := ComputeExternal(ExternalInput{
			Source: db, Root: root, Routes: routeTable, BorderRouters: []BorderRouterEntry{nssaASBR(t, nssa, "3.3.3.3")},
			NSSAAreas: []types.AreaID{nssa}, NSSABorderRouter: true, MaxPaths: 8,
		})
		require.Len(t, routes, 1)
		assert.Equal(t, netip.MustParsePrefix("0.0.0.0/0"), routes[0].Prefix)
	})

	t.Run("summary import suppressed", func(t *testing.T) {
		db := ospflsdb.New(nil)
		require.True(t, db.Install(nssa, type7LSA(t, "0.0.0.0", "0.0.0.0", "3.3.3.3", 10, true)))

		// RFC requirement: RFC3101-2.5-1 positive -- an NSSA border router
		// ignores Type-7 defaults when summary import is suppressed.
		routes := ComputeExternal(ExternalInput{
			Source: db, Root: root, Routes: routeTable, BorderRouters: []BorderRouterEntry{nssaASBR(t, nssa, "3.3.3.3")},
			NSSAAreas: []types.AreaID{nssa}, NSSABorderRouter: true,
			NSSAPolicies: map[types.AreaID]AreaSummaryPolicy{
				nssa: {Type: types.AreaTypeNSSA, NoSummary: true},
			},
			MaxPaths: 8,
		})
		assert.Empty(t, routes)
	})

	t.Run("P-bit clear", func(t *testing.T) {
		db := ospflsdb.New(nil)
		require.True(t, db.Install(nssa, type7LSA(t, "0.0.0.0", "0.0.0.0", "3.3.3.3", 10, false)))

		// RFC requirement: RFC3101-2.4-4 negative -- an NSSA border router
		// does not install a received Type-7 default whose P-bit is clear.
		routes := ComputeExternal(ExternalInput{
			Source: db, Root: root, Routes: routeTable, BorderRouters: []BorderRouterEntry{nssaASBR(t, nssa, "3.3.3.3")},
			NSSAAreas: []types.AreaID{nssa}, NSSABorderRouter: true, MaxPaths: 8,
		})
		assert.Empty(t, routes)
	})
}

// TestOSPFNSSANonBorderRouterInstallsPClearDefault is the permissive direction of the two
// gates above: both are scoped by ExternalInput.NSSABorderRouter, so a router that is not
// an NSSA border router installs the P-clear default its border router originated. Without
// this case a gate that dropped every Type-7 default would pass every refusal assertion.
func TestOSPFNSSANonBorderRouterInstallsPClearDefault(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	nssa := areaID(t, "0.0.0.5")
	routeTable := []RouteEntry{{
		AreaID:   nssa,
		Prefix:   netip.MustParsePrefix("192.168.0.0/24"),
		Metric:   3,
		Type:     RouteIntraArea,
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}},
	}}

	db := ospflsdb.New(nil)
	require.True(t, db.Install(nssa, type7LSA(t, "0.0.0.0", "0.0.0.0", "3.3.3.3", 10, false)))

	// RFC requirement: RFC3101-2.4-4 negative -- the install rule binds an NSSA
	// border router, so a router that is not one takes the P-clear default.
	// RFC requirement: RFC3101-2.5-1 negative -- the suppressed-summary rule
	// likewise binds an NSSA border router only.
	routes := ComputeExternal(ExternalInput{
		Source: db, Root: root, Routes: routeTable, BorderRouters: []BorderRouterEntry{nssaASBR(t, nssa, "3.3.3.3")},
		NSSAAreas: []types.AreaID{nssa}, NSSABorderRouter: false,
		NSSAPolicies: map[types.AreaID]AreaSummaryPolicy{
			nssa: {Type: types.AreaTypeNSSA, NoSummary: true},
		},
		MaxPaths: 8,
	})
	require.Len(t, routes, 1, "an NSSA internal router installs the border router's P-clear default")
	assert.Equal(t, netip.MustParsePrefix("0.0.0.0/0"), routes[0].Prefix)
}

// TestNSSASourcePreferencePreservesOtherExits verifies that replacing one
// equivalent Type-5 LSA with its preferred Type-7 leaves another equal-cost
// forwarding address in ECMP.
func TestNSSASourcePreferencePreservesOtherExits(t *testing.T) {
	nssa := areaID(t, "0.0.0.5")
	db := ospflsdb.New(nil)
	for _, external := range []packet.LSA{
		externalLSA(t, "10.40.0.0", "2.2.2.2", true, 1, "192.168.0.5"),
		externalLSA(t, "10.40.0.0", "5.5.5.5", true, 1, "192.168.0.6"),
	} {
		require.True(t, db.Install(types.BackboneArea, external))
	}
	require.True(t, db.Install(nssa, type7LSA(t, "10.40.0.0", "255.255.255.0", "3.3.3.3", 1, true)))
	routes := ComputeExternal(ExternalInput{
		Source: db, Root: testRID(t, "1.1.1.1"), NSSAAreas: []types.AreaID{nssa},
		BorderRouters: []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 3, "10.0.0.9"), asbrBorder(t, "5.5.5.5", 3, "10.0.0.6"), nssaASBR(t, nssa, "3.3.3.3")},
		Routes: []RouteEntry{
			{AreaID: types.BackboneArea, Prefix: netip.MustParsePrefix("192.168.0.5/32"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}}},
			{AreaID: types.BackboneArea, Prefix: netip.MustParsePrefix("192.168.0.6/32"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.6")}}},
			{AreaID: nssa, Prefix: netip.MustParsePrefix("192.168.0.5/32"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.8")}}},
		},
	})
	require.Len(t, routes, 1)
	assert.ElementsMatch(t, []NextHop{
		{Addr: netip.MustParseAddr("10.0.0.6")},
		{Addr: netip.MustParseAddr("10.0.0.8")},
	}, routes[0].NextHops)
}

// TestNSSASourcePreferenceCannotOverrideE1 verifies Section 2.5(6)(b) before
// the final equivalent-LSA preference.
func TestNSSASourcePreferenceCannotOverrideE1(t *testing.T) {
	nssa := areaID(t, "0.0.0.5")
	db := ospflsdb.New(nil)
	require.True(t, db.Install(types.BackboneArea, externalLSA(t, "10.40.0.0", "2.2.2.2", false, 100, "192.168.0.5")))
	require.True(t, db.Install(nssa, type7LSA(t, "10.40.0.0", "255.255.255.0", "3.3.3.3", 1, true)))
	routes := ComputeExternal(ExternalInput{
		Source: db, Root: testRID(t, "1.1.1.1"), NSSAAreas: []types.AreaID{nssa},
		BorderRouters: []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 3, "10.0.0.9"), nssaASBR(t, nssa, "3.3.3.3")},
		Routes: []RouteEntry{
			{AreaID: types.BackboneArea, Prefix: netip.MustParsePrefix("192.168.0.5/32"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}}},
			{AreaID: nssa, Prefix: netip.MustParsePrefix("192.168.0.5/32"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.8")}}},
		},
	})
	require.Len(t, routes, 1)
	assert.Equal(t, RouteExternalType1, routes[0].Type)
	assert.Equal(t, uint64(103), routes[0].Metric)
	assert.Equal(t, testRID(t, "2.2.2.2"), routes[0].Origin)
}

// RFC requirement: RFC3101-2.5-3 positive -- resolving an eligible Type-5 forwarding address preserves internal path preference and equal-cost next hops.
// MUTATION: resolveForwarding selects the cheapest candidate regardless of path type or drops another area's equal-cost path.
func TestExternalForwardingPreservesInternalPreferenceAndECMP(t *testing.T) {
	source := testSource(t, types.BackboneArea, externalLSA(t, "10.40.0.0", "2.2.2.2", false, 5, "192.168.0.5"))
	forwarding := netip.MustParsePrefix("192.168.0.0/24")
	routes := ComputeExternal(ExternalInput{
		Source: source, Root: testRID(t, "1.1.1.1"),
		BorderRouters: []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 1, "10.0.0.2")},
		Routes: []RouteEntry{
			{AreaID: types.BackboneArea, Prefix: forwarding, Metric: 1, Type: RouteInterArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.8")}}},
			{AreaID: types.BackboneArea, Prefix: forwarding, Metric: 20, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}}},
			{AreaID: areaID(t, "0.0.0.2"), Prefix: forwarding, Metric: 20, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.10")}}},
		},
	})
	require.Len(t, routes, 1)
	assert.Equal(t, uint64(25), routes[0].Metric)
	assert.Equal(t, []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}, {Addr: netip.MustParseAddr("10.0.0.10")}}, routes[0].NextHops)
}
