// VALIDATES: spec-ospf-11 RFC 2328 sec 3.6 -- a stub area ABR injects exactly one
// Type 3 default (0.0.0.0/0) at default-cost and no Type 4 ASBR-Summary; a totally-
// stubby area (no-summary) suppresses every Type 3 EXCEPT that default.
// PREVENTS: regressions where a stub area leaks Type 4, gets no default, or a
// totally-stubby area still floods inter-area Type 3 summaries.
package spf

import (
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
