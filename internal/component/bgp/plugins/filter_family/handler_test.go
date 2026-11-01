package filter_family

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func setInstances(t *testing.T, m map[string]*familyFilter) {
	t.Helper()
	instancesByName.Store(&m)
	t.Cleanup(func() { instancesByName.Store(nil) })
}

func famInput(filter, direction string, body []byte) *sdk.FilterUpdateInput {
	return &sdk.FilterUpdateInput{Filter: filter, Direction: direction, Raw: body}
}

func flowInstances() map[string]*familyFilter {
	fam, _ := family.LookupFamily("ipv4/flow")
	return map[string]*familyFilter{
		"NoFlow": {name: "NoFlow", family: fam, action: actionRemove},
		"Kill":   {name: "Kill", family: fam, action: actionTearDown},
	}
}

// TestHandleRemovePureSuppresses validates AC-2/AC-4: a pure MP-only flowspec
// UPDATE is rejected (whole-UPDATE suppress) when remove matches.
//
// VALIDATES: AC-2 (export), AC-4 (import) -- remove of an MP-only family suppresses.
// PREVENTS: sending/keeping an empty UPDATE after stripping the only NLRI.
func TestHandleRemovePureSuppresses(t *testing.T) {
	setInstances(t, flowInstances())
	out := handleFilterUpdate(famInput("NoFlow", "export", buildUpdate(mpFlowReachAttr(), nil)))
	assert.Equal(t, sdk.FilterReject, out.Action)
}

// TestHandleRemoveMixedStrips validates AC-3: a mixed UPDATE keeps its legacy NLRI
// and loses the flowspec family, answered as the family-strip intent the engine
// resolves (D-5), never as a raw body.
//
// VALIDATES: AC-3 -- mixed UPDATE: strip MP family, keep ipv4-unicast NLRI.
// PREVENTS: dropping the whole UPDATE (which would lose the unicast routes), or
// the plugin rewriting the body itself.
func TestHandleRemoveMixedStrips(t *testing.T) {
	setInstances(t, flowInstances())
	nlri := []byte{24, 10, 0, 0}
	attrs := append(append([]byte{}, originAttr...), mpFlowReachAttr()...)
	out := handleFilterUpdate(famInput("NoFlow", "export", buildUpdate(attrs, nlri)))
	require.Equal(t, sdk.FilterModify, out.Action)
	assert.Empty(t, out.Raw, "the strip is an intent, not a raw rewrite")
	assert.Equal(t, []rpc.FilterSet{{Field: fieldFamily, Action: rpc.FilterSetActionRemove, Value: json.RawMessage(`"ipv4/flow"`)}}, out.Sets)
}

// TestHandleRemoveKeepsOtherFamily validates AC-26 at the plugin: an UPDATE
// carrying the matched family beside another MP family strips only the matched
// one, so the other family's routes still reach the peer.
//
// VALIDATES: AC-26 -- one family of two leaves; the UPDATE is not suppressed.
// PREVENTS: the raw rewrite's fault, which dropped every MP attribute and then
// suppressed the UPDATE as emptied, losing the other family's withdrawals.
func TestHandleRemoveKeepsOtherFamily(t *testing.T) {
	setInstances(t, flowInstances())
	attrs := append(append([]byte{}, mpFlowReachAttr()...), mpIPv6UnreachAttr()...)
	out := handleFilterUpdate(famInput("NoFlow", "import", buildUpdate(attrs, nil)))
	require.Equal(t, sdk.FilterModify, out.Action)
	assert.Equal(t, []rpc.FilterSet{{Field: fieldFamily, Action: rpc.FilterSetActionRemove, Value: json.RawMessage(`"ipv4/flow"`)}}, out.Sets)
}

// TestHandleTearDown validates AC-5: a matching import UPDATE requests teardown
// with a Cease / Connection Rejected NOTIFICATION; export is defensively accepted.
//
// VALIDATES: AC-5 -- tear-down on import sets Teardown + NOTIFICATION codes.
// PREVENTS: tearing a session down on export, or with a wrong/zero code.
func TestHandleTearDown(t *testing.T) {
	setInstances(t, flowInstances())

	out := handleFilterUpdate(famInput("Kill", "import", buildUpdate(mpFlowReachAttr(), nil)))
	assert.Equal(t, sdk.FilterReject, out.Action)
	assert.True(t, out.Teardown)
	assert.Equal(t, uint8(6), out.NotifyCode)    // Cease
	assert.Equal(t, uint8(5), out.NotifySubcode) // Connection Rejected

	// Export direction must NOT tear down (defensive; config also forbids it).
	out = handleFilterUpdate(famInput("Kill", "export", buildUpdate(mpFlowReachAttr(), nil)))
	assert.Equal(t, sdk.FilterAccept, out.Action)
	assert.False(t, out.Teardown)
}

// TestHandleNoMatchAndUnknown validates AC-6 (no match -> accept) and fail-closed
// on an unknown filter name.
//
// VALIDATES: AC-6 -- non-matching family passes unchanged; unknown filter rejects.
// PREVENTS: a different family being altered, or an unknown ref silently passing.
func TestHandleNoMatchAndUnknown(t *testing.T) {
	setInstances(t, flowInstances())

	// ipv4/unicast UPDATE, flowspec filter -> no match -> accept.
	out := handleFilterUpdate(famInput("NoFlow", "import", buildUpdate(originAttr, []byte{24, 10, 0, 0})))
	assert.Equal(t, sdk.FilterAccept, out.Action)

	// Unknown filter name -> fail-closed reject.
	out = handleFilterUpdate(famInput("Nope", "import", buildUpdate(mpFlowReachAttr(), nil)))
	assert.Equal(t, sdk.FilterReject, out.Action)
}
