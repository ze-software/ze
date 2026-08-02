// VALIDATES: RFC 2328 Section 3.6 and RFC 3101 Section 2.7 default
// origination. A stub or no-summary NSSA gets one Type-3 default at
// default-cost. A no-summary area suppresses every other Type-3 and all Type-4s.
// PREVENTS: missing defaults and summary leakage into stub or NSSA areas.
package spf

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
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

// TestOSPFNSSAType3SummaryImport pins RFC 3101 Section 2.7 summary import
// and no-summary default behavior.
func TestOSPFNSSAType3SummaryImport(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	type3 := summaryDesired{Type: types.LSTypeSummaryNetwork, LSID: testLSID(t, "10.10.0.0"), Mask: [4]byte{255, 255, 0, 0}, Metric: 7}
	type4 := summaryDesired{Type: types.LSTypeSummaryASBR, LSID: types.LinkStateID(root), Metric: 3}
	defaultRoute := summaryDesired{Type: types.LSTypeSummaryNetwork, Metric: 9}
	desired := []summaryDesired{type3, type4}

	// RFC requirement: RFC3101-2.7-1 positive -- a regular NSSA imports
	// inter-area Type-3 summary-LSAs and drops Type-4 ASBR summaries.
	imported := applyAreaTypePolicy(desired, AreaSummaryPolicy{Type: AreaTypeNSSA})
	assert.True(t, slices.Contains(imported, type3), "regular NSSA imports the inter-area Type-3 summary")
	assert.False(t, slices.Contains(imported, type4), "NSSA drops the Type-4 ASBR summary")
	// RFC requirement: RFC3101-2.7-2 negative -- a regular NSSA does not
	// receive the no-summary Type-3 default.
	assert.False(t, slices.Contains(imported, defaultRoute), "regular NSSA gets no Type-3 default")

	// RFC requirement: RFC3101-2.7-1 negative -- a no-summary NSSA
	// suppresses imported inter-area Type-3 and Type-4 summaries.
	suppressed := applyAreaTypePolicy(desired, AreaSummaryPolicy{Type: AreaTypeNSSA, NoSummary: true, DefaultCost: 9})
	assert.False(t, slices.Contains(suppressed, type3), "no-summary NSSA suppresses the inter-area Type-3 summary")
	assert.False(t, slices.Contains(suppressed, type4), "no-summary NSSA drops the Type-4 ASBR summary")
	// RFC requirement: RFC3101-2.7-2 positive -- an ABR originates a
	// Type-3 default when summary import is disabled.
	// RFC requirement: RFC3101-2.4-5 positive -- the no-summary NSSA still
	// receives the required default-destination LSA.
	assert.True(t, slices.Contains(suppressed, defaultRoute), "no-summary NSSA gets a Type-3 default")
}

func TestOSPFNSSANoSummaryDefaultInjection(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	backbone := types.BackboneArea
	nssa := areaID(t, "0.0.0.5")
	results := map[types.AreaID]*Result{backbone: resultWithStub(backbone, root, "10.10.0.0", 7)}

	t.Run("no-summary", func(t *testing.T) {
		db := ospflsdb.New(nil)
		OriginateSummaries(SummaryInput{
			Sink: db, Root: root, Areas: []types.AreaID{backbone, nssa},
			Results: results,
			Policies: map[types.AreaID]AreaSummaryPolicy{
				nssa: {Type: AreaTypeNSSA, NoSummary: true, DefaultCost: 11},
			},
		})

		// RFC requirement: RFC3101-2.7-2 positive -- an ABR
		// originates a Type-3 default when summary import is disabled.
		// RFC requirement: RFC3101-2.4-5 positive -- the no-summary
		// NSSA receives the required default-destination LSA.
		lsa, ok := db.LookupLSA(nssa, summaryDefaultKey(root))
		require.True(t, ok)
		body, err := lsa.DecodeSummary()
		require.NoError(t, err)
		assert.Equal(t, uint32(11), body.Metric)
	})

	t.Run("regular", func(t *testing.T) {
		db := ospflsdb.New(nil)
		OriginateSummaries(SummaryInput{
			Sink: db, Root: root, Areas: []types.AreaID{backbone, nssa},
			Results: results,
			Policies: map[types.AreaID]AreaSummaryPolicy{
				nssa: {Type: AreaTypeNSSA, DefaultCost: 11},
			},
		})

		// RFC requirement: RFC3101-2.7-2 negative -- a regular
		// NSSA gets no Type-3 default.
		_, ok := db.LookupLSA(nssa, summaryDefaultKey(root))
		assert.False(t, ok)
	})
}
