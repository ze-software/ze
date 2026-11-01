package policychain

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
)

// TestUnconfiguredChainEndsByTheSession proves a chain compiled for a policy
// the operator never configured (its members only inserted by plugins) ends
// as RFC 8212 Section 3 decides for the session: accept on an internal
// session, a policy reject (not a failure) on an external one, and the folded
// sets dropped with the reject.
// Method: one accepting member that sets LOCAL_PREF, run on both session kinds.
func TestUnconfiguredChainEndsByTheSession(t *testing.T) {
	reg, err := policy.NewRegistry(policy.Instance{Name: "lp", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 7)})
	if err != nil {
		t.Fatal(err)
	}
	chain, err := CompileUnconfigured(testAttachment, reg, []Member{{Name: "lp"}}, Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	wire := update(t, localPref(100))
	for _, tc := range []struct {
		name string
		ebgp bool
		want Outcome
	}{
		{"internal session accepts", false, Outcome{Action: OutcomeAccept}},
		{"external session rejects", true, Outcome{Action: OutcomeReject}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var view policyview.View
			var sets RunSets
			view.Reset(wire, policyview.Session{EBGP: tc.ebgp})
			defer view.Clear()
			if got := chain.Run(&view, &sets); got != tc.want {
				t.Fatalf("outcome = %+v, want %+v", got, tc.want)
			}
			if tc.ebgp && !sets.Folded().Empty() {
				t.Fatal("sets kept under the session's reject")
			}
		})
	}
}
