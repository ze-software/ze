// Design: docs/architecture/core-design.md — policy filter chain tests
// Related: filter_ordered.go -- the steps these tests drive
//
// Contract tests of the policy steps that outlive the text chain. They moved
// here from filter_chain_test.go and filter_failed_flag_test.go, which are
// deleted with the text format.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestDecodeFilterRawOverride verifies the raw override bounds rejection
// (rib-arch-2: the override is now raw []byte, not a hex string).
//
// VALIDATES: raw override rejects too-short bodies (fail-safe), AND reports
// "no override asked for" separately from "the override cannot be a body".
// PREVENTS: a malformed raw response replacing the payload with garbage, and the
// earlier fold where both answers were nil so an undecodable override read as
// "this filter wanted nothing" and the ORIGINAL payload went out.
//
// The callers are the guard's real entry points and are tested in
// filter_ordered_test.go (TestRunEgressPolicyChainASN4ShortRawOverrideFailsClosed,
// TestRunIngressPolicyChainShortRawOverrideFailsClosed,
// TestPolicyChainRawOverrideBoundary). This is the helper half only
// (ai/rules/evidence.md, test corollary).
func TestDecodeFilterRawOverride(t *testing.T) {
	override, malformed := decodeFilterRawOverride(nil)
	assert.Nil(t, override, "nil")
	assert.NoError(t, malformed, "nil asks for no override; it is not a malformed one")

	override, malformed = decodeFilterRawOverride([]byte{})
	assert.Nil(t, override, "empty")
	assert.NoError(t, malformed, "empty asks for no override; it is not a malformed one")

	override, malformed = decodeFilterRawOverride([]byte{0})
	assert.Nil(t, override, "1 byte < 4-byte minimum")
	assert.Error(t, malformed, "one byte asked for a replacement that cannot be an UPDATE body")

	override, malformed = decodeFilterRawOverride([]byte{0, 0})
	assert.Nil(t, override, "2 bytes < 4-byte minimum")
	assert.Error(t, malformed, "two bytes asked for a replacement that cannot be an UPDATE body")

	override, malformed = decodeFilterRawOverride([]byte{0, 0, 0})
	assert.Nil(t, override, "3 bytes: one below the boundary")
	assert.Error(t, malformed, "one below the minimum is still an undecodable override")

	override, malformed = decodeFilterRawOverride([]byte{0, 0, 0, 0})
	assert.Equal(t, []byte{0, 0, 0, 0}, override, "4 bytes: the minimum body")
	assert.NoError(t, malformed, "the shortest legal body is not malformed")

	override, malformed = decodeFilterRawOverride([]byte{0, 0, 0, 0, 1})
	assert.Equal(t, []byte{0, 0, 0, 0, 1}, override, "5 bytes: one above the boundary")
	assert.NoError(t, malformed, "above the minimum is not malformed")

	override, malformed = decodeFilterRawOverride([]byte{0, 1, 0, 0})
	assert.Nil(t, override, "a withdrawn length past the end")
	assert.Error(t, malformed, "section lengths that overrun the body cannot be an UPDATE body")
}

// TestEgressChainCarriesTheFailedFlagToTheStep is the consequence half: the flag
// exists to reach forwardUpdateCore, and this is the hop that carries it.
//
// VALIDATES: AC-11 -- a filter that could not run leaves the egress step
// `failed`, while a filter that rejected the route on purpose does not.
// PREVENTS: the two branches above being correct and the chain flattening them
// into one answer, which would put the route's drop back into suppressedCount
// and report a lost route as a policy decision (forward_failure_verdict_test.go).
func TestEgressChainCarriesTheFailedFlagToTheStep(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	wire := wireu.NewWireUpdate(modBufTestPayload(), ctxID)
	refs := []routepolicy.Ref{{Name: "policy-plugin:export-policy"}}
	policy := compiledPolicy(t, libraryTestMixed, routepolicy.FinalAccept, refs...)

	t.Run("a filter that could not run", func(t *testing.T) {
		r := &Reactor{api: liveFilterServer(t)}

		got := r.runEgressPolicyChainASN4(policy, "10.0.0.2", 65002, 65000, false, policyview.Session{EBGP: true}, wire, true)

		assert.False(t, got.accept, "fail-closed: the route is withheld")
		assert.True(t, got.failed, "the step must report that it could not decide")
	})

	t.Run("a filter that rejected the route", func(t *testing.T) {
		r := &Reactor{
			api: liveFilterServer(t),
			filterTransportSeam: &fakeFilterTransport{
				onError: rpc.OnErrorReject,
				out:     &rpc.FilterUpdateOutput{Action: rpc.FilterReject},
			},
		}

		got := r.runEgressPolicyChainASN4(policy, "10.0.0.2", 65002, 65000, false, policyview.Session{EBGP: true}, wire, true)

		assert.False(t, got.accept, "the filter said no")
		assert.False(t, got.failed, "this IS a policy decision, and the caller may count it as one")
	})
}
