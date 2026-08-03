// Design: docs/architecture/core-design.md — policy filter chain tests
package reactor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// frefs builds a FilterRef chain from canonical strings, treating an
// "inactive:" prefix as the structural deactivation flag. It lets these tests
// keep expressing chains in the compact string form they were written against
// while exercising the post-refactor []FilterRef API.
func frefs(names ...string) []filterapi.FilterRef {
	if len(names) == 0 {
		return nil
	}
	out := make([]filterapi.FilterRef, len(names))
	for i, n := range names {
		if rest, ok := strings.CutPrefix(n, "inactive:"); ok {
			out[i] = filterapi.FilterRef{Name: rest, Inactive: true}
		} else {
			out[i] = filterapi.FilterRef{Name: n}
		}
	}
	return out
}

// TestPolicyFilterChainAccept verifies accept passes through unchanged.
//
// VALIDATES: AC-5 -- Import filter returns accept, UPDATE passes through.
// PREVENTS: Accept action corrupting the update text.
func TestPolicyFilterChainAccept(t *testing.T) {
	calls := 0
	fn := func(_, _, _, _ string, _ uint32, text string) PolicyResponse {
		calls++
		return PolicyResponse{Action: PolicyAccept}
	}
	res := PolicyFilterChain(
		frefs("test:accept"), "import", "10.0.0.1", 65001,
		"origin igp as-path 65001 65002", fn,
	)
	assert.Equal(t, PolicyAccept, res.Action)
	assert.Equal(t, "origin igp as-path 65001 65002", res.Text)
	assert.Equal(t, 1, calls)
}

// TestPolicyFilterChainReject verifies reject short-circuits.
//
// VALIDATES: AC-6 -- Import filter returns reject, UPDATE dropped.
// PREVENTS: Reject not stopping chain.
func TestPolicyFilterChainReject(t *testing.T) {
	calls := 0
	fn := func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		calls++
		return PolicyResponse{Action: PolicyReject}
	}
	res := PolicyFilterChain(
		frefs("test:reject", "test:never"), "import", "10.0.0.1", 65001,
		"origin igp", fn,
	)
	assert.Equal(t, PolicyReject, res.Action)
	assert.Empty(t, res.Text)
	assert.Equal(t, 1, calls) // second filter never called
}

// TestPolicyFilterChainModify verifies modify changes attributes.
//
// VALIDATES: AC-7 -- Import filter modifies local-pref.
// PREVENTS: Delta not applied to output.
func TestPolicyFilterChainModify(t *testing.T) {
	fn := func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
		return PolicyResponse{Action: PolicyModify, Delta: "local-preference 200"}
	}
	res := PolicyFilterChain(
		frefs("test:modify"), "import", "10.0.0.1", 65001,
		"origin igp local-preference 100", fn,
	)
	assert.Equal(t, PolicyAccept, res.Action)
	assert.Contains(t, res.Text, "local-preference 200")
	assert.Contains(t, res.Text, "origin igp")
}

// TestPolicyFilterChainPipedTransform verifies piped transforms.
//
// VALIDATES: AC-11 -- Three filters, first modifies, second sees modification.
// PREVENTS: Second filter seeing stale data.
func TestPolicyFilterChainPipedTransform(t *testing.T) {
	call := 0
	fn := func(_, _, _, _ string, _ uint32, text string) PolicyResponse {
		call++
		switch call {
		case 1: // First filter sets local-pref to 200
			return PolicyResponse{Action: PolicyModify, Delta: "local-preference 200"}
		case 2: // Second filter sees 200, changes to 300
			assert.Contains(t, text, "local-preference 200")
			return PolicyResponse{Action: PolicyModify, Delta: "local-preference 300"}
		case 3: // Third filter accepts
			assert.Contains(t, text, "local-preference 300")
			return PolicyResponse{Action: PolicyAccept}
		}
		return PolicyResponse{Action: PolicyAccept}
	}
	res := PolicyFilterChain(
		frefs("a:set200", "b:set300", "c:accept"), "import", "10.0.0.1", 65001,
		"origin igp local-preference 100", fn,
	)
	assert.Equal(t, PolicyAccept, res.Action)
	assert.Contains(t, res.Text, "local-preference 300")
}

// TestPolicyFilterChainShortCircuit verifies reject stops chain.
//
// VALIDATES: AC-6 -- Reject short-circuits, no further filters called.
// PREVENTS: Filters after reject still executing.
func TestPolicyFilterChainShortCircuit(t *testing.T) {
	calls := 0
	fn := func(_, filterName, _, _ string, _ uint32, _ string) PolicyResponse {
		calls++
		if filterName == "reject" {
			return PolicyResponse{Action: PolicyReject}
		}
		return PolicyResponse{Action: PolicyAccept}
	}
	res := PolicyFilterChain(
		frefs("a:accept", "b:reject", "c:never"), "import", "10.0.0.1", 65001,
		"origin igp", fn,
	)
	assert.Equal(t, PolicyReject, res.Action)
	assert.Equal(t, 2, calls) // c:never never called
}

// TestPolicyFilterChainEmpty verifies empty chain accepts.
//
// VALIDATES: Empty filter chain = default accept.
// PREVENTS: Crash on nil/empty filter list.
func TestPolicyFilterChainEmpty(t *testing.T) {
	res := PolicyFilterChain(nil, "import", "10.0.0.1", 65001, "origin igp", nil)
	assert.Equal(t, PolicyAccept, res.Action)
	assert.Equal(t, "origin igp", res.Text)
}

// TestPolicyFilterChainInactiveSkipped verifies inactive: entries are skipped.
//
// VALIDATES: Filters prefixed with "inactive:" are not called.
// PREVENTS: Deactivated filters still running in the chain.
func TestPolicyFilterChainInactiveSkipped(t *testing.T) {
	var called []string
	fn := func(plugin, filter, _, _ string, _ uint32, _ string) PolicyResponse {
		called = append(called, plugin+":"+filter)
		return PolicyResponse{Action: PolicyAccept}
	}
	PolicyFilterChain(frefs("inactive:rpki:validate", "community:scrub"), "import", "10.0.0.1", 65001, "origin igp", fn)
	assert.Equal(t, []string{"community:scrub"}, called, "inactive filter should not be called")
}

// TestPolicyFilterChainDispatch verifies plugin:filter name splitting.
//
// VALIDATES: AC-17 -- Filter name dispatched correctly to callback.
// PREVENTS: Wrong plugin/filter name passed to callback.
func TestPolicyFilterChainDispatch(t *testing.T) {
	var gotPlugin, gotFilter, gotDir string
	fn := func(plugin, filter, dir, _ string, _ uint32, _ string) PolicyResponse {
		gotPlugin = plugin
		gotFilter = filter
		gotDir = dir
		return PolicyResponse{Action: PolicyAccept}
	}
	PolicyFilterChain(frefs("rpki:validate"), "import", "10.0.0.1", 65001, "origin igp", fn)
	assert.Equal(t, "rpki", gotPlugin)
	assert.Equal(t, "validate", gotFilter)
	assert.Equal(t, "import", gotDir)
}

// TestPolicyFilterChainTeardown verifies a teardown request short-circuits the
// chain, drops the route (reject), and surfaces the NOTIFICATION code/subcode.
//
// VALIDATES: filter_family tear-down -- import filter requests session teardown.
// PREVENTS: teardown not propagating out of the chain, or not dropping the route.
func TestPolicyFilterChainTeardown(t *testing.T) {
	calls := 0
	fn := func(_, filterName, _, _ string, _ uint32, _ string) PolicyResponse {
		calls++
		if filterName == "kill" {
			return PolicyResponse{Action: PolicyReject, Teardown: true, NotifyCode: 6, NotifySubcode: 5}
		}
		return PolicyResponse{Action: PolicyAccept}
	}
	res := PolicyFilterChain(
		frefs("fam:kill", "test:never"), "import", "10.0.0.1", 65001,
		"origin igp", fn,
	)
	assert.Equal(t, PolicyReject, res.Action)
	assert.True(t, res.Teardown)
	assert.Equal(t, uint8(6), res.NotifyCode)
	assert.Equal(t, uint8(5), res.NotifySubcode)
	assert.Equal(t, 1, calls) // second filter never called (short-circuit)
}

// TestPolicyFilterChainRawTerminal verifies a raw full-payload rewrite is terminal:
// it surfaces on the result and stops the chain (no downstream filters run).
//
// VALIDATES: filter_family remove (mixed UPDATE) -- raw payload replacement.
// PREVENTS: raw override being silently dropped or composed with later text deltas.
func TestPolicyFilterChainRawTerminal(t *testing.T) {
	calls := 0
	fn := func(_, filterName, _, _ string, _ uint32, _ string) PolicyResponse {
		calls++
		if filterName == "strip" {
			return PolicyResponse{Action: PolicyModify, Raw: []byte{0, 0, 0, 0}}
		}
		return PolicyResponse{Action: PolicyAccept}
	}
	res := PolicyFilterChain(
		frefs("fam:strip", "test:never"), "import", "10.0.0.1", 65001,
		"origin igp", fn,
	)
	assert.Equal(t, PolicyModify, res.Action)
	assert.Equal(t, []byte{0, 0, 0, 0}, res.Raw)
	assert.Equal(t, 1, calls) // raw rewrite is terminal: second filter never called
}

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
	assert.False(t, malformed, "nil asks for no override; it is not a malformed one")

	override, malformed = decodeFilterRawOverride([]byte{})
	assert.Nil(t, override, "empty")
	assert.False(t, malformed, "empty asks for no override; it is not a malformed one")

	override, malformed = decodeFilterRawOverride([]byte{0})
	assert.Nil(t, override, "1 byte < 4-byte minimum")
	assert.True(t, malformed, "one byte asked for a replacement that cannot be an UPDATE body")

	override, malformed = decodeFilterRawOverride([]byte{0, 0})
	assert.Nil(t, override, "2 bytes < 4-byte minimum")
	assert.True(t, malformed, "two bytes asked for a replacement that cannot be an UPDATE body")

	override, malformed = decodeFilterRawOverride([]byte{0, 0, 0})
	assert.Nil(t, override, "3 bytes: one below the boundary")
	assert.True(t, malformed, "one below the minimum is still an undecodable override")

	override, malformed = decodeFilterRawOverride([]byte{0, 0, 0, 0})
	assert.Equal(t, []byte{0, 0, 0, 0}, override, "4 bytes: the minimum body")
	assert.False(t, malformed, "the shortest legal body is not malformed")

	override, malformed = decodeFilterRawOverride([]byte{0, 0, 0, 0, 1})
	assert.Equal(t, []byte{0, 0, 0, 0, 1}, override, "5 bytes: one above the boundary")
	assert.False(t, malformed, "above the minimum is not malformed")
}

// TestApplyFilterDelta verifies delta application.
//
// VALIDATES: Delta-only output merges correctly with current attributes.
// PREVENTS: Delta clobbering unrelated attributes.
func TestApplyFilterDelta(t *testing.T) {
	tests := []struct {
		name    string
		current string
		delta   string
		want    string
	}{
		{
			name:    "modify local-pref",
			current: "origin igp local-preference 100",
			delta:   "local-preference 200",
			want:    "origin igp local-preference 200",
		},
		{
			name:    "add community",
			current: "origin igp",
			delta:   "community 65000:1 65000:2",
			want:    "origin igp community 65000:1 65000:2",
		},
		{
			name:    "empty delta",
			current: "origin igp",
			delta:   "",
			want:    "origin igp",
		},
		{
			name:    "modify nlri",
			current: "origin igp nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24",
			delta:   "nlri ipv4/unicast add 10.0.0.0/24",
			want:    "origin igp nlri ipv4/unicast add 10.0.0.0/24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyFilterDelta(tt.current, tt.delta)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestValidateModifyDelta verifies declared attribute enforcement.
//
// VALIDATES: AC-13 -- Filter modifying undeclared attribute is rejected.
// PREVENTS: Plugin modifying attributes it didn't declare interest in.
func TestValidateModifyDelta(t *testing.T) {
	tests := []struct {
		name     string
		delta    string
		declared []string
		wantViol string
	}{
		{
			name:     "valid modify of declared attr",
			delta:    "local-preference 200",
			declared: []string{"local-preference", "community"},
			wantViol: "",
		},
		{
			name:     "modify undeclared attr",
			delta:    "community 65000:1",
			declared: []string{"local-preference"},
			wantViol: "community",
		},
		{
			name:     "empty delta is valid",
			delta:    "",
			declared: []string{"local-preference"},
			wantViol: "",
		},
		{
			name:     "empty declared list rejects any modify",
			delta:    "community 65000:1",
			declared: nil,
			wantViol: "community", // empty declared = all modifications invalid (caller skips validation when declared is empty)
		},
		{
			name:     "nlri modification when declared",
			delta:    "nlri ipv4/unicast add 10.0.0.0/24",
			declared: []string{"nlri"},
			wantViol: "",
		},
		{
			name:     "nlri modification when not declared",
			delta:    "nlri ipv4/unicast add 10.0.0.0/24",
			declared: []string{"local-preference"},
			wantViol: "nlri",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateModifyDelta(tt.delta, tt.declared)
			assert.Equal(t, tt.wantViol, got)
		})
	}
}

// TestPolicyFilterChainPropagatesFailed verifies the chain carries the "the
// filter did not decide this" flag out of every exit that can reject.
//
// VALIDATES: PolicyResponse.Failed reaches PolicyChainResult.Failed. The egress
// step maps that onto egressStepResult.failed, and forwardUpdateCore counts a
// suppression only when it is false, so a lost flag silently turns an
// infrastructure failure back into a "policy said no" and re-opens the fail-open
// that spec-fixit-bgp-egress-rail-divergence exists to close.
// PREVENTS: adding a new reject exit to the chain (or editing an existing one)
// without carrying Failed -- the teardown exit shipped that way and was caught
// only by review.
func TestPolicyFilterChainPropagatesFailed(t *testing.T) {
	refs := []filterapi.FilterRef{{Name: "p:f"}}

	cases := []struct {
		name string
		resp PolicyResponse
		want bool
	}{
		{"plain reject is a decision", PolicyResponse{Action: PolicyReject}, false},
		{"failed reject is not a decision", PolicyResponse{Action: PolicyReject, Failed: true}, true},
		{"teardown reject carries the flag", PolicyResponse{Action: PolicyReject, Teardown: true, Failed: true}, true},
		{"teardown decision does not", PolicyResponse{Action: PolicyReject, Teardown: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PolicyFilterChain(refs, "export", "10.0.0.2", 65002, "origin igp",
				func(_, _, _, _ string, _ uint32, _ string) PolicyResponse { return tc.resp })
			assert.Equal(t, PolicyReject, got.Action, "every case here rejects")
			assert.Equal(t, tc.want, got.Failed, "Failed must survive the chain unchanged")
		})
	}
}

// TestPolicyFilterFuncFlagsNonDecisions verifies a reject produced without any
// filter deciding is marked Failed.
//
// VALIDATES: the nil-API guard is a MISS, not a policy decision. The same
// applies to an IPC error, an unparseable action, and an undeclared-attribute
// modify override, which need a live plugin server to drive and are covered by
// reading rather than by this test (see the spec's Review Gate Run 5).
// PREVENTS: an outcome counter reading "the filter engine is absent" as "the
// operator's policy rejected this route".
func TestPolicyFilterFuncFlagsNonDecisions(t *testing.T) {
	r := &Reactor{} // no API server
	resp := r.policyFilterFunc(nil)("p", "f", "export", "10.0.0.2", 65002, "origin igp")
	assert.Equal(t, PolicyReject, resp.Action, "no filter engine must fail closed")
	assert.True(t, resp.Failed, "a guard miss is not a policy decision")
}
