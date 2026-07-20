// VALIDATES: RFC 4577 sec 4.2.6 -- a route learned from a Type 4 (ASBR Summary) LSA is
// never turned into an OSPF route table entry, so it can never reach the OSPF -> BGP
// redistribution export (which consumes spf.RouteDelta entries only).
// PREVENTS: a regression where the inter-area computation turns an ASBR summary into a
// forwarding prefix, which would leak router reachability into BGP as a route.
package spf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC4577-4.2.6-5 positive -- the Type 3 (network) summary path IS the
// redistributable one: ComputeInterArea turns a Type 3 Summary-LSA into a RouteEntry
// (interarea.go:157), and the OSPF redistribution source exports exactly those route-table
// entries to BGP (emitDelta/addEntry, redistribute/source.go:99-137). This pins the
// contrast case for the MUST NOT below: prefix summaries are exported, ASBR summaries are not.
func TestRFC4577Type3SummaryBecomesRedistributableRoute(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	abr := testRID(t, "2.2.2.2")
	area := types.BackboneArea
	nh := netip.MustParseAddr("10.0.0.2")
	res := resultWithRouter(area, root, abr, 10, nh, 0)
	src := testSource(t, area, summaryNetworkLSA(t, "10.20.0.0", "2.2.2.2", 7))

	routes, border := ComputeInterArea(InterAreaInput{
		Source: src, Root: root, Areas: []types.AreaID{area},
		Results: map[types.AreaID]*Result{area: res}, MaxPaths: 8,
	})
	require.Len(t, routes, 1, "a Type 3 network summary yields one inter-area route")
	assert.Equal(t, netip.MustParsePrefix("10.20.0.0/24"), routes[0].Prefix)
	assert.Equal(t, RouteInterArea, routes[0].Type)
	assert.Empty(t, border, "a Type 3 network summary is not a border-router record")
}

// RFC requirement: RFC4577-4.2.6-5 negative -- a route received in a Type 4 (ASBR Summary)
// LSA is NOT redistributed to BGP: ComputeInterArea diverts every IsASBR summary into a
// BorderRouterEntry and `continue`s before the RouteEntry append (interarea.go:150-152,
// decoded from LSTypeSummaryASBR at interarea.go:187), so no route-table entry exists for
// it. The OSPF redistribution source emits ONLY spf.RouteDelta route entries
// (emitDelta/addEntry, redistribute/source.go:99-137) and never reads border routers, so
// the ASBR reachability cannot become a BGP route.
func TestRFC4577Type4SummaryNotRedistributed(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	abr := testRID(t, "2.2.2.2")
	asbr := testRID(t, "4.4.4.4")
	area := types.BackboneArea
	res := resultWithRouter(area, root, abr, 8, netip.MustParseAddr("10.0.0.2"), 0)
	src := testSource(t, area, summaryASBRLSA(t, "4.4.4.4", "2.2.2.2", 9))

	routes, border := ComputeInterArea(InterAreaInput{
		Source: src, Root: root, Areas: []types.AreaID{area},
		Results: map[types.AreaID]*Result{area: res}, MaxPaths: 8,
	})
	assert.Empty(t, routes, "a Type 4 ASBR summary must not produce a redistributable route")
	require.Len(t, border, 1, "it stays ASBR reachability for external resolution")
	assert.Equal(t, BorderRouterASBR, border[0].Kind)
	assert.Equal(t, asbr, border[0].RouterID)
}
