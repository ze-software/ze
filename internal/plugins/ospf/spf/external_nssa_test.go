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

func type7LSA(t *testing.T, network, adv string, type2 bool, metric uint32, fwd string, propagate bool) packet.LSA {
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
			NetworkMask:    testIP(t, "255.255.255.0"),
			ExternalType2:  type2,
			Metric:         metric,
			ForwardingAddr: testIP(t, fwd),
		},
	}
}

func TestOSPFNSSAPreference(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	nssa := areaID(t, "0.0.0.5")
	fa := "192.168.0.5"

	db := ospflsdb.New(nil)
	require.True(t, db.Install(types.BackboneArea, externalLSA(t, "10.40.0.0", "2.2.2.2", true, 1, fa)), "Type 5")
	require.True(t, db.Install(nssa, type7LSA(t, "10.40.0.0", "3.3.3.3", true, 100, fa, true)), "Type 7 P=1")
	require.True(t, db.Install(nssa, type7LSA(t, "10.40.0.0", "4.4.4.4", true, 1, fa, false)), "Type 7 P=0")

	routeTable := []RouteEntry{{Prefix: netip.MustParsePrefix("192.168.0.0/24"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}}}}

	routes := ComputeExternal(ExternalInput{Source: db, Root: root, Routes: routeTable, NSSAAreas: []types.AreaID{nssa}, MaxPaths: 8})
	require.Len(t, routes, 1, "one winning external route for the shared prefix")
	assert.Equal(t, netip.MustParsePrefix("10.40.0.0/24"), routes[0].Prefix)
	assert.Equal(t, testRID(t, "3.3.3.3"), routes[0].Origin, "the Type 7 P=1 wins over the Type 5 and the lower-cost Type 7 P=0 (RFC 3101 sec 2.5)")
}
