package policychain

import (
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestWithdrawVerdict proves an external member's withdraw answer is its own
// outcome, never a reject: the run ends OutcomeWithdraw naming the member, it
// is not a failure, the later member is not called, and the sets an earlier
// member folded are dropped (a withdrawal carries no attributes). Sets beside
// withdraw fail the route like sets beside accept. Method: a chain of an
// in-process local-preference set, a withdrawing external member and a probe
// that records being called, on both directions.
func TestWithdrawVerdict(t *testing.T) {
	answer := func(out rpc.FilterUpdateOutput) func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
		return func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) { return &out, nil }
	}
	declared := ExternalDeclaration{Reads: []string{"local-preference"}, Sets: []string{"local-preference"}, OnError: rpc.OnErrorReject}
	for _, direction := range []string{"import", "export"} {
		t.Run(direction, func(t *testing.T) {
			caller := &fakeCaller{filters: map[string]fakeFilter{
				"ext:gone":  {decl: declared, answer: answer(rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw})},
				"ext:after": {decl: declared, answer: answer(rpc.FilterUpdateOutput{Action: rpc.FilterAccept})},
			}}
			chain := compileMembers(t, testAttachment,
				[]Member{{Name: "lp200"}, {Name: "gone", Plugin: "ext"}, {Name: "after", Plugin: "ext"}},
				policy.Instance{Name: "lp200", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 200)})
			outcome, sets := runRoute(t, chain, caller, direction, localPref(100))
			if outcome.Action != OutcomeWithdraw {
				t.Fatalf("outcome = %+v, want withdraw", outcome)
			}
			if outcome.Failed {
				t.Fatalf("outcome = %+v: a withdraw is a decision, not a failure", outcome)
			}
			if outcome.Member != "ext:gone" {
				t.Fatalf("outcome member = %q, want ext:gone", outcome.Member)
			}
			if _, called := caller.sent["ext:after"]; called {
				t.Fatal("a member after the withdraw was called")
			}
			if !sets.Folded().Empty() {
				t.Fatal("the sets folded before the withdraw survived it")
			}
		})
	}
	t.Run("sets beside withdraw", func(t *testing.T) {
		caller := &fakeCaller{filters: map[string]fakeFilter{"ext:gone": {decl: declared, answer: answer(rpc.FilterUpdateOutput{
			Action: rpc.FilterWithdraw, Sets: []rpc.FilterSet{set("local-preference", rpc.FilterSetActionSet, "5")}})}}}
		chain := compileMembers(t, testAttachment, []Member{{Name: "gone", Plugin: "ext"}})
		outcome, _ := runRoute(t, chain, caller, "import", localPref(100))
		if outcome.Action != OutcomeReject || !outcome.Failed {
			t.Fatalf("outcome = %+v, want a failed reject", outcome)
		}
	})
}
