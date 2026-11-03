package policychain

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The panicking test types: each sets local-preference, then panics, so a
// sets-kept defect shows in the folded sets.
const (
	typePanicClosed = "policychain-test-panic-closed" // on-error reject
	typePanicOpen   = "policychain-test-panic-open"   // on-error accept
)

func panicEvaluator(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
	return func(_ *policyview.View, sets *Sets) (Decision, error) {
		if err := sets.LocalPreference(NumericActionSet, 7); err != nil {
			return DecisionUnspecified, err
		}
		var table []int
		return Decision(table[3]), nil // index out of range: a Ze defect in the member
	}, nil
}

func init() {
	lp := policy.FieldLocalPreference
	policy.MustRegisterFilterType(typePanicClosed, []policy.Field{lp}, []policy.Field{lp})
	policy.MustRegisterFilterType(typePanicOpen, []policy.Field{lp}, []policy.Field{lp})
	MustRegisterEvaluator(typePanicClosed, rpc.OnErrorReject, panicEvaluator)
	MustRegisterEvaluator(typePanicOpen, rpc.OnErrorAccept, panicEvaluator)
}

// TestChainMemberPanicAppliesOnError proves an in-process member that panics
// cannot take the calling goroutine down: the run recovers it as the member's
// failure, so an on-error reject member fails the route closed (a Failed
// reject naming it) and an on-error accept member passes it with none of its
// sets, each counted. Method: a member that writes a set and then indexes past
// an empty slice, run twice on the same chain to show it stays usable; an
// escaped panic would end the test binary.
func TestChainMemberPanicAppliesOnError(t *testing.T) {
	closed := compile(t, policy.FinalAccept, policy.Instance{Name: "boom", Type: typePanicClosed})
	for range 2 {
		outcome, sets := run(t, closed, update(t, localPref(100)))
		if outcome != (Outcome{Action: OutcomeReject, Failed: true, Member: "boom"}) {
			t.Fatalf("fail-closed outcome = %+v, want a failed reject naming the member", outcome)
		}
		if !sets.Folded().Empty() {
			t.Fatal("a panicked member's sets were kept")
		}
	}
	if closed.Refused() != 2 {
		t.Fatalf("refused = %d, want 2", closed.Refused())
	}

	open := compile(t, policy.FinalAccept,
		policy.Instance{Name: "boom", Type: typePanicOpen},
		policy.Instance{Name: "med", Type: typeSetMED, Config: numericConfig(NumericActionSet, 5)})
	outcome, sets := run(t, open, update(t, localPref(100), med(1)))
	if outcome.Action != OutcomeAccept || outcome.Failed {
		t.Fatalf("fail-open outcome = %+v, want accept", outcome)
	}
	if sets.Folded().written&setKindLocalPreference.bit() != 0 {
		t.Fatal("a panicked fail-open member's local-preference set was applied")
	}
	if open.Refused() != 1 {
		t.Fatalf("fail-open refused = %d, want 1", open.Refused())
	}
}
