package bgpconfig

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
)

// viewTestType stands for a filter type whose plugin answers over the route
// view: it reads and sets local-preference.
const viewTestType = "bgpconfig-view-test"

func init() {
	policy.MustRegisterFilterType(viewTestType,
		[]policy.Field{policy.FieldLocalPreference}, []policy.Field{policy.FieldLocalPreference})
}

// TestViewFilterChainCompilesToLibrary proves that a chain of plugin filters
// alone compiles to the library chain (D-12), and the library chain calls the
// plugin by its plugin and filter name with the fields it declared.
func TestViewFilterChainCompilesToLibrary(t *testing.T) {
	reg, err := policy.NewRegistry(policy.Instance{Name: "pref", Type: viewTestType})
	if err != nil {
		t.Fatal(err)
	}
	p, err := filterapi.NewPolicy([]policy.Ref{{Name: "plug:pref"}}, policy.FinalAccept)
	if err != nil {
		t.Fatal(err)
	}
	got, err := compileLibraryChain(p, reg, policychain.Bases{}, "bgp import", "peer p import filters")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got.ActiveExternal(0) {
		t.Fatal("a view filter is still reached over the text chain")
	}
	chain, ok := got.Library().(*policychain.Chain)
	if !ok {
		t.Fatalf("a chain of one view filter carries library %T, want *policychain.Chain", got.Library())
	}

	caller := &recordingCaller{decl: policychain.ExternalDeclaration{Reads: []string{"local-preference"}}}
	view := new(policyview.View)
	view.Reset(updateWithLocalPref(t), policyview.Session{})
	defer view.Clear()
	sets := new(policychain.RunSets)
	sets.SetRoute(policychain.ExternalRoute{Caller: caller, Direction: "import"})
	if outcome := chain.Run(view, sets); outcome.Action != policychain.OutcomeAccept || outcome.Failed {
		t.Fatalf("outcome = %+v, want accept", outcome)
	}
	if len(caller.calls) != 1 || caller.calls[0] != `plug:pref {"local-preference":100}` {
		t.Fatalf("plugin filter calls = %v, want one call of plug:pref sent local-preference 100", caller.calls)
	}
}
