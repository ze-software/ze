// Design: docs/architecture/core-design.md -- Policy Filter Chain
// Related: library_chains_test.go -- recordingCaller, updateWithLocalPref

package bgpconfig

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestLibraryChainSkipsInactiveMembers is the library form of the text
// chain's inactive-member test: a deactivated filter reference compiles into
// no member, so the run never calls it, the active member after it is still
// called, and a chain whose every reference is inactive carries no library
// chain at all (the session's final decides it).
func TestLibraryChainSkipsInactiveMembers(t *testing.T) {
	reg, err := policy.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	mixed, err := filterapi.NewPolicy([]policy.Ref{{Name: "ext:off", Inactive: true}, {Name: "ext:on"}}, policy.FinalAccept)
	if err != nil {
		t.Fatal(err)
	}
	got, err := compileLibraryChain(mixed, reg, policychain.Bases{}, "bgp import", "peer p import filters")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	chain, ok := got.Library().(*policychain.Chain)
	if !ok {
		t.Fatalf("a chain with one active plugin filter carries library %T, want *policychain.Chain", got.Library())
	}
	caller := &recordingCaller{decl: policychain.ExternalDeclaration{
		Reads: []string{"local-preference"}, OnError: rpc.OnErrorReject,
	}}
	view := new(policyview.View)
	view.Reset(updateWithLocalPref(t, 100), policyview.Session{})
	defer view.Clear()
	sets := new(policychain.RunSets)
	sets.SetRoute(policychain.ExternalRoute{Caller: caller, Direction: "import"})
	if outcome := chain.Run(view, sets); outcome.Action != policychain.OutcomeAccept || outcome.Failed {
		t.Fatalf("outcome = %+v, want accept", outcome)
	}
	if len(caller.calls) != 1 || caller.calls[0] != `ext:on {"local-preference":100}` {
		t.Fatalf("plugin filter calls = %q, want only ext:on, the inactive ext:off never called", caller.calls)
	}

	inactive, err := filterapi.NewPolicy([]policy.Ref{{Name: "ext:off", Inactive: true}}, policy.FinalAccept)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := compileLibraryChain(inactive, reg, policychain.Bases{}, "bgp import", "ctx"); err != nil || got.Library() != nil {
		t.Fatalf("an all-inactive chain = library %v, err %v; want none and no error", got.Library(), err)
	}
}
