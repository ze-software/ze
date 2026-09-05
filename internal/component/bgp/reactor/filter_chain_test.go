// Design: docs/architecture/core-design.md — policy filter chain tests
package reactor

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
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

// filterSubjectFixture returns the subject the product renders for one UPDATE
// carrying every attribute attrNameToCode advertises and one IPv4 unicast
// prefix, through the arm every runtime caller reaches (a nil declared list).
//
// The subject is RENDERED, never typed. A hand-written subject is what let five
// attributes reach no filter for the whole life of appendSingleAttr: a test
// that types its own text asserts what its author believed the product emits,
// and that belief was wrong for every one of those days
// (plan/journal/silent-fall-through.md).
//
// The equality check below is the fixture's own guard. A wire this helper
// cannot parse renders as an empty subject, and a test that then asserts the
// ABSENCE of something passes for that reason rather than for the reason it was
// written -- which is what medAttrsWire (forward_med_test.go) did while it
// built its wire on a context id nothing had registered.
func filterSubjectFixture(t *testing.T) string {
	t.Helper()

	attrs := attrsFixtureWire(t)
	packed := attrs.Packed()

	// RFC 4271 Section 4.3: Withdrawn Routes Length, Withdrawn Routes, Total
	// Path Attribute Length, Path Attributes, then the NLRI.
	body := make([]byte, 0, 4+len(packed)+4)
	body = binary.BigEndian.AppendUint16(body, 0)
	body = binary.BigEndian.AppendUint16(body, uint16(len(packed))) //nolint:gosec // test data, bounded
	body = append(body, packed...)
	body = append(body, 24, 10, 0, 0) // 10.0.0.0/24

	wireUpdate := wireu.NewWireUpdate(body, attrs.SourceContext())
	subject := string(AppendUpdateForFilter(nil, attrs, wireUpdate, nil))

	require.Equal(t, attrsFixtureSubject+" nlri ipv4/unicast add 10.0.0.0/24", subject,
		"the fixture must render the whole subject, or every test reading it proves nothing")
	return subject
}

// TestFilterSubjectRoundTripsThroughParseAndFormat renders the subject a filter
// chain carries, reads it back with parseFilterAttrs, and renders it again with
// formatFilterAttrs.
//
// VALIDATES: AC-8 -- the round trip is byte stable over all thirteen attribute
// names and the NLRI block, which is assumption A-3.
// PREVENTS: a chain that changes one attribute silently reshaping the others.
// applyFilterDelta (filter_chain.go) merges every delta INTO the parsed subject
// and re-renders the whole of it, so a name the parser drops leaves the route
// at the first modify, and a name the two orders disagree about moves. The
// caller then compares the result with the text it sent (filter_ordered.go) and
// rebuilds the payload for a route no filter asked to change.
func TestFilterSubjectRoundTripsThroughParseAndFormat(t *testing.T) {
	subject := filterSubjectFixture(t)

	parsed := parseFilterAttrs(subject)

	for name := range attrNameToCode {
		id, known := filterAttrNameToID[name]
		require.True(t, known, "every advertised name needs a parser id: "+name)
		assert.True(t, parsed.has(id), "the parser must read "+name+" back out of the rendered subject")
	}
	assert.True(t, parsed.has(faNLRI), "the NLRI block must survive the parse")
	assert.Empty(t, parsed.unknownName, "every token the renderer wrote must be a name the parser knows")

	assert.Equal(t, subject, formatFilterAttrs(parsed),
		"render, parse and re-render must produce the same bytes")
}

// TestParseFilterAttrsStruct covers the filter-text parse: every directive
// name, the single-token and multi-token shapes, the nlri block, and the two
// valueless tokens whose presence is the whole signal.
//
// THE WHITESPACE CASES ARE THE POINT. The parse no longer calls strings.Fields
// and no longer rebuilds a multi-token value with textbuf.Join: it takes the
// window the tokens already sit in. That is only equal to the join when one
// space separates them, so a run separated by anything else goes through
// joinFilterTokens instead, and these cases are what says the two agree.
//
// VALIDATES: AC-1 -- the parse output is unchanged by the allocation work.
// PREVENTS: a value silently keeping a plugin's tabs or double spaces, which
// reads as a changed attribute and emits a wire operation nothing asked for.
func TestParseFilterAttrsStruct(t *testing.T) {
	t.Run("every_directive_name", func(t *testing.T) {
		text := "origin igp as-path 65000 65001 next-hop 192.0.2.1 med 50 " +
			"local-preference 200 atomic-aggregate aggregator 65000:192.0.2.1 " +
			"community 65000:1 65000:2 originator-id 10.0.0.1 " +
			"cluster-list 10.0.0.1 10.0.0.2 extended-community target:65000:100 " +
			"aigp 100 large-community 65000:1:2 community-add 65000:3 " +
			"community-remove 65000:4 large-community-add 65000:5:6 " +
			"large-community-remove 65000:7:8 extended-community-add target:65000:9 " +
			"extended-community-remove target:65000:10 med-remove " +
			"as-path-prepend 2 remove-private strip " +
			"nlri ipv4/unicast add 10.0.0.0/24 10.1.0.0/24"

		attrs := parseFilterAttrs(text)

		want := map[filterAttrID]string{
			faOrigin: "igp", faASPath: "65000 65001", faNextHop: "192.0.2.1",
			faMED: "50", faLocalPreference: "200", faAtomicAggregate: "",
			faAggregator: "65000:192.0.2.1", faCommunity: "65000:1 65000:2",
			faOriginatorID: "10.0.0.1", faClusterList: "10.0.0.1 10.0.0.2",
			faExtendedCommunity: "target:65000:100", faAIGP: "100",
			faLargeCommunity: "65000:1:2", faCommunityAdd: "65000:3",
			faCommunityRemove: "65000:4", faLargeCommunityAdd: "65000:5:6",
			faLargeCommunityRemove: "65000:7:8", faExtendedCommunityAdd: "target:65000:9",
			faExtendedCommunityRemove: "target:65000:10", faMEDRemove: "",
			faASPathPrepend: "2", faRemovePrivate: "strip",
			faNLRI: "nlri ipv4/unicast add 10.0.0.0/24 10.1.0.0/24",
		}
		for id := filterAttrID(0); id < faCount; id++ { //nolint:modernize // matches the enum walk in textDeltaToModOps
			value, present := attrs.get(id)
			require.True(t, present, "%s is in the text and must be present", filterAttrNames[id])
			assert.Equal(t, want[id], value, filterAttrNames[id])
		}
		assert.Empty(t, attrs.unknownName, "every token at a key position is a known name")
	})

	t.Run("presence_is_not_a_non_empty_value", func(t *testing.T) {
		attrs := parseFilterAttrs("origin igp atomic-aggregate med-remove")

		value, present := attrs.get(faAtomicAggregate)
		assert.True(t, present, "ATOMIC_AGGREGATE is present with a zero-length wire value")
		assert.Empty(t, value)
		assert.True(t, attrs.has(faMEDRemove), "med-remove names an action and takes no operand")
		assert.False(t, attrs.has(faCommunity), "an attribute the text does not name is absent")
	})

	t.Run("a_valueless_token_does_not_eat_the_next_name", func(t *testing.T) {
		attrs := parseFilterAttrs("atomic-aggregate med 50")

		value, present := attrs.get(faMED)
		require.True(t, present, "the token after atomic-aggregate is still a name")
		assert.Equal(t, "50", value)
	})

	t.Run("separators_that_are_not_one_space", func(t *testing.T) {
		cases := []struct {
			name string
			text string
			want string
		}{
			{"two_spaces", "community 65000:1  65000:2", "65000:1 65000:2"},
			{"a_tab", "community 65000:1\t65000:2", "65000:1 65000:2"},
			{"mixed_runs", "community  65000:1 \t 65000:2   65000:3", "65000:1 65000:2 65000:3"},
			{"leading_and_trailing", "  community 65000:1 65000:2  ", "65000:1 65000:2"},
			{"one_space_is_the_window", "community 65000:1 65000:2", "65000:1 65000:2"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				value, present := parseFilterAttrs(c.text).get(faCommunity)
				require.True(t, present)
				assert.Equal(t, c.want, value,
					"the value is normalized to single spaces, whatever the text used")
			})
		}
	})

	t.Run("an_unknown_name_is_recorded_once", func(t *testing.T) {
		attrs := parseFilterAttrs("origin igp bogus 1 alsobogus 2")

		assert.Equal(t, "bogus", attrs.unknownName, "the FIRST unknown token is what validateModifyDelta reports")
		assert.True(t, attrs.has(faOrigin), "a known name after an unknown one is still parsed")
	})

	t.Run("empty_and_whitespace_only_text", func(t *testing.T) {
		assert.Zero(t, parseFilterAttrs("").present, "no text, no attribute")
		assert.Zero(t, parseFilterAttrs("   \t ").present, "whitespace names nothing either")
	})

	t.Run("into_caller_storage_answers_the_same", func(t *testing.T) {
		text := "origin igp community 65000:1 65000:2 nlri ipv4/unicast add 10.0.0.0/24"

		var reused filterAttrs
		parseFilterAttrsInto(&reused, "med 50 large-community 1:2:3")
		parseFilterAttrsInto(&reused, text)

		assert.Equal(t, *parseFilterAttrs(text), reused,
			"a reused struct carries nothing from the parse before it")
	})
}
