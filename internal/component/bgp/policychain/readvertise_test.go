package policychain

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// runReadvertise runs chain as the stale readvertise rail does.
func runReadvertise(t *testing.T, chain *Chain, caller ExternalCaller, attrs ...[]byte) Outcome {
	t.Helper()
	var view policyview.View
	sets := new(RunSets)
	sets.SetRoute(ExternalRoute{Caller: caller, Direction: "export", Peer: "192.0.2.1", PeerAS: 65001, Readvertise: true})
	view.Reset(update(t, attrs...), policyview.Session{})
	defer view.Clear()
	return chain.Run(&view, sets)
}

// readvertiseChain compiles [lp200, probe:opted, probe:plain] under final
// reject: an in-process member, a plugin filter that declares readvertise and
// one that does not.
func readvertiseChain(t *testing.T) *Chain {
	t.Helper()
	reg, err := policy.NewRegistry(policy.Instance{Name: "lp200", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 200)})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := Compile(testAttachment, reg,
		[]Member{{Name: "lp200"}, {Name: "opted", Plugin: "probe"}, {Name: "plain", Plugin: "probe"}},
		policy.FinalReject, Bases{LocalPreference: 100})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return chain
}

func acceptAnswer(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
	return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
}

func rejectAnswer(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
	return &rpc.FilterUpdateOutput{Action: rpc.FilterReject}, nil
}

// TestReadvertiseRunsOnlyOptedMembers proves the stale readvertise rail's run
// (spec-filter-wire-6 AC-13, "the readvertise rail runs only the LLGR egress
// step"): only the plugin filter that declares readvertise is called, the
// in-process member and the plugin filter that does not opt in are passed
// over (the opted filter reads the received local-preference, not lp200's
// set, and plain is never sent the route), and the route reaching the end is
// accepted although the chain's final is reject. Method: one chain run both
// ways; the ordinary run is the control that calls plain and rejects.
func TestReadvertiseRunsOnlyOptedMembers(t *testing.T) {
	caller := &fakeCaller{filters: map[string]fakeFilter{
		"probe:opted": {decl: ExternalDeclaration{Reads: []string{"local-preference"}, OnError: rpc.OnErrorReject, Readvertise: true}, answer: acceptAnswer},
		"probe:plain": {decl: ExternalDeclaration{Reads: []string{"local-preference"}, OnError: rpc.OnErrorReject}, answer: rejectAnswer},
	}}
	chain := readvertiseChain(t)

	outcome := runReadvertise(t, chain, caller, localPref(100))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("readvertise outcome = %+v, want accept at the end of the opted members", outcome)
	}
	if got := caller.sent["probe:opted"]; got != `{"local-preference":100}` {
		t.Fatalf("opted filter was sent %s, want the received local-preference 100 (lp200 passed over)", got)
	}
	if got, ok := caller.sent["probe:plain"]; ok {
		t.Fatalf("a filter that does not declare readvertise was sent %s on the readvertise rail", got)
	}
	if !chain.Readvertises(caller) {
		t.Fatal("Readvertises = false for a chain holding an opted member")
	}

	caller.sent = nil
	outcome, _ = runRoute(t, chain, caller, "export", localPref(100))
	if outcome.Action != OutcomeReject || outcome.Member != "probe:plain" {
		t.Fatalf("ordinary outcome = %+v, want plain's reject (control)", outcome)
	}
}

// TestReadvertiseOptedVerdictDecides proves an opted member's withdraw and
// reject decide the readvertise run, as the RFC 9494 egress step's do.
func TestReadvertiseOptedVerdictDecides(t *testing.T) {
	withdraw := func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
		return &rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}, nil
	}
	for _, tc := range []struct {
		name   string
		answer func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error)
		want   OutcomeAction
	}{
		{"withdraw", withdraw, OutcomeWithdraw},
		{"reject", rejectAnswer, OutcomeReject},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caller := &fakeCaller{filters: map[string]fakeFilter{
				"probe:opted": {decl: ExternalDeclaration{Reads: []string{"local-preference"}, OnError: rpc.OnErrorReject, Readvertise: true}, answer: tc.answer},
				"probe:plain": {decl: ExternalDeclaration{Reads: []string{"local-preference"}, OnError: rpc.OnErrorReject}, answer: acceptAnswer},
			}}
			outcome := runReadvertise(t, readvertiseChain(t), caller, localPref(100))
			if outcome.Action != tc.want || outcome.Member != "probe:opted" {
				t.Fatalf("outcome = %+v, want %v from probe:opted", outcome, tc.want)
			}
		})
	}
}

// TestReadvertiseUndeclaredMemberFailsClosed proves a member whose plugin
// declares no such filter runs on the rail and fails the route, rather than
// being passed over: whether it would opt in is unknown, and it may be the
// step that withholds the stale route (RFC 9494 Section 4.3).
func TestReadvertiseUndeclaredMemberFailsClosed(t *testing.T) {
	caller := &fakeCaller{filters: map[string]fakeFilter{
		"probe:plain": {decl: ExternalDeclaration{Reads: []string{"local-preference"}, OnError: rpc.OnErrorAccept}, answer: acceptAnswer},
	}}
	chain := readvertiseChain(t)
	if !chain.Readvertises(caller) {
		t.Fatal("Readvertises = false for a chain holding an undeclared member")
	}
	outcome := runReadvertise(t, chain, caller, localPref(100))
	if outcome.Action != OutcomeReject || !outcome.Failed || outcome.Member != "probe:opted" {
		t.Fatalf("outcome = %+v, want a failed reject from the undeclared probe:opted", outcome)
	}
}

// TestReadvertisesNoOptedMember proves a chain whose plugin filters all
// declare and none opts in does not take the readvertise rail.
func TestReadvertisesNoOptedMember(t *testing.T) {
	caller := &fakeCaller{filters: map[string]fakeFilter{
		"probe:opted": {decl: ExternalDeclaration{Reads: []string{"local-preference"}}, answer: acceptAnswer},
		"probe:plain": {decl: ExternalDeclaration{Reads: []string{"local-preference"}}, answer: acceptAnswer},
	}}
	if readvertiseChain(t).Readvertises(caller) {
		t.Fatal("Readvertises = true for a chain with no opted member")
	}
}
