// VALIDATES: RFC 2328 Sections 3.6 and 12.4.3 at the summary origination seam -- an
// area border router injects one Type 3 default into a stub area and none into a
// normal area, and originates summary-LSAs into an attached area for the other
// areas' intra-area routes, condensed to the configured address ranges.
// PREVENTS: a stub area losing its only route out, a normal area receiving an
// unasked default, and a component prefix escaping its configured range.
package spf

import (
	"net/netip"
	"testing"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func abrInput(db *ospflsdb.LSDB, root types.RouterID, other types.AreaID, policy AreaSummaryPolicy, ranges []AreaRange) SummaryInput {
	backbone := types.BackboneArea
	return SummaryInput{
		Sink:     db,
		Root:     root,
		Areas:    []types.AreaID{backbone, other},
		Options:  map[types.AreaID]types.Options{other: 0, backbone: types.OptionE},
		Ranges:   map[types.AreaID][]AreaRange{backbone: ranges},
		Results:  map[types.AreaID]*Result{backbone: resultWithStub(backbone, root, "10.10.0.0", 7)},
		Policies: map[types.AreaID]AreaSummaryPolicy{other: policy},
	}
}

func type3Key(root types.RouterID, lsid types.LinkStateID) types.LSAKey {
	return types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: lsid, AdvertisingRouter: root}
}

// RFC requirement: RFC2328-3.6-1 positive -- an area border router attached to a stub area advertises a default route into it as a Type 3 summary-LSA with Link State ID 0.0.0.0, mask 0.0.0.0 and the configured default cost (applyAreaTypePolicy appends the default for AreaTypeStub, area_type.go).
func TestRFC2328StubAreaGetsDefaultSummary(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	stub := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	OriginateSummaries(abrInput(db, root, stub, AreaSummaryPolicy{Type: types.AreaTypeStub, DefaultCost: 5}, nil))
	def, ok := db.LookupLSA(stub, type3Key(root, types.LinkStateID([4]byte{})))
	if !ok {
		t.Fatal("stub area has no Type 3 default summary-LSA")
	}
	body, err := def.DecodeSummary()
	if err != nil {
		t.Fatalf("DecodeSummary: %v", err)
	}
	if body.NetworkMask != ([4]byte{}) || body.Metric != 5 {
		t.Fatalf("default summary body = %+v, want mask 0.0.0.0 metric 5", body)
	}
}

// RFC requirement: RFC2328-3.6-1 negative -- an area border router injects no default summary-LSA into a normal (non-stub) area: the default appears only where the area type asks for it (applyAreaTypePolicy returns desired unchanged for a normal area, area_type.go).
func TestRFC2328NormalAreaGetsNoDefaultSummary(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	normal := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	OriginateSummaries(abrInput(db, root, normal, AreaSummaryPolicy{Type: types.AreaTypeNormal, DefaultCost: 5}, nil))
	if def, ok := db.LookupLSA(normal, type3Key(root, types.LinkStateID([4]byte{}))); ok {
		t.Fatalf("normal area received a default summary-LSA: %+v", def.Header)
	}
	if _, ok := db.LookupLSA(normal, type3Key(root, testLSID(t, "10.10.0.0"))); !ok {
		t.Fatal("the backbone's intra-area route was not summarized into the normal area")
	}
}

// RFC requirement: RFC2328-12.4.3-1 positive -- an area border router originates into an attached area a summary-LSA for each intra-area route of its other areas, and condenses the routes that fall inside a configured area address range into one summary-LSA for the range carrying the range's mask (OriginateSummaries with applyAreaRanges, summary.go).
func TestRFC2328ABRSummarizesAndCondenses(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	other := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	OriginateSummaries(abrInput(db, root, other, AreaSummaryPolicy{Type: types.AreaTypeNormal}, nil))
	lsa, ok := db.LookupLSA(other, type3Key(root, testLSID(t, "10.10.0.0")))
	if !ok {
		t.Fatal("no summary-LSA for the backbone's intra-area route 10.10.0.0/24")
	}
	body, err := lsa.DecodeSummary()
	if err != nil {
		t.Fatalf("DecodeSummary: %v", err)
	}
	if body.NetworkMask != ([4]byte{255, 255, 255, 0}) || body.Metric != 7 {
		t.Fatalf("summary body = %+v, want mask 255.255.255.0 metric 7", body)
	}

	condensed := ospflsdb.New(nil)
	ranges := []AreaRange{{Prefix: netip.MustParsePrefix("10.10.0.0/16"), Advertise: true}}
	OriginateSummaries(abrInput(condensed, root, other, AreaSummaryPolicy{Type: types.AreaTypeNormal}, ranges))
	rangeLSA, ok := condensed.LookupLSA(other, type3Key(root, testLSID(t, "10.10.0.0")))
	if !ok {
		t.Fatal("no summary-LSA for the configured range 10.10.0.0/16")
	}
	body, err = rangeLSA.DecodeSummary()
	if err != nil {
		t.Fatalf("DecodeSummary: %v", err)
	}
	if body.NetworkMask != ([4]byte{255, 255, 0, 0}) {
		t.Fatalf("range summary mask = %v, want 255.255.0.0 (the range, not the component)", body.NetworkMask)
	}
}

// RFC requirement: RFC2328-12.4.3-1 negative -- a route that lies inside a configured area address range is not advertised as its own summary-LSA: only the range's summary-LSA appears in the attached area, and a range configured not to advertise suppresses both (applyAreaRanges, summary.go).
func TestRFC2328ABRComponentNeverEscapesRange(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	other := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	ranges := []AreaRange{{Prefix: netip.MustParsePrefix("10.10.0.0/16"), Advertise: true}}
	OriginateSummaries(abrInput(db, root, other, AreaSummaryPolicy{Type: types.AreaTypeNormal}, ranges))
	for _, h := range db.Summary(other) {
		if h.Type != types.LSTypeSummaryNetwork || h.AdvertisingRouter != root {
			continue
		}
		lsa, _ := db.LookupLSA(other, h.Key())
		body, err := lsa.DecodeSummary()
		if err != nil {
			t.Fatalf("DecodeSummary: %v", err)
		}
		if body.NetworkMask == ([4]byte{255, 255, 255, 0}) {
			t.Fatalf("the component 10.10.0.0/24 was advertised beside its range: %+v", h)
		}
	}
	hidden := ospflsdb.New(nil)
	OriginateSummaries(abrInput(hidden, root, other, AreaSummaryPolicy{Type: types.AreaTypeNormal}, []AreaRange{{Prefix: netip.MustParsePrefix("10.10.0.0/16"), Advertise: false}}))
	if h, ok := hidden.LookupLSA(other, type3Key(root, testLSID(t, "10.10.0.0"))); ok {
		t.Fatalf("a non-advertised range still produced a summary-LSA: %+v", h.Header)
	}
}
