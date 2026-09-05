// VALIDATES: RFC 3101 Section 2.4 and Section 2.5 install gates on the OSPFv3 address
// family -- an NSSA border router refuses a received Type-7 default whose P-bit is clear,
// and refuses every Type-7 default while it suppresses summary-route import.
// PREVENTS: the shared gate being proven on OSPFv2 alone. It filters on the scope-aware
// types.LSType.NSSA() helper, so it is meant to match the 0x2007 NSSA-LSA as well as the
// 0x0007 one, and nothing until now read the OSPFv3 side of that claim.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v6NSSADefaultSource builds a one-LSA source holding a peer-originated OSPFv3 Type-7
// default (LS Type 0x2007, zero-length prefix). propagate sets the prefix P-bit, which is
// where RFC 5340 Appendix A.4.8 carries what OSPFv2 keeps in the LSA header Options.
func v6NSSADefaultSource(t *testing.T, advertising types.RouterID, propagate bool) fakeV6Source {
	t.Helper()
	prefix, ok := netipToV6Prefix(netip.PrefixFrom(netip.IPv6Unspecified(), 0), 0)
	require.True(t, ok, "the default destination encodes as a zero-length OSPFv3 prefix")
	if propagate {
		prefix.Options = ospfv3types.OptPrefixP
	}
	lsa := ospfv3packet.LSA{
		Header: ospfv3packet.LSAHeader{
			Age: 1, Type: ospfv3types.LSTypeNSSA,
			LinkStateID:       ospfv3types.LinkStateID{0, 0, 0, 1},
			AdvertisingRouter: ospfv3types.RouterID(advertising),
			Sequence:          ospfv3types.InitialSequenceNumber,
		},
		External: &ospfv3packet.ExternalLSA{Metric: 10, Prefix: prefix},
	}
	raw := make([]byte, (&lsa).EncodedLen())
	(&lsa).WriteTo(raw, 0)
	hdr := v6LSAHeaderToNeutral(lsa.Header)
	return fakeV6Source{
		headers: []packet.LSAHeader{hdr},
		lsas:    map[types.LSAKey]packet.LSA{hdr.Key(): {Header: hdr, RawBytes: raw}},
	}
}

// TestOSPFv3NSSABorderRouterDefaultPBit drives the OSPFv3 external computation over a
// peer's Type-7 default in each of the three reachable states: installable, refused for a
// clear P-bit, and refused because this router suppresses summary import. The permissive
// case is the control -- without it, a gate that dropped every OSPFv3 NSSA-LSA would pass
// the two refusals.
func TestOSPFv3NSSABorderRouterDefaultPBit(t *testing.T) {
	asbr := types.RouterID{2, 2, 2, 2}
	nssa := types.AreaID{0, 0, 0, 1}
	nextHop := netip.MustParseAddr("fe80::2")
	reach := []ospfspf.BorderRouterEntry{{
		RouterID: asbr, Kind: ospfspf.BorderRouterASBR, Metric: 10,
		NextHops: []ospfspf.NextHop{{Addr: nextHop}},
	}}
	defaultPrefix := netip.PrefixFrom(netip.IPv6Unspecified(), 0)

	compute := func(src fakeV6Source, borderRouter, noSummary bool) []ospfspf.RouteEntry {
		in := ospfspf.ExternalInput{
			Source: src, Root: types.RouterID{1, 1, 1, 1},
			NSSAAreas: []types.AreaID{nssa}, BorderRouters: reach,
			NSSABorderRouter: borderRouter,
		}
		if noSummary {
			in.NSSAPolicies = map[types.AreaID]ospfspf.AreaSummaryPolicy{
				nssa: {Type: ospfspf.AreaTypeNSSA, NoSummary: true},
			}
		}
		return v6Strategy{}.ComputeExternal(in)
	}

	t.Run("P-bit set", func(t *testing.T) {
		// RFC requirement: RFC3101-2.4-4 positive -- an NSSA border router
		// installs a received Type-7 default whose P-bit is set, and on OSPFv3
		// that bit rides in the prefix options (RFC 5340 App A.4.8).
		routes := compute(v6NSSADefaultSource(t, asbr, true), true, false)
		require.Len(t, routes, 1)
		assert.Equal(t, defaultPrefix, routes[0].Prefix)
	})

	t.Run("P-bit clear", func(t *testing.T) {
		// RFC requirement: RFC3101-2.4-4 negative -- an NSSA border router
		// refuses a received OSPFv3 Type-7 default whose P-bit is clear.
		assert.Empty(t, compute(v6NSSADefaultSource(t, asbr, false), true, false))
	})

	t.Run("summary import suppressed", func(t *testing.T) {
		// RFC requirement: RFC3101-2.5-1 positive -- an NSSA border router
		// that suppresses Type-3 summary import ignores an OSPFv3 Type-7
		// default even when its P-bit is set.
		assert.Empty(t, compute(v6NSSADefaultSource(t, asbr, true), true, true))
	})
}

// TestOSPFv3NSSANonBorderRouterInstallsPClearDefault proves the two refusals above are
// scoped to a border router. An NSSA internal router has no Type-3 summary default to
// protect and no translation duty, so RFC 3101 leaves it free to follow the P-clear
// default its border router originated for exactly that purpose.
func TestOSPFv3NSSANonBorderRouterInstallsPClearDefault(t *testing.T) {
	asbr := types.RouterID{2, 2, 2, 2}
	nssa := types.AreaID{0, 0, 0, 1}
	nextHop := netip.MustParseAddr("fe80::2")

	// RFC requirement: RFC3101-2.4-4 negative -- the install rule binds an
	// NSSA border router, so a router that is not one takes the P-clear default.
	// RFC requirement: RFC3101-2.5-1 negative -- the suppressed-summary rule
	// likewise binds an NSSA border router only.
	routes := v6Strategy{}.ComputeExternal(ospfspf.ExternalInput{
		Source: v6NSSADefaultSource(t, asbr, false), Root: types.RouterID{1, 1, 1, 1},
		NSSAAreas: []types.AreaID{nssa},
		BorderRouters: []ospfspf.BorderRouterEntry{{
			RouterID: asbr, Kind: ospfspf.BorderRouterASBR, Metric: 10,
			NextHops: []ospfspf.NextHop{{Addr: nextHop}},
		}},
		NSSAPolicies: map[types.AreaID]ospfspf.AreaSummaryPolicy{
			nssa: {Type: ospfspf.AreaTypeNSSA, NoSummary: true},
		},
	})
	require.Len(t, routes, 1, "an NSSA internal router installs the border router's P-clear default")
	assert.Equal(t, netip.PrefixFrom(netip.IPv6Unspecified(), 0), routes[0].Prefix)
}
