package bgpconfig

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// libraryTestType stands for a filter type with an in-process BGP evaluator:
// it sets local-preference, which both BGP attachments carry and allow.
const libraryTestType = "bgpconfig-library-test"

func init() {
	policy.MustRegisterFilterType(libraryTestType, nil, []policy.Field{policy.FieldLocalPreference})
	policychain.MustRegisterEvaluator(libraryTestType, rpc.OnErrorReject, func(string, map[string]any) (policychain.Evaluator, error) {
		return func(_ *policyview.View, sets *policychain.Sets) (policychain.Decision, error) {
			return policychain.DecisionPass, sets.LocalPreference(policychain.NumericActionSet, 200)
		}, nil
	})
}

// TestLibraryChainAtPeerLoad proves the config half of the 3a wiring: a BGP
// chain may name a type with an in-process evaluator (bgpServesFilterType),
// its policy then carries the compiled library chain, a chain naming no such
// type carries none, and a chain mixing a library member with a plugin filter
// is refused naming both filters and what to do.
func TestLibraryChainAtPeerLoad(t *testing.T) {
	reg, err := policy.NewRegistry(policy.Instance{Name: "lp", Type: libraryTestType})
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := reg.Lookup("lp")
	if err := bgpServesFilterType(policy.Ref{Name: "lp"}, entry, map[string]string{}, "bgp import", "peer p import filters"); err != nil {
		t.Fatalf("a type with an in-process evaluator was refused: %v", err)
	}

	library, err := filterapi.NewPolicy([]policy.Ref{{Name: "lp"}}, policy.FinalAccept)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compileLibraryChain(library, reg, policychain.Bases{}, "bgp import", "peer p import filters")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if compiled.Library() == nil {
		t.Fatal("a chain of one library member carries no library chain")
	}
	if compiled.ActiveExternal(0) {
		t.Fatal("a library member is reached over filter-update")
	}

	plain, err := filterapi.NewPolicy([]policy.Ref{{Name: "ext:other"}}, policy.FinalAccept)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := compileLibraryChain(plain, reg, policychain.Bases{}, "bgp import", "ctx"); err != nil || got.Library() != nil {
		t.Fatalf("a plugin-only chain = library %v, err %v; want none and no error", got.Library(), err)
	}

	mixed, err := filterapi.NewPolicy([]policy.Ref{{Name: "lp"}, {Name: "ext:other"}}, policy.FinalAccept)
	if err != nil {
		t.Fatal(err)
	}
	_, err = compileLibraryChain(mixed, reg, policychain.Bases{}, "bgp import", "peer p import filters")
	if err == nil {
		t.Fatal("a chain mixing a library member and a plugin filter was accepted")
	}
	for _, want := range []string{"peer p import filters", `"lp"`, `"ext:other"`, "chain of its own"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}
}

// TestParseAttributeBases proves the library chain's arithmetic bases come
// from bgp defaults attribute, each absent leaf standing at its YANG default.
func TestParseAttributeBases(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	tree := config.NewTree()
	bases, err := parseAttributeBases(tree, schema)
	if err != nil {
		t.Fatal(err)
	}
	if bases != (policychain.Bases{LocalPreference: 100, MED: 0}) {
		t.Fatalf("defaults = %+v, want local-preference 100, med 0", bases)
	}
	tree.GetOrCreateContainer("bgp").GetOrCreateContainer("defaults").GetOrCreateContainer("attribute").Set("med", "40")
	bases, err = parseAttributeBases(tree, schema)
	if err != nil {
		t.Fatal(err)
	}
	if bases != (policychain.Bases{LocalPreference: 100, MED: 40}) {
		t.Fatalf("configured med 40 = %+v, want local-preference 100, med 40", bases)
	}
}
