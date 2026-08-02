// VALIDATES: spec-ospf-11 RFC 3101 sec 2.5 -- when the same external prefix is reachable
// via a Type 7 (P=1), a Type 5, and a Type 7 (P=0), the external route computation picks
// the Type 7 P=1 (source preference is the primary key, ahead of the sec 16.4 cost), so
// one winning route is produced.
// PREVENTS: regressions where a lower-cost Type 5 or P=0 Type 7 beats a P=1 Type 7, or
// NSSA Type 7 LSAs are ignored by the external computation.
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

	db := ospflsdb.New(nil)
	require.True(t, db.Install(types.BackboneArea, externalLSA(t, "10.40.0.0", "2.2.2.2", true, 1, fa)), "Type 5")
	require.True(t, db.Install(nssa, type7LSA(t, "10.40.0.0", "255.255.255.0", "3.3.3.3", 100, true)), "Type 7 P=1")
	require.True(t, db.Install(nssa, type7LSA(t, "10.40.0.0", "255.255.255.0", "4.4.4.4", 1, false)), "Type 7 P=0")

	routeTable := []RouteEntry{{Prefix: netip.MustParsePrefix("192.168.0.0/24"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}}}}

	routes := ComputeExternal(ExternalInput{Source: db, Root: root, Routes: routeTable, NSSAAreas: []types.AreaID{nssa}, MaxPaths: 8})
	require.Len(t, routes, 1, "one winning external route for the shared prefix")
	assert.Equal(t, netip.MustParsePrefix("10.40.0.0/24"), routes[0].Prefix)
	assert.Equal(t, testRID(t, "3.3.3.3"), routes[0].Origin, "the Type 7 P=1 wins over the Type 5 and the lower-cost Type 7 P=0 (RFC 3101 sec 2.5)")
}

// TestOSPFNSSABorderRouterDefaultPBit verifies the P-bit install gate for a
// Type-7 default received by an NSSA border router.
func TestOSPFNSSABorderRouterDefaultPBit(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	nssa := areaID(t, "0.0.0.5")
	routeTable := []RouteEntry{{
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
			Source: db, Root: root, Routes: routeTable,
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
			Source: db, Root: root, Routes: routeTable,
			NSSAAreas: []types.AreaID{nssa}, NSSABorderRouter: true,
			NSSAPolicies: map[types.AreaID]AreaSummaryPolicy{
				nssa: {Type: AreaTypeNSSA, NoSummary: true},
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
			Source: db, Root: root, Routes: routeTable,
			NSSAAreas: []types.AreaID{nssa}, NSSABorderRouter: true, MaxPaths: 8,
		})
		assert.Empty(t, routes)
	})
}
