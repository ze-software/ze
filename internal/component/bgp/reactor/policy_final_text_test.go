// Design: docs/architecture/core-design.md -- explicit terminal policy disposition.
// Related: policy_final_test.go -- the disposition tests that outlive the text chain
//
// The text chain's own final-disposition tests. They drive PolicyFilterChain
// and are deleted with it; their library twins are
// policy_library_boundaries_test.go and policychain TestChainFinalReject.
package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// TestPolicyFilterChainFinalDisposition exercises exhaustion and raw terminal
// outcomes without allowing a filter's accept or modification to grant acceptance.
func TestPolicyFilterChainFinalDisposition(t *testing.T) {
	for _, final := range []routepolicy.Final{routepolicy.FinalAccept, routepolicy.FinalReject} {
		for _, action := range []PolicyAction{PolicyAccept, PolicyReject, PolicyModify} {
			for _, raw := range []bool{false, true} {
				policy, err := filterapi.NewPolicy([]routepolicy.Ref{{Name: "test:first"}, {Name: "test:last"}}, final)
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				result := PolicyFilterChain(policy, true, directionExport, "192.0.2.1", 65001, "origin igp", func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
					calls++
					if raw && action == PolicyModify {
						return PolicyResponse{Action: action, Raw: []byte{0, 0, 0, 0}}
					}
					return PolicyResponse{Action: action, Delta: "local-preference 200"}
				})
				wantReject := final == routepolicy.FinalReject || action == PolicyReject
				if (result.Action == PolicyReject) != wantReject || result.Failed || result.Teardown {
					t.Fatalf("final=%v action=%v raw=%v: result=%+v", final, action, raw, result)
				}
				wantCalls := 2
				if action == PolicyReject || (raw && action == PolicyModify) {
					wantCalls = 1
				}
				if calls != wantCalls {
					t.Fatalf("calls=%d want=%d", calls, wantCalls)
				}
			}
		}
	}
}

// TestPolicyFinalWithoutActiveFilters proves nil transport is irrelevant to a
// terminal decision, including a direction whose only reference is inactive.
func TestPolicyFinalWithoutActiveFilters(t *testing.T) {
	for _, refs := range [][]routepolicy.Ref{nil, {{Name: "test:inactive", Inactive: true}}} {
		for _, final := range []routepolicy.Final{routepolicy.FinalAccept, routepolicy.FinalReject} {
			policy, err := filterapi.NewPolicy(refs, final)
			if err != nil {
				t.Fatal(err)
			}
			result := PolicyFilterChain(policy, true, directionExport, "192.0.2.1", 65001, "origin igp", nil)
			if (result.Action != PolicyReject) != policy.AcceptsAtEnd(true) || result.Failed {
				t.Fatalf("final=%v result=%+v", final, result)
			}
		}
	}
}

// TestPolicyFinalDoesNotReplaceTerminalFailure preserves transport and teardown
// distinctions even though a final reject is itself a genuine policy decision.
func TestPolicyFinalDoesNotReplaceTerminalFailure(t *testing.T) {
	for _, final := range []routepolicy.Final{routepolicy.FinalAccept, routepolicy.FinalReject} {
		for _, terminal := range []PolicyResponse{
			{Action: PolicyReject, Failed: true},
			{Action: PolicyReject, Teardown: true, NotifyCode: 6, NotifySubcode: 2},
		} {
			policy := mustPolicy(t, []routepolicy.Ref{{Name: "test:first"}, {Name: "test:unreachable"}}, final)
			calls := 0
			result := PolicyFilterChain(policy, true, directionImport, "192.0.2.1", 65001, "origin igp", func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				calls++
				return terminal
			})
			if calls != 1 || result.Action != PolicyReject || result.Failed != terminal.Failed ||
				result.Teardown != terminal.Teardown || result.NotifyCode != terminal.NotifyCode || result.NotifySubcode != terminal.NotifySubcode {
				t.Fatalf("terminal result changed by final=%v: %+v", final, result)
			}
		}
	}
}
