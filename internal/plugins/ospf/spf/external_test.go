// VALIDATES: spec-ospf-10 RFC 2328 sec 16.4 AS-External route computation -- E1 cost
// (dist-to-ASBR + advertised), E2 cost (advertised only), E1 always preferred over E2
// (trap #7), forwarding-address resolution (0.0.0.0 via ASBR / non-zero re-resolved /
// unreachable skipped), externals ranked below internal.
// PREVENTS: regressions where a low-cost E2 beats a high-cost E1, a non-zero FA is
// ignored, an unreachable ASBR/FA installs a black-hole, or an external beats internal.
package spf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func externalLSA(t *testing.T, network, adv string, type2 bool, metric uint32, fwd string) packet.LSA {
	t.Helper()
	return packet.LSA{
		Header: packet.LSAHeader{
			Options:           types.OptionE,
			Type:              types.LSTypeASExternal,
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

func asbrBorder(t *testing.T, rid string, metric uint64, nh string) BorderRouterEntry {
	t.Helper()
	return BorderRouterEntry{RouterID: testRID(t, rid), Kind: BorderRouterASBR, Metric: metric, NextHops: []NextHop{{Addr: netip.MustParseAddr(nh)}}}
}

func TestOSPFExternalE1Cost(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	src := testSource(t, types.BackboneArea, externalLSA(t, "10.99.0.0", "2.2.2.2", false, 5, "0.0.0.0"))
	border := []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 10, "10.0.0.2")}

	routes := ComputeExternal(ExternalInput{Source: src, Root: root, BorderRouters: border, MaxPaths: 8})
	require.Len(t, routes, 1)
	assert.Equal(t, RouteExternalType1, routes[0].Type)
	assert.Equal(t, uint64(15), routes[0].Metric, "E1 cost = dist-to-ASBR(10) + advertised(5)")
	assert.Equal(t, netip.MustParsePrefix("10.99.0.0/24"), routes[0].Prefix)
	require.Len(t, routes[0].NextHops, 1)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), routes[0].NextHops[0].Addr)

	none := ComputeExternal(ExternalInput{Source: src, Root: root, BorderRouters: nil, MaxPaths: 8})
	assert.Empty(t, none, "unreachable ASBR -> LSA skipped")
}

// RFC requirement: RFC2328-16.2-2 negative -- an AS-external-LSA whose composed cost reaches LSInfinity is skipped by the routing calculation and installs no route (externalCandidateFrom, external.go:169-186).
func TestOSPFExternalLSInfinityDropped(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	src := testSource(t, types.BackboneArea, externalLSA(t, "10.94.0.0", "2.2.2.2", false, 5, "0.0.0.0"))
	// cost-to-ASBR one below LSInfinity; E1 adds the advertised metric and saturates
	// at LSInfinity (unreachable) -- no route installed (boundary, no wrap).
	border := []BorderRouterEntry{asbrBorder(t, "2.2.2.2", LSInfinity-1, "10.0.0.2")}
	routes := ComputeExternal(ExternalInput{Source: src, Root: root, BorderRouters: border, MaxPaths: 8})
	assert.Empty(t, routes, "E1 composed cost reaching LSInfinity installs no route")
}

func TestOSPFExternalE2Cost(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	src := testSource(t, types.BackboneArea, externalLSA(t, "10.98.0.0", "2.2.2.2", true, 7, "0.0.0.0"))
	border := []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 10, "10.0.0.2")}

	routes := ComputeExternal(ExternalInput{Source: src, Root: root, BorderRouters: border, MaxPaths: 8})
	require.Len(t, routes, 1)
	assert.Equal(t, RouteExternalType2, routes[0].Type)
	assert.Equal(t, uint64(7), routes[0].Metric, "E2 cost = advertised metric only, NOT + dist-to-ASBR")
}

// RFC requirement: RFC2328-16.4-1 negative -- the cheaper candidate is REJECTED when it is the lower-preference path type: a Type-2 external at cost 1 loses to a Type-1 external at cost 110, so metric never overrides the E1-over-E2 rule (betterExternal, external.go:234-251).
func TestOSPFExternalE1PreferredOverE2(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	src := testSource(t, types.BackboneArea,
		externalLSA(t, "10.97.0.0", "2.2.2.2", false, 100, "0.0.0.0"), // high-cost E1
		externalLSA(t, "10.97.0.0", "3.3.3.3", true, 1, "0.0.0.0"),    // low-cost E2
	)
	border := []BorderRouterEntry{
		asbrBorder(t, "2.2.2.2", 10, "10.0.0.2"),
		asbrBorder(t, "3.3.3.3", 1, "10.0.0.3"),
	}

	out := ComputeExternal(ExternalInput{Source: src, Root: root, BorderRouters: border, MaxPaths: 8})
	require.Len(t, out, 1)
	assert.Equal(t, RouteExternalType1, out[0].Type, "trap #7: E1 always beats E2 regardless of metric")
	assert.Equal(t, uint64(110), out[0].Metric, "the surviving E1 cost = 10 + 100")
}

func TestOSPFExternalE1ECMPMerge(t *testing.T) {
	// Two equal-total-cost E1 LSAs for the same prefix via different ASBRs must ECMP-merge.
	// Forwarding distance is an E2-only tie-break (RFC 2328 sec 16.4 step (d)); keying E1 on it
	// wrongly split these into separate preference and dropped one next-hop.
	root := testRID(t, "1.1.1.1")
	src := testSource(t, types.BackboneArea,
		externalLSA(t, "10.96.0.0", "2.2.2.2", false, 5, "0.0.0.0"), // E1, advertised 5
		externalLSA(t, "10.96.0.0", "3.3.3.3", false, 3, "0.0.0.0"), // E1, advertised 3
	)
	border := []BorderRouterEntry{
		asbrBorder(t, "2.2.2.2", 10, "10.0.0.2"), // dist 10 -> E1 total 15, fwdDist 10
		asbrBorder(t, "3.3.3.3", 12, "10.0.0.3"), // dist 12 -> E1 total 15, fwdDist 12
	}
	out := ComputeExternal(ExternalInput{Source: src, Root: root, BorderRouters: border, MaxPaths: 8})
	require.Len(t, out, 1)
	assert.Equal(t, RouteExternalType1, out[0].Type)
	assert.Equal(t, uint64(15), out[0].Metric)
	require.Len(t, out[0].NextHops, 2, "equal-cost E1 paths via different ASBRs must ECMP-merge despite different fwdDist")
}

func TestOSPFExternalForwardingAddress(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	src := testSource(t, types.BackboneArea, externalLSA(t, "10.96.0.0", "2.2.2.2", true, 5, "192.168.0.5"))
	border := []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 10, "10.0.0.2")}
	routeTable := []RouteEntry{{Prefix: netip.MustParsePrefix("192.168.0.0/24"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}}}}

	routes := ComputeExternal(ExternalInput{Source: src, Root: root, BorderRouters: border, Routes: routeTable, MaxPaths: 8})
	require.Len(t, routes, 1)
	require.Len(t, routes[0].NextHops, 1)
	assert.Equal(t, netip.MustParseAddr("10.0.0.9"), routes[0].NextHops[0].Addr, "non-zero FA -> next-hop toward the FA, not the ASBR")
	assert.Equal(t, uint64(5), routes[0].Metric, "E2 cost = advertised metric")

	skipped := ComputeExternal(ExternalInput{Source: src, Root: root, BorderRouters: border, Routes: nil, MaxPaths: 8})
	assert.Empty(t, skipped, "unreachable non-zero FA -> LSA skipped")
}

func TestOSPFExternalForwardingAddressDefaultOnly(t *testing.T) {
	// RFC 2328 sec 16.4: a non-zero forwarding address resolved ONLY by the default route
	// is treated as unreachable -- the LSA is skipped, not installed via the default (which
	// would make every external with an unreachable FA appear reachable).
	root := testRID(t, "1.1.1.1")
	src := testSource(t, types.BackboneArea, externalLSA(t, "10.96.0.0", "2.2.2.2", true, 5, "192.168.0.5"))
	border := []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 10, "10.0.0.2")}
	defaultOnly := []RouteEntry{{Prefix: netip.MustParsePrefix("0.0.0.0/0"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}}}}

	routes := ComputeExternal(ExternalInput{Source: src, Root: root, BorderRouters: border, Routes: defaultOnly, MaxPaths: 8})
	assert.Empty(t, routes, "FA covered only by the default route -> LSA skipped")
}

func TestOSPFExternalBelowInternal(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	src := testSource(t, types.BackboneArea, externalLSA(t, "10.95.0.0", "2.2.2.2", false, 1, "0.0.0.0"))
	border := []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 1, "10.0.0.2")}

	external := ComputeExternal(ExternalInput{Source: src, Root: root, BorderRouters: border, MaxPaths: 8})
	require.Len(t, external, 1)
	internal := RouteEntry{AreaID: types.BackboneArea, Prefix: netip.MustParsePrefix("10.95.0.0/24"), Metric: 1000, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.5")}}}
	selected := selectBestRoutes(append([]RouteEntry{internal}, external...), 8)
	require.Len(t, selected, 1)
	assert.Equal(t, RouteIntraArea, selected[0].Type, "internal route beats external even at higher metric")
}
