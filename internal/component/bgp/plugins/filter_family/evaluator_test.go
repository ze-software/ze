package filter_family

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// handleFilterUpdate runs one encoded route through the evaluator the SDK
// installs on both transports (evaluateFamilyFilter), as the forked
// transport hands it a filter-update route.
func handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	route := rpc.InputFilterRoute(in)
	out, err := evaluateFamilyFilter(&route)
	if err != nil {
		panic("BUG: evaluateFamilyFilter answered an error: " + err.Error())
	}
	return out
}

// VALIDATES: the evaluator honours the fast-path contract on both transports
// (FR-9, AC-36): no blocking, the same answer twice, no view kept past the
// call, equal answers on the slot and over the batch, concurrent use; the
// routes cover strip, suppress, tear-down and no match.
// PREVENTS: an evaluator that is only correct when the bridge loop serialised it.
func TestFamilyFilterConformance(t *testing.T) {
	setInstances(t, flowInstances())
	routes := []sdk.ConformanceRoute{
		{Input: rpc.FilterUpdateInput{Filter: "NoFlow", Direction: "export", Raw: buildUpdate(mpFlowReachAttr(), nil)}},
		{Input: rpc.FilterUpdateInput{Filter: "Kill", Direction: "import", Raw: buildUpdate(mpFlowReachAttr(), nil)}},
		{Input: rpc.FilterUpdateInput{Filter: "NoFlow", Direction: "import", Raw: buildUpdate(originAttr, []byte{24, 10, 0, 0})}},
	}
	sdk.CheckFilterConformance(t, "family-filter", evaluateFamilyFilter, routes)
}
