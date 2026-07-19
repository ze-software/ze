// VALIDATES: spec-ospf-11 RFC 2328 sec 3.6 -- a stub area ABR injects exactly one
// Type 3 default (0.0.0.0/0) at default-cost and no Type 4 ASBR-Summary; a totally-
// stubby area (no-summary) suppresses every Type 3 EXCEPT that default.
// PREVENTS: regressions where a stub area leaks Type 4, gets no default, or a
// totally-stubby area still floods inter-area Type 3 summaries.
package spf

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ospflsdb "codeberg.org/thomas-mangin/ze/internal/plugins/ospf/lsdb"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func summaryDefaultKey(root types.RouterID) types.LSAKey {
	return types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: types.LinkStateID([4]byte{}), AdvertisingRouter: root}
}

func TestOSPFStubDefaultInjection(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	backbone := types.BackboneArea
	stub := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	results := map[types.AreaID]*Result{backbone: resultWithStub(backbone, root, "10.10.0.0", 7)}

	OriginateSummaries(SummaryInput{
		Sink:     db,
		Root:     root,
		Areas:    []types.AreaID{backbone, stub},
		Options:  map[types.AreaID]types.Options{stub: 0},
		Results:  results,
		Policies: map[types.AreaID]AreaSummaryPolicy{stub: {Type: AreaTypeStub, DefaultCost: 5}},
	})

	def, ok := db.LookupLSA(stub, summaryDefaultKey(root))
	require.True(t, ok, "stub area gets a Type 3 default")
	body, err := def.DecodeSummary()
	require.NoError(t, err)
	assert.Equal(t, uint32(5), body.Metric, "default originated at default-cost")
	assert.Equal(t, [4]byte{}, body.NetworkMask, "0.0.0.0/0 mask is 0.0.0.0")

	// A normal (not totally-stubby) stub still receives inter-area Type 3 summaries.
	_, ok = db.LookupLSA(stub, types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: testLSID(t, "10.10.0.0"), AdvertisingRouter: root})
	assert.True(t, ok, "a normal stub still gets inter-area Type 3 summaries")
}

func TestOSPFTotallyStubbyOnlyDefault(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	backbone := types.BackboneArea
	stub := areaID(t, "0.0.0.1")
	db := ospflsdb.New(nil)
	results := map[types.AreaID]*Result{backbone: resultWithStub(backbone, root, "10.10.0.0", 7)}

	OriginateSummaries(SummaryInput{
		Sink:     db,
		Root:     root,
		Areas:    []types.AreaID{backbone, stub},
		Options:  map[types.AreaID]types.Options{stub: 0},
		Results:  results,
		Policies: map[types.AreaID]AreaSummaryPolicy{stub: {Type: AreaTypeStub, NoSummary: true, DefaultCost: 9}},
	})

	def, ok := db.LookupLSA(stub, summaryDefaultKey(root))
	require.True(t, ok, "totally-stubby still gets the injected default")
	body, err := def.DecodeSummary()
	require.NoError(t, err)
	assert.Equal(t, uint32(9), body.Metric)

	_, ok = db.LookupLSA(stub, types.LSAKey{Type: types.LSTypeSummaryNetwork, LinkStateID: testLSID(t, "10.10.0.0"), AdvertisingRouter: root})
	assert.False(t, ok, "no-summary suppresses every inter-area Type 3 except the default")
}

// TestOSPFNSSAType3SummaryImport pins RFC 3101 sec 2.7: a regular NSSA (summary import is the
// default) still imports inter-area Type-3 summary-LSAs, while a no-summary (totally-)NSSA
// suppresses them. A Type-4 ASBR-summary is never imported into an NSSA in either case.
func TestOSPFNSSAType3SummaryImport(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	type3 := summaryDesired{Type: types.LSTypeSummaryNetwork, LSID: testLSID(t, "10.10.0.0"), Mask: [4]byte{255, 255, 0, 0}, Metric: 7}
	type4 := summaryDesired{Type: types.LSTypeSummaryASBR, LSID: types.LinkStateID(root), Metric: 3}
	desired := []summaryDesired{type3, type4}

	// Regular NSSA: summary import is the default, so the inter-area Type-3 survives; the
	// Type-4 ASBR-summary is dropped.
	// RFC requirement: RFC3101-2.7-1 positive -- a regular NSSA imports the inter-area Type-3
	// summary-LSA (the ABR keeps it in the desired set it originates into the NSSA).
	imported := applyAreaTypePolicy(desired, AreaSummaryPolicy{Type: AreaTypeNSSA})
	assert.True(t, slices.Contains(imported, type3), "a regular NSSA imports the inter-area Type-3 summary")
	assert.False(t, slices.Contains(imported, type4), "a Type-4 ASBR-summary is not imported into an NSSA")

	// No-summary (totally-)NSSA: the inter-area Type-3 is suppressed (and the Type-4 is still
	// dropped).
	// RFC requirement: RFC3101-2.7-1 negative -- a no-summary (totally-)NSSA suppresses the
	// inter-area Type-3 summary-LSA (import is off), and still drops the Type-4 ASBR-summary.
	suppressed := applyAreaTypePolicy(desired, AreaSummaryPolicy{Type: AreaTypeNSSA, NoSummary: true})
	assert.False(t, slices.Contains(suppressed, type3), "a no-summary NSSA suppresses the inter-area Type-3 summary")
	assert.False(t, slices.Contains(suppressed, type4), "a Type-4 ASBR-summary is not imported into a no-summary NSSA")
}
