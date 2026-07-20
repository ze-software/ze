package spf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func TestOSPFABRDetection(t *testing.T) {
	backbone := types.BackboneArea
	nonBackbone := areaID(t, "0.0.0.1")
	other := areaID(t, "0.0.0.2")

	assert.True(t, IsABR([]types.AreaID{backbone, nonBackbone}))
	assert.False(t, IsABR([]types.AreaID{nonBackbone, other}), "two non-backbone areas do not make an ABR in v1")
	assert.False(t, IsABR([]types.AreaID{backbone}), "backbone only is not an ABR")
}

func TestOSPFConfiguredAreaRangeCoverage(t *testing.T) {
	// A configured /16 range suppresses every more-specific component summary it COVERS
	// (RFC 2328 sec 12.4.3), not only an exact-match prefix. Regression: the old exact-match
	// form left an accepted 10.0.5.0/24 summary un-suppressed under a 10.0.0.0/16 range.
	ranges := []AreaRange{{Prefix: netip.MustParsePrefix("10.0.0.0/16")}}
	if !isConfiguredAreaRange(netip.MustParsePrefix("10.0.5.0/24"), ranges) {
		t.Fatal("a /16 range did not cover a more-specific /24 component summary")
	}
	if !isConfiguredAreaRange(netip.MustParsePrefix("10.0.0.0/16"), ranges) {
		t.Fatal("exact-match prefix not recognized as in-range")
	}
	if isConfiguredAreaRange(netip.MustParsePrefix("11.0.0.0/24"), ranges) {
		t.Fatal("an out-of-range prefix was wrongly suppressed")
	}
	if isConfiguredAreaRange(netip.MustParsePrefix("10.0.0.0/8"), ranges) {
		t.Fatal("a less-specific /8 must NOT be covered by a /16 range")
	}
}

// RFC requirement: RFC2328-16.2-1 positive -- a router that is not an ABR computes inter-area routes from the summary-LSAs of the area it is attached to (ComputeInterAreaWith, interarea.go:118-158).
// RFC requirement: RFC2328-16.2-2 positive -- a summary-LSA that is not MaxAge, not self-originated, and whose composed cost is below LSInfinity is used: cost IAC = distance to the advertising ABR (10) + the LSA metric (7) (ComputeInterAreaWith, interarea.go:134-157).
func TestOSPFInterAreaRoute(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	abr := testRID(t, "2.2.2.2")
	area := types.BackboneArea
	nh := netip.MustParseAddr("10.0.0.2")
	res := resultWithRouter(area, root, abr, 10, nh, 0)
	src := testSource(t, area, summaryNetworkLSA(t, "10.20.0.0", "2.2.2.2", 7))

	routes, _ := ComputeInterArea(InterAreaInput{Source: src, Root: root, Areas: []types.AreaID{area}, Results: map[types.AreaID]*Result{area: res}, MaxPaths: 8})
	require.Len(t, routes, 1)
	assert.Equal(t, RouteInterArea, routes[0].Type)
	assert.Equal(t, netip.MustParsePrefix("10.20.0.0/24"), routes[0].Prefix)
	assert.Equal(t, uint64(17), routes[0].Metric)
	require.Len(t, routes[0].NextHops, 1)
	assert.Equal(t, nh, routes[0].NextHops[0].Addr)
}

func TestOSPFInterAreaPreference(t *testing.T) {
	pfx := netip.MustParsePrefix("10.30.0.0/24")
	nh := []NextHop{{Addr: netip.MustParseAddr("192.0.2.1")}}
	selected := selectBestRoutes([]RouteEntry{
		{AreaID: areaID(t, "0.0.0.1"), Prefix: pfx, Metric: 1, Type: RouteInterArea, Origin: testRID(t, "2.2.2.2"), NextHops: nh},
		{AreaID: areaID(t, "0.0.0.2"), Prefix: pfx, Metric: 100, Type: RouteIntraArea, Origin: testRID(t, "3.3.3.3"), NextHops: nh},
	}, 8)
	require.Len(t, selected, 1)
	assert.Equal(t, RouteIntraArea, selected[0].Type)
	assert.Equal(t, uint64(100), selected[0].Metric)
}

// RFC requirement: RFC2328-16.2-1 negative -- an ABR REFUSES a non-backbone summary-LSA even when it is the cheaper path: only the backbone summary contributes, so the installed cost is the backbone one (ComputeInterAreaWith abr gate, interarea.go:119, 128-130).
func TestOSPFABRBackboneOnlyAcceptance(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	backbone := types.BackboneArea
	area1 := areaID(t, "0.0.0.1")
	abr0 := testRID(t, "2.2.2.2")
	abr1 := testRID(t, "3.3.3.3")
	pfxLSID := "10.40.0.0"
	results := map[types.AreaID]*Result{
		backbone: resultWithRouter(backbone, root, abr0, 10, netip.MustParseAddr("10.0.0.2"), 0),
		area1:    resultWithRouter(area1, root, abr1, 1, netip.MustParseAddr("10.1.0.3"), 0),
	}
	src := testSource(t, backbone, summaryNetworkLSA(t, pfxLSID, "2.2.2.2", 20))
	src.Install(area1, summaryNetworkLSA(t, pfxLSID, "3.3.3.3", 1))

	routes, _ := ComputeInterArea(InterAreaInput{Source: src, Root: root, Areas: []types.AreaID{backbone, area1}, Results: results, MaxPaths: 8})
	require.Len(t, routes, 1)
	assert.Equal(t, uint64(30), routes[0].Metric, "ABR must ignore non-backbone summaries")
	assert.Equal(t, abr0, routes[0].Origin)
}

func TestOSPFType4RouteToASBR(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	abr := testRID(t, "2.2.2.2")
	asbr := testRID(t, "4.4.4.4")
	area := types.BackboneArea
	res := resultWithRouter(area, root, abr, 8, netip.MustParseAddr("10.0.0.2"), 0)
	src := testSource(t, area, summaryASBRLSA(t, "4.4.4.4", "2.2.2.2", 9))

	routes, border := ComputeInterArea(InterAreaInput{Source: src, Root: root, Areas: []types.AreaID{area}, Results: map[types.AreaID]*Result{area: res}, MaxPaths: 8})
	assert.Empty(t, routes, "Type 4 builds an ASBR route for external resolution, not a forwarding prefix")
	require.Len(t, border, 1)
	assert.Equal(t, BorderRouterASBR, border[0].Kind)
	assert.Equal(t, asbr, border[0].RouterID)
	assert.Equal(t, uint64(17), border[0].Metric)
}

func TestOSPFBorderRouterSnapshot(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	abr := testRID(t, "2.2.2.2")
	asbr := testRID(t, "3.3.3.3")
	area := types.BackboneArea
	res := resultWithRouter(area, root, abr, 4, netip.MustParseAddr("10.0.0.2"), packet.RouterFlagB)
	res.Graph.Routers[asbr] = &RouterVertex{ID: asbr, Flags: packet.RouterFlagE}
	res.Nodes[routerVertex(asbr)] = &NodeResult{ID: routerVertex(asbr), Metric: 9, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.3")}}}

	_, border := ComputeInterArea(InterAreaInput{Root: root, Areas: []types.AreaID{area}, Results: map[types.AreaID]*Result{area: res}, MaxPaths: 8})
	snap := BorderRouterSnapshot(border)
	require.Len(t, snap, 2)
	assert.Equal(t, "abr", snap[0].Kind)
	assert.Equal(t, "2.2.2.2", snap[0].RouterID)
	assert.Equal(t, "0.0.0.0", snap[0].Area)
	assert.Equal(t, uint64(4), snap[0].Metric)
	require.Len(t, snap[0].NextHops, 1)
	assert.Equal(t, "10.0.0.2", snap[0].NextHops[0].NextHop)
	assert.Equal(t, "asbr", snap[1].Kind)
	assert.Equal(t, "3.3.3.3", snap[1].RouterID)
	assert.Equal(t, "0.0.0.0", snap[1].Area)
	assert.Equal(t, uint64(9), snap[1].Metric)
	require.Len(t, snap[1].NextHops, 1)
	assert.Equal(t, "10.0.0.3", snap[1].NextHops[0].NextHop)
}

// RFC requirement: RFC2328-16.2-2 negative -- a summary-LSA whose composed cost reaches LSInfinity is skipped and installs no inter-area route (ComputeInterAreaWith metric gate, interarea.go:146-149).
func TestOSPFInterAreaLSInfinityDropped(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	abr := testRID(t, "2.2.2.2")
	area := types.BackboneArea
	// cost-to-ABR is one below LSInfinity; adding the summary metric saturates at
	// LSInfinity, which RFC 2328 treats as unreachable -- no route is installed.
	res := resultWithRouter(area, root, abr, LSInfinity-1, netip.MustParseAddr("10.0.0.2"), 0)
	src := testSource(t, area, summaryNetworkLSA(t, "10.20.0.0", "2.2.2.2", 5))

	routes, _ := ComputeInterArea(InterAreaInput{Source: src, Root: root, Areas: []types.AreaID{area}, Results: map[types.AreaID]*Result{area: res}, MaxPaths: 8})
	assert.Empty(t, routes, "inter-area cost reaching LSInfinity must not install a route")
}

func resultWithRouter(area types.AreaID, root, router types.RouterID, metric uint64, nh netip.Addr, flags uint8) *Result {
	g := NewGraph(area)
	g.Routers[root] = &RouterVertex{ID: root}
	g.Routers[router] = &RouterVertex{ID: router, Flags: flags}
	return &Result{Area: area, Root: root, Graph: g, Nodes: map[VertexID]*NodeResult{
		routerVertex(root):   {ID: routerVertex(root), Metric: 0},
		routerVertex(router): {ID: routerVertex(router), Metric: metric, NextHops: []NextHop{{Addr: nh}}},
	}}
}

func summaryNetworkLSA(t *testing.T, lsid, adv string, metric uint32) packet.LSA {
	t.Helper()
	return packet.LSA{Header: packet.LSAHeader{Options: types.OptionE, Type: types.LSTypeSummaryNetwork, LinkStateID: testLSID(t, lsid), AdvertisingRouter: testRID(t, adv), Sequence: types.InitialSequenceNumber}, Summary: &packet.SummaryLSA{NetworkMask: testIP(t, "255.255.255.0"), Metric: metric}}
}

func summaryASBRLSA(t *testing.T, asbr, adv string, metric uint32) packet.LSA {
	t.Helper()
	return packet.LSA{Header: packet.LSAHeader{Options: types.OptionE, Type: types.LSTypeSummaryASBR, LinkStateID: testLSID(t, asbr), AdvertisingRouter: testRID(t, adv), Sequence: types.InitialSequenceNumber}, Summary: &packet.SummaryLSA{Metric: metric}}
}

func areaID(t *testing.T, s string) types.AreaID {
	t.Helper()
	id, err := types.ParseAreaID(s)
	if err != nil {
		t.Fatalf("ParseAreaID(%q): %v", s, err)
	}
	return id
}
