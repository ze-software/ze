// Design: docs/architecture/core-design.md — policy filter chain tests
package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		[]string{"test:accept"}, "import", "10.0.0.1", 65001,
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
		[]string{"test:reject", "test:never"}, "import", "10.0.0.1", 65001,
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
		[]string{"test:modify"}, "import", "10.0.0.1", 65001,
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
		[]string{"a:set200", "b:set300", "c:accept"}, "import", "10.0.0.1", 65001,
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
		[]string{"a:accept", "b:reject", "c:never"}, "import", "10.0.0.1", 65001,
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
	PolicyFilterChain([]string{"inactive:rpki:validate", "community:scrub"}, "import", "10.0.0.1", 65001, "origin igp", fn)
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
	PolicyFilterChain([]string{"rpki:validate"}, "import", "10.0.0.1", 65001, "origin igp", fn)
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
		[]string{"fam:kill", "test:never"}, "import", "10.0.0.1", 65001,
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
			return PolicyResponse{Action: PolicyModify, Raw: "00000000"}
		}
		return PolicyResponse{Action: PolicyAccept}
	}
	res := PolicyFilterChain(
		[]string{"fam:strip", "test:never"}, "import", "10.0.0.1", 65001,
		"origin igp", fn,
	)
	assert.Equal(t, PolicyModify, res.Action)
	assert.Equal(t, "00000000", res.Raw)
	assert.Equal(t, 1, calls) // raw rewrite is terminal: second filter never called
}

// TestDecodeFilterRawOverride verifies hex decoding and bounds rejection.
//
// VALIDATES: raw override decode rejects malformed/too-short bodies (fail-safe).
// PREVENTS: a malformed raw response replacing the payload with garbage.
func TestDecodeFilterRawOverride(t *testing.T) {
	assert.Nil(t, decodeFilterRawOverride(""))     // empty
	assert.Nil(t, decodeFilterRawOverride("zz"))   // not hex
	assert.Nil(t, decodeFilterRawOverride("0000")) // 2 bytes < 4-byte minimum
	assert.Equal(t, []byte{0, 0, 0, 0}, decodeFilterRawOverride("00000000"))
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
