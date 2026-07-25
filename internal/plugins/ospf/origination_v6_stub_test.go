// VALIDATES: plan/learned/973-ospfv3-6-interop-coverage.md -- OSPFv3 stub-area ABR default
// origination. A v6 ABR attached to a stub area injects exactly one Inter-Area-Prefix
// default (::/0) into the stub at the configured default-cost (RFC 5340 sec 3.5 / RFC 2328
// sec 3.6), never injects an Inter-Area-Router-LSA into the stub, and (totally-stubby)
// suppresses every other inter-area prefix. PREVENTS: a v6 stub area whose internal routers
// have no way out because the ABR never originated the default -- the gap that blocked the
// ospf-v6-stub-frr interop scenario (the OSPFv2 path had this via spf.applyAreaTypePolicy;
// the OSPFv3 path did not).
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

func TestOSPFv6OriginateStubDefault(t *testing.T) {
	// An ABR attached to the backbone and a stub area injects exactly one Inter-Area-Prefix
	// default (::/0) into the stub at the configured default-cost; a normal stub keeps the
	// inter-area prefixes but NEVER receives an Inter-Area-Router-LSA (the v6 Type-4
	// equivalent), so the stub internal routers get a way out (::/0) without carrying
	// external-reachability state.
	e := newV6OriginEngine()
	root := types.RouterID{172, 30, 0, 2}
	stub := types.AreaID{0, 0, 0, 2}
	asbr := types.RouterID{10, 0, 0, 9}

	// A backbone intra-area prefix that would normally be summarized into the stub.
	bb, ok := netipToV6Prefix(netip.MustParsePrefix("2001:db8:bb::/64"), 3)
	if !ok {
		t.Fatal("netipToV6Prefix failed")
	}
	if _, ok := e.v6OriginateIntraAreaPrefix(types.BackboneArea, root, []ospfv3packet.Prefix{bb}); !ok {
		t.Fatal("seeding backbone Intra-Area-Prefix-LSA failed")
	}

	backboneRes := &ospfspf.Result{
		Area:  types.BackboneArea,
		Graph: ospfspf.NewGraph(types.BackboneArea),
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{
			{Kind: ospfspf.VertexRouter, Router: root}: {Metric: 0},
			{Kind: ospfspf.VertexRouter, Router: asbr}: {Metric: 20},
		},
	}
	// An ASBR in the backbone: without the stub policy it would be advertised into the stub as
	// an Inter-Area-Router-LSA, so its absence proves the stub policy suppressed Type 0x2004.
	backboneRes.Graph.Routers[asbr] = &ospfspf.RouterVertex{ID: asbr, Flags: packet.RouterFlagE}
	stubRes := &ospfspf.Result{
		Area:  stub,
		Graph: ospfspf.NewGraph(stub),
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{{Kind: ospfspf.VertexRouter, Router: root}: {Metric: 0}},
	}
	in := ospfspf.SummaryInput{
		Root:    root,
		Areas:   []types.AreaID{types.BackboneArea, stub},
		Results: map[types.AreaID]*ospfspf.Result{types.BackboneArea: backboneRes, stub: stubRes},
		Policies: map[types.AreaID]ospfspf.AreaSummaryPolicy{
			stub: {Type: ospfspf.AreaTypeStub, DefaultCost: 10},
		},
	}
	if out := e.v6OriginateSummaries(in); out.Changed == 0 {
		t.Fatal("v6OriginateSummaries originated nothing for a stub ABR")
	}

	// The stub default ::/0 is originated at LSID 1 (the unspecified address sorts first) with
	// the configured default-cost.
	defKey := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeInterAreaPrefix), LinkStateID: v6SummaryLSID(1), AdvertisingRouter: root}
	lsa, found := e.lsdb.LookupLSA(stub, defKey)
	if !found {
		t.Fatal("stub default Inter-Area-Prefix-LSA (::/0) not originated into the stub")
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	iap, err := decoded.DecodeInterAreaPrefix()
	if err != nil {
		t.Fatalf("DecodeInterAreaPrefix: %v", err)
	}
	if gotPfx, _ := v6PrefixToNetip(iap.Prefix, afIPv6Unicast); gotPfx != netip.MustParsePrefix("::/0") {
		t.Errorf("stub default prefix = %s, want ::/0", gotPfx)
	}
	if iap.Metric != 10 {
		t.Errorf("stub default metric = %d, want 10 (default-cost)", iap.Metric)
	}

	// A normal (non-totally-stubby) stub still receives the backbone inter-area prefix (LSID 2).
	bbKey := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeInterAreaPrefix), LinkStateID: v6SummaryLSID(2), AdvertisingRouter: root}
	if _, found := e.lsdb.LookupLSA(stub, bbKey); !found {
		t.Error("backbone inter-area prefix missing from the (non-totally-stubby) stub")
	}

	// No Inter-Area-Router-LSA (Type 0x2004) is ever injected into a stub area.
	for lsid := uint32(1); lsid <= 3; lsid++ {
		rtrKey := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeInterAreaRouter), LinkStateID: v6SummaryLSID(lsid), AdvertisingRouter: root}
		if _, found := e.lsdb.LookupLSA(stub, rtrKey); found {
			t.Errorf("Inter-Area-Router-LSA (LSID %d) wrongly injected into a stub area", lsid)
		}
	}
}

func TestOSPFv6OriginateTotallyStubbyOnlyDefault(t *testing.T) {
	// A totally-stubby area (no-summary) receives ONLY the ::/0 default: every other
	// inter-area prefix is suppressed (RFC 2328 sec 3.6), so the stub carries a single LSA.
	e := newV6OriginEngine()
	root := types.RouterID{172, 30, 0, 2}
	stub := types.AreaID{0, 0, 0, 2}

	bb, _ := netipToV6Prefix(netip.MustParsePrefix("2001:db8:bb::/64"), 3)
	if _, ok := e.v6OriginateIntraAreaPrefix(types.BackboneArea, root, []ospfv3packet.Prefix{bb}); !ok {
		t.Fatal("seeding backbone Intra-Area-Prefix-LSA failed")
	}

	backboneRes := &ospfspf.Result{
		Area:  types.BackboneArea,
		Graph: ospfspf.NewGraph(types.BackboneArea),
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{{Kind: ospfspf.VertexRouter, Router: root}: {Metric: 0}},
	}
	stubRes := &ospfspf.Result{
		Area:  stub,
		Graph: ospfspf.NewGraph(stub),
		Nodes: map[ospfspf.VertexID]*ospfspf.NodeResult{{Kind: ospfspf.VertexRouter, Router: root}: {Metric: 0}},
	}
	in := ospfspf.SummaryInput{
		Root:    root,
		Areas:   []types.AreaID{types.BackboneArea, stub},
		Results: map[types.AreaID]*ospfspf.Result{types.BackboneArea: backboneRes, stub: stubRes},
		Policies: map[types.AreaID]ospfspf.AreaSummaryPolicy{
			stub: {Type: ospfspf.AreaTypeStub, NoSummary: true, DefaultCost: 5},
		},
	}
	if out := e.v6OriginateSummaries(in); out.Changed == 0 {
		t.Fatal("v6OriginateSummaries originated nothing for a totally-stubby ABR")
	}

	// Only ::/0 (LSID 1) exists; the backbone prefix that LSID 2 would carry is suppressed.
	defKey := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeInterAreaPrefix), LinkStateID: v6SummaryLSID(1), AdvertisingRouter: root}
	if _, found := e.lsdb.LookupLSA(stub, defKey); !found {
		t.Fatal("totally-stubby default ::/0 not originated")
	}
	bbKey := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeInterAreaPrefix), LinkStateID: v6SummaryLSID(2), AdvertisingRouter: root}
	if _, found := e.lsdb.LookupLSA(stub, bbKey); found {
		t.Error("totally-stubby stub wrongly received an inter-area prefix beyond the default")
	}
}
