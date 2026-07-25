// VALIDATES: spec-ospf-af-unify -- OSPFv3 ABR inter-area summary origination. An ABR
// condenses each attached area's intra-area networks + ASBRs into the OTHER areas as
// Inter-Area-Prefix-LSAs (RFC 5340 App A.4.5) and Inter-Area-Router-LSAs (App A.4.6) through
// the same OriginateSelf / FlushStaleSelfLSAs seams as the self LSAs, and withdraws them
// when the router stops being an ABR (RFC 2328 sec 3.3). PREVENTS: a v6 ABR forming
// adjacencies in two areas but never advertising one area's prefixes into the other, so the
// two areas never gain inter-area reachability.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv6OriginateSummaries(t *testing.T) {
	e := newV6OriginEngine()
	root := types.RouterID{172, 30, 0, 2}
	area1 := types.AreaID{0, 0, 0, 1}
	asbr := types.RouterID{10, 0, 0, 9}

	// The ABR's own intra-area prefix in area 1 (its Intra-Area-Prefix-LSA references its
	// own Router-LSA, prefix metric 5). Seeding via the self-origination seam puts a real
	// Intra-Area-Prefix-LSA in the area-1 store for v6SummaryNetworks to read back.
	p, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:1::/64"), 5)
	if !ok {
		t.Fatal("netipToV6Prefix failed")
	}
	if _, ok := e.v6OriginateIntraAreaPrefix(area1, root, []ospfv3packet.Prefix{p}); !ok {
		t.Fatal("seeding area-1 Intra-Area-Prefix-LSA failed")
	}

	// SPF results: the root is reached at cost 0 in both areas; area 1 also contains an
	// ASBR (E-bit set in the graph) reached at cost 20.
	area1Res := &ospfspf.Result{
		Area:  area1,
		Graph: ospfspf.NewGraph(area1),
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{
			{Kind: ospfspf.VertexRouter, Router: root}: {Metric: 0},
			{Kind: ospfspf.VertexRouter, Router: asbr}: {Metric: 20},
		},
	}
	area1Res.Graph.Routers[asbr] = &ospfspf.RouterVertex{ID: asbr, Flags: packet.RouterFlagE}
	backboneRes := &ospfspf.Result{
		Area:  types.BackboneArea,
		Graph: ospfspf.NewGraph(types.BackboneArea),
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{
			{Kind: ospfspf.VertexRouter, Router: root}: {Metric: 0},
		},
	}

	in := ospfspf.SummaryInput{
		Root:    root,
		Areas:   []types.AreaID{types.BackboneArea, area1},
		Results: map[types.AreaID]*ospfspf.Result{types.BackboneArea: backboneRes, area1: area1Res},
	}

	out := e.v6OriginateSummaries(in)
	if out.Changed == 0 {
		t.Fatal("v6OriginateSummaries originated nothing for an ABR")
	}

	// The area-1 prefix is summarized INTO the backbone (Inter-Area-Prefix-LSA, LSID 1).
	prefixKey := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeInterAreaPrefix), LinkStateID: v6SummaryLSID(1), AdvertisingRouter: root}
	lsa, found := e.lsdb.LookupLSA(types.BackboneArea, prefixKey)
	if !found {
		t.Fatal("Inter-Area-Prefix-LSA not originated into the backbone")
	}
	if !ospfv3packet.VerifyLSAChecksum(lsa.RawBytes) {
		t.Fatal("Inter-Area-Prefix-LSA Fletcher checksum invalid")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	iap, err := decoded.DecodeInterAreaPrefix()
	if err != nil {
		t.Fatalf("DecodeInterAreaPrefix: %v", err)
	}
	gotPfx, ok := v6PrefixToNetip(iap.Prefix, afIPv6Unicast)
	if !ok || gotPfx != netip.MustParsePrefix("2001:db8:1::/64") {
		t.Errorf("summarized prefix = %s, want 2001:db8:1::/64", gotPfx)
	}
	if iap.Metric != 5 {
		t.Errorf("summary metric = %d, want 5 (cost 0 to self + prefix metric 5)", iap.Metric)
	}

	// The area-1 ASBR is summarized into the backbone (Inter-Area-Router-LSA, LSID 1).
	routerKey := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeInterAreaRouter), LinkStateID: v6SummaryLSID(1), AdvertisingRouter: root}
	rlsa, found := e.lsdb.LookupLSA(types.BackboneArea, routerKey)
	if !found {
		t.Fatal("Inter-Area-Router-LSA not originated into the backbone")
	}
	rdec, err := ospfv3packet.DecodeLSA(rlsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA(router): %v", err)
	}
	iar, err := rdec.DecodeInterAreaRouter()
	if err != nil {
		t.Fatalf("DecodeInterAreaRouter: %v", err)
	}
	if iar.DestinationRouter != ospfv3types.RouterID(asbr) {
		t.Errorf("summarized ASBR = %v, want %v", iar.DestinationRouter, asbr)
	}
	if iar.Metric != 20 {
		t.Errorf("ASBR summary metric = %d, want 20", iar.Metric)
	}

	// The area-1 prefix must NOT be re-advertised back into its own source area.
	if _, found := e.lsdb.LookupLSA(area1, prefixKey); found {
		t.Error("Inter-Area-Prefix-LSA wrongly originated back into its source area")
	}

	// Idempotent: an unchanged ABR topology re-originates nothing.
	if out2 := e.v6OriginateSummaries(in); out2.Changed != 0 {
		t.Errorf("second v6OriginateSummaries changed %d, want 0 (idempotent)", out2.Changed)
	}

	// When the router is no longer an ABR (active in a single area), it withdraws every
	// inter-area summary it originated (RFC 2328 sec 3.3).
	notABR := ospfspf.SummaryInput{
		Root:    root,
		Areas:   []types.AreaID{types.BackboneArea},
		Results: map[types.AreaID]*ospfspf.Result{types.BackboneArea: backboneRes},
	}
	if out3 := e.v6OriginateSummaries(notABR); out3.Changed == 0 {
		t.Fatal("non-ABR pass withdrew nothing")
	}
	withdrawn, _ := e.lsdb.LookupLSA(types.BackboneArea, prefixKey)
	if !withdrawn.Header.Age.IsMaxAge() {
		t.Errorf("Inter-Area-Prefix-LSA age = %v, want MaxAge (withdrawn when no longer ABR)", withdrawn.Header.Age)
	}
}

func TestOSPFv6OriginateSummariesRange(t *testing.T) {
	// An ABR with a configured area range aggregates two intra-area /64s into a single
	// Inter-Area-Prefix-LSA for the /48 (RFC 2328 sec 12.4.3), instead of one summary each.
	e := newV6OriginEngine()
	root := types.RouterID{172, 30, 0, 2}
	area1 := types.AreaID{0, 0, 0, 1}

	p1, _ := netipToV6Prefix(netip.MustParsePrefix("2001:db8:1:1::/64"), 5)
	p2, _ := netipToV6Prefix(netip.MustParsePrefix("2001:db8:1:2::/64"), 8)
	if _, ok := e.v6OriginateIntraAreaPrefix(area1, root, []ospfv3packet.Prefix{p1, p2}); !ok {
		t.Fatal("seeding area-1 Intra-Area-Prefix-LSA failed")
	}

	area1Res := &ospfspf.Result{
		Area:  area1,
		Graph: ospfspf.NewGraph(area1),
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{{Kind: ospfspf.VertexRouter, Router: root}: {Metric: 0}},
	}
	backboneRes := &ospfspf.Result{
		Area:  types.BackboneArea,
		Graph: ospfspf.NewGraph(types.BackboneArea),
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{{Kind: ospfspf.VertexRouter, Router: root}: {Metric: 0}},
	}
	in := ospfspf.SummaryInput{
		Root:    root,
		Areas:   []types.AreaID{types.BackboneArea, area1},
		Results: map[types.AreaID]*ospfspf.Result{types.BackboneArea: backboneRes, area1: area1Res},
		Ranges: map[types.AreaID][]ospfspf.AreaRange{
			area1: {{Prefix: netip.MustParsePrefix("2001:db8:1::/48"), Advertise: true}},
		},
	}
	if out := e.v6OriginateSummaries(in); out.Changed == 0 {
		t.Fatal("v6OriginateSummaries originated nothing")
	}

	// The two /64s collapse into one aggregated /48 summary with the max component metric (8).
	aggKey := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeInterAreaPrefix), LinkStateID: v6SummaryLSID(1), AdvertisingRouter: root}
	lsa, found := e.lsdb.LookupLSA(types.BackboneArea, aggKey)
	if !found {
		t.Fatal("aggregated Inter-Area-Prefix-LSA not originated")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	iap, err := decoded.DecodeInterAreaPrefix()
	if err != nil {
		t.Fatalf("DecodeInterAreaPrefix: %v", err)
	}
	if gotPfx, _ := v6PrefixToNetip(iap.Prefix, afIPv6Unicast); gotPfx != netip.MustParsePrefix("2001:db8:1::/48") {
		t.Errorf("aggregated prefix = %s, want 2001:db8:1::/48", gotPfx)
	}
	if iap.Metric != 8 {
		t.Errorf("aggregated metric = %d, want 8 (max component)", iap.Metric)
	}
	// Only the aggregate exists: no per-/64 summary (LSID 2) was originated.
	secondKey := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeInterAreaPrefix), LinkStateID: v6SummaryLSID(2), AdvertisingRouter: root}
	if _, found := e.lsdb.LookupLSA(types.BackboneArea, secondKey); found {
		t.Error("a second Inter-Area-Prefix-LSA exists: the /64s were not aggregated")
	}
}
