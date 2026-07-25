// VALIDATES: RFC 2328 Section 12.4.3 / 3.3 ABR Summary-LSA origination -- Type 3
// network and Type 4 ASBR summaries, area-range aggregation/suppression, LS-ID
// collision handling, transit-LAN summarization, MaxAge withdrawal, and the
// Router-LSA B-bit.
// PREVENTS: regressions where an ABR stops originating, mis-aggregates a range,
// omits a broadcast LAN subnet, or fails to flush stale summaries.
package spf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFABRBitSet(t *testing.T) {
	router := testRID(t, "1.1.1.1")
	backbone := types.BackboneArea
	area1 := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	db.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{
			{Name: "bb", AreaID: backbone, Address: testIP(t, "10.0.0.1"), NetworkMask: testIP(t, "255.255.255.0"), NetworkType: ospflsdb.NetworkBroadcast, State: ospflsdb.InterfaceStateDR},
			{Name: "a1", AreaID: area1, Address: testIP(t, "10.1.0.1"), NetworkMask: testIP(t, "255.255.255.0"), NetworkType: ospflsdb.NetworkBroadcast, State: ospflsdb.InterfaceStateDR},
		}
	})
	db.OriginateFromTopology(router, false)
	lsa, ok := db.LookupLSA(backbone, types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(router), AdvertisingRouter: router})
	require.True(t, ok)
	body, err := lsa.DecodeRouter()
	require.NoError(t, err)
	assert.NotZero(t, body.Flags&packet.RouterFlagB)

	db2 := ospflsdb.New(nil)
	db2.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{
			{Name: "a1", AreaID: area1, Address: testIP(t, "10.1.0.1"), NetworkMask: testIP(t, "255.255.255.0"), NetworkType: ospflsdb.NetworkBroadcast, State: ospflsdb.InterfaceStateDR},
			{Name: "a2", AreaID: areaID(t, "0.0.0.2"), Address: testIP(t, "10.2.0.1"), NetworkMask: testIP(t, "255.255.255.0"), NetworkType: ospflsdb.NetworkBroadcast, State: ospflsdb.InterfaceStateDR},
		}
	})
	db2.OriginateFromTopology(router, false)
	lsa, ok = db2.LookupLSA(area1, types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(router), AdvertisingRouter: router})
	require.True(t, ok)
	body, err = lsa.DecodeRouter()
	require.NoError(t, err)
	assert.Zero(t, body.Flags&packet.RouterFlagB, "no real backbone means no ABR bit")

	db3 := ospflsdb.New(nil)
	db3.SetTopology(func() []ospflsdb.InterfaceInfo {
		return []ospflsdb.InterfaceInfo{
			{Name: "bb", AreaID: backbone, Address: testIP(t, "10.0.0.1"), NetworkMask: testIP(t, "255.255.255.0"), NetworkType: ospflsdb.NetworkBroadcast, State: ospflsdb.InterfaceStateDown},
			{Name: "a1", AreaID: area1, Address: testIP(t, "10.1.0.1"), NetworkMask: testIP(t, "255.255.255.0"), NetworkType: ospflsdb.NetworkBroadcast, State: ospflsdb.InterfaceStateDR},
		}
	})
	db3.OriginateFromTopology(router, false)
	lsa, ok = db3.LookupLSA(area1, types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(router), AdvertisingRouter: router})
	require.True(t, ok)
	body, err = lsa.DecodeRouter()
	require.NoError(t, err)
	assert.Zero(t, body.Flags&packet.RouterFlagB, "down backbone does not make an active ABR")
}

func TestOSPFType3Origination(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	backbone := types.BackboneArea
	area1 := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	results := map[types.AreaID]*Result{area1: resultWithStub(area1, root, "10.10.0.0", 7)}

	res := OriginateSummaries(SummaryInput{Sink: db, Root: root, Areas: []types.AreaID{backbone, area1}, Options: map[types.AreaID]types.Options{backbone: types.OptionE}, Results: results})
	assert.Equal(t, 1, res.Counts[backbone])
	lsa, ok := db.LookupLSA(backbone, types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: testLSID(t, "10.10.0.0"), AdvertisingRouter: root})
	require.True(t, ok)
	assert.True(t, lsa.Header.Options.Has(types.OptionE))
	body, err := lsa.DecodeSummary()
	require.NoError(t, err)
	assert.Equal(t, uint32(7), body.Metric)
	assert.Equal(t, testIP(t, "255.255.255.0"), body.NetworkMask)
}

func TestOSPFTransitNetworkSummary(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	backbone := types.BackboneArea
	area1 := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	// area1 has a broadcast LAN 10.20.0.0/24 (DR interface 10.20.0.1) attached to
	// the ABR; its own subnet must be summarized into the backbone as a Type 3.
	results := map[types.AreaID]*Result{area1: resultWithTransit(area1, root, "10.20.0.1", "255.255.255.0", 3)}

	OriginateSummaries(SummaryInput{Sink: db, Root: root, Areas: []types.AreaID{backbone, area1}, Results: results})
	lsa, ok := db.LookupLSA(backbone, types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: testLSID(t, "10.20.0.0"), AdvertisingRouter: root})
	require.True(t, ok, "transit LAN subnet not summarized into backbone")
	body, err := lsa.DecodeSummary()
	require.NoError(t, err)
	assert.Equal(t, uint32(3), body.Metric)
	assert.Equal(t, testIP(t, "255.255.255.0"), body.NetworkMask)
}

func TestOSPFSummaryFlood(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	backbone := types.BackboneArea
	area1 := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	results := map[types.AreaID]*Result{area1: resultWithStub(area1, root, "10.15.0.0", 9)}

	OriginateSummaries(SummaryInput{Sink: db, Root: root, Areas: []types.AreaID{backbone, area1}, Results: results})
	headers := db.Summary(backbone)
	require.NotEmpty(t, headers)
	lsa, ok := db.LookupLSA(backbone, types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: testLSID(t, "10.15.0.0"), AdvertisingRouter: root})
	require.True(t, ok)
	assert.Equal(t, types.LSTypeSummaryNetwork, lsa.Header.Type)
}

func TestOSPFType4Origination(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	asbr := testRID(t, "4.4.4.4")
	backbone := types.BackboneArea
	area1 := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	results := map[types.AreaID]*Result{area1: resultWithASBR(area1, root, asbr, 12)}

	OriginateSummaries(SummaryInput{Sink: db, Root: root, Areas: []types.AreaID{backbone, area1}, Results: results})
	lsa, ok := db.LookupLSA(backbone, types.LSAKey{Type: types.LSTypeSummaryASBR, LinkStateID: types.LinkStateID(asbr), AdvertisingRouter: root})
	require.True(t, ok)
	assert.Equal(t, types.LinkStateID(asbr), lsa.Header.LinkStateID, "Type 4 LS ID is the ASBR Router ID")
	body, err := lsa.DecodeSummary()
	require.NoError(t, err)
	assert.Equal(t, uint32(12), body.Metric)
	assert.Equal(t, [4]byte{}, body.NetworkMask)
	assert.Zero(t, body.TOS, "single TOS 0 per the umbrella codec")
}

func TestOSPFBackboneType4ReOrigination(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	asbr := testRID(t, "4.4.4.4")
	backbone := types.BackboneArea
	area1 := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)

	OriginateSummaries(SummaryInput{
		Sink:  db,
		Root:  root,
		Areas: []types.AreaID{backbone, area1},
		BorderRouters: []BorderRouterEntry{{
			RouterID: asbr,
			AreaID:   backbone,
			Kind:     BorderRouterASBR,
			Metric:   17,
			NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.4")}},
		}},
	})
	lsa, ok := db.LookupLSA(area1, types.LSAKey{Type: types.LSTypeSummaryASBR, LinkStateID: types.LinkStateID(asbr), AdvertisingRouter: root})
	require.True(t, ok)
	body, err := lsa.DecodeSummary()
	require.NoError(t, err)
	assert.Equal(t, uint32(17), body.Metric)
	assert.Equal(t, [4]byte{}, body.NetworkMask)
}

func TestOSPFSummaryLSIDCollision(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	backbone := types.BackboneArea
	area1 := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	res := &Result{Area: area1, Root: root, Graph: NewGraph(area1), Nodes: map[VertexID]*NodeResult{routerVertex(root): {ID: routerVertex(root), Metric: 0}}}
	res.Graph.Routers[root] = &RouterVertex{ID: root, Links: []packet.RouterLink{
		{LinkID: testLSID(t, "10.20.0.0"), LinkData: testIP(t, "255.255.255.0"), Type: packet.RouterLinkTypeStub, Metric: 5},
		{LinkID: testLSID(t, "10.20.0.0"), LinkData: testIP(t, "255.255.255.128"), Type: packet.RouterLinkTypeStub, Metric: 6},
	}}

	OriginateSummaries(SummaryInput{Sink: db, Root: root, Areas: []types.AreaID{backbone, area1}, Results: map[types.AreaID]*Result{area1: res}})
	var summaries int
	for _, h := range db.Summary(backbone) {
		if h.Type == types.LSTypeSummaryNetwork && h.AdvertisingRouter == root {
			summaries++
		}
	}
	assert.Equal(t, 2, summaries)
	// The /24 sorts before the /25 (mask byte order), keeps the natural LS ID, and
	// the /25 is bumped to 10.20.0.1. Assert each LS ID binds to the RIGHT body so a
	// swap of the two bodies would fail (not just that two LS IDs exist).
	first, ok := db.LookupLSA(backbone, types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: testLSID(t, "10.20.0.0"), AdvertisingRouter: root})
	require.True(t, ok)
	firstBody, err := first.DecodeSummary()
	require.NoError(t, err)
	assert.Equal(t, testIP(t, "255.255.255.0"), firstBody.NetworkMask)
	assert.Equal(t, uint32(5), firstBody.Metric)
	second, ok := db.LookupLSA(backbone, types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: testLSID(t, "10.20.0.1"), AdvertisingRouter: root})
	require.True(t, ok)
	secondBody, err := second.DecodeSummary()
	require.NoError(t, err)
	assert.Equal(t, testIP(t, "255.255.255.128"), secondBody.NetworkMask)
	assert.Equal(t, uint32(6), secondBody.Metric)
}

func TestOSPFAreaRangeAggregate(t *testing.T) {
	in := []summaryNetwork{{Prefix: netip.MustParsePrefix("10.30.1.0/24"), Metric: 10}, {Prefix: netip.MustParsePrefix("10.30.2.0/24"), Metric: 20}}
	out := applyAreaRanges(in, []AreaRange{{Prefix: netip.MustParsePrefix("10.30.0.0/16"), Advertise: true}})
	require.Len(t, out, 1)
	assert.Equal(t, netip.MustParsePrefix("10.30.0.0/16"), out[0].Prefix)
	assert.Equal(t, uint64(20), out[0].Metric)

	out = applyAreaRanges(in, []AreaRange{{Prefix: netip.MustParsePrefix("10.30.0.0/16"), Advertise: true, Cost: 3, HasCost: true}})
	require.Len(t, out, 1)
	assert.Equal(t, uint64(3), out[0].Metric)

	// A network outside the range passes through individually alongside the aggregate.
	withOutside := append(append([]summaryNetwork(nil), in...), summaryNetwork{Prefix: netip.MustParsePrefix("10.99.0.0/24"), Metric: 7})
	out = applyAreaRanges(withOutside, []AreaRange{{Prefix: netip.MustParsePrefix("10.30.0.0/16"), Advertise: true}})
	require.Len(t, out, 2)
	byPrefix := map[netip.Prefix]uint64{}
	for _, n := range out {
		byPrefix[n.Prefix] = n.Metric
	}
	assert.Equal(t, uint64(20), byPrefix[netip.MustParsePrefix("10.30.0.0/16")], "aggregate uses max component cost")
	assert.Equal(t, uint64(7), byPrefix[netip.MustParsePrefix("10.99.0.0/24")], "uncovered network passes through")
}

func TestOSPFAreaRangeNotAdvertise(t *testing.T) {
	in := []summaryNetwork{{Prefix: netip.MustParsePrefix("10.40.1.0/24"), Metric: 10}, {Prefix: netip.MustParsePrefix("10.41.1.0/24"), Metric: 20}}
	out := applyAreaRanges(in, []AreaRange{{Prefix: netip.MustParsePrefix("10.40.0.0/16"), Advertise: false}})
	require.Len(t, out, 1)
	assert.Equal(t, netip.MustParsePrefix("10.41.1.0/24"), out[0].Prefix)
}

func TestApplyAreaRanges(t *testing.T) {
	// The exported AF-neutral aggregator also handles IPv6 (the OSPFv3 Inter-Area-Prefix
	// summary path): two /64s collapse into a configured /48 with the max component cost.
	v6in := []RangeInput{
		{Prefix: netip.MustParsePrefix("2001:db8:1:1::/64"), Metric: 10},
		{Prefix: netip.MustParsePrefix("2001:db8:1:2::/64"), Metric: 25},
	}
	out := ApplyAreaRanges(v6in, []AreaRange{{Prefix: netip.MustParsePrefix("2001:db8:1::/48"), Advertise: true}})
	require.Len(t, out, 1)
	assert.Equal(t, netip.MustParsePrefix("2001:db8:1::/48"), out[0].Prefix)
	assert.Equal(t, uint64(25), out[0].Metric)

	// Cross-family: a v4 range cannot cover a v6 prefix, so both v6 networks pass through.
	out = ApplyAreaRanges(v6in, []AreaRange{{Prefix: netip.MustParsePrefix("10.0.0.0/8"), Advertise: true}})
	require.Len(t, out, 2)

	// Metric boundary: a range cost AT LSInfinity is suppressed (its components are still
	// withdrawn); one BELOW LSInfinity is advertised.
	one := []RangeInput{{Prefix: netip.MustParsePrefix("2001:db8:2::/64"), Metric: 5}}
	out = ApplyAreaRanges(one, []AreaRange{{Prefix: netip.MustParsePrefix("2001:db8:2::/48"), Advertise: true, Cost: uint32(LSInfinity), HasCost: true}})
	assert.Empty(t, out, "a range cost of LSInfinity must not be advertised")
	out = ApplyAreaRanges(one, []AreaRange{{Prefix: netip.MustParsePrefix("2001:db8:2::/48"), Advertise: true, Cost: uint32(LSInfinity) - 1, HasCost: true}})
	require.Len(t, out, 1)
	assert.Equal(t, LSInfinity-1, out[0].Metric)
}

func TestOSPFSummaryWithdraw(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	backbone := types.BackboneArea
	area1 := areaID(t, "0.0.0.1")
	now := time.Unix(0, 0)
	db := ospflsdb.New(func() time.Time { return now })
	results := map[types.AreaID]*Result{area1: resultWithStub(area1, root, "10.50.0.0", 5)}
	OriginateSummaries(SummaryInput{Sink: db, Root: root, Areas: []types.AreaID{backbone, area1}, Results: results})
	now = now.Add(10 * time.Second)
	OriginateSummaries(SummaryInput{Sink: db, Root: root, Areas: []types.AreaID{backbone, area1}, Results: map[types.AreaID]*Result{area1: resultWithRouter(area1, root, testRID(t, "2.2.2.2"), 1, netip.MustParseAddr("10.0.0.2"), 0)}})

	lsa, ok := db.LookupLSA(backbone, types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: testLSID(t, "10.50.0.0"), AdvertisingRouter: root})
	require.True(t, ok)
	assert.True(t, lsa.Header.Age.IsMaxAge())
}

func TestOSPFSummaryFlushesInactiveArea(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	backbone := types.BackboneArea
	area1 := areaID(t, "0.0.0.1")
	now := time.Unix(0, 0)
	db := ospflsdb.New(func() time.Time { return now })
	results := map[types.AreaID]*Result{area1: resultWithStub(area1, root, "10.60.0.0", 5)}
	OriginateSummaries(SummaryInput{Sink: db, Root: root, Areas: []types.AreaID{backbone, area1}, Results: results})
	now = now.Add(10 * time.Second)
	OriginateSummaries(SummaryInput{Sink: db, Root: root, Areas: []types.AreaID{backbone}, FlushAreas: []types.AreaID{area1}})

	lsa, ok := db.LookupLSA(backbone, types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: testLSID(t, "10.60.0.0"), AdvertisingRouter: root})
	require.True(t, ok)
	assert.True(t, lsa.Header.Age.IsMaxAge())
}

func resultWithStub(area types.AreaID, root types.RouterID, pfx string, metric uint16) *Result {
	g := NewGraph(area)
	g.Routers[root] = &RouterVertex{ID: root, Links: []packet.RouterLink{stubLinkFromPrefix(pfx, metric)}}
	return &Result{Area: area, Root: root, Graph: g, Nodes: map[VertexID]*NodeResult{routerVertex(root): {ID: routerVertex(root), Metric: 0}}}
}

func resultWithASBR(area types.AreaID, root, asbr types.RouterID, metric uint64) *Result {
	g := NewGraph(area)
	g.Routers[root] = &RouterVertex{ID: root}
	g.Routers[asbr] = &RouterVertex{ID: asbr, Flags: packet.RouterFlagE}
	return &Result{Area: area, Root: root, Graph: g, Nodes: map[VertexID]*NodeResult{routerVertex(root): {ID: routerVertex(root), Metric: 0}, routerVertex(asbr): {ID: routerVertex(asbr), Metric: metric}}}
}

func stubLinkFromPrefix(prefix string, metric uint16) packet.RouterLink {
	pfx := netip.MustParsePrefix(prefix + "/24")
	return packet.RouterLink{LinkID: types.LinkStateID(pfx.Addr().As4()), LinkData: maskFromBits(pfx.Bits()), Type: packet.RouterLinkTypeStub, Metric: types.Metric(metric)}
}

// resultWithTransit builds an SPF result whose area contains a transit (broadcast
// LAN) network vertex directly attached to the root, used to prove an ABR
// summarizes its own LAN subnet (RFC 2328 Section 16.1 step (4) + Section 12.4.3).
func resultWithTransit(area types.AreaID, root types.RouterID, dr, mask string, metric uint64) *Result {
	g := NewGraph(area)
	g.Routers[root] = &RouterVertex{ID: root}
	nvID := types.LinkStateID(netip.MustParseAddr(dr).As4())
	g.Networks[nvID] = &NetworkVertex{ID: nvID, AdvertisingDR: root, NetworkMask: netip.MustParseAddr(mask).As4()}
	return &Result{Area: area, Root: root, Graph: g, Nodes: map[VertexID]*NodeResult{
		routerVertex(root):  {ID: routerVertex(root), Metric: 0},
		networkVertex(nvID): {ID: networkVertex(nvID), Metric: metric},
	}}
}
