package bgpconfig

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
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
// compiles into one library chain that holds both, in chain order (FR-1,
// D-12): run, it calls the plugin filter by its plugin and filter name after
// the library member, which it reads the set of.
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
	got, err := compileLibraryChain(mixed, reg, policychain.Bases{}, "bgp import", "peer p import filters")
	if err != nil {
		t.Fatalf("a chain mixing a library member and a plugin filter was refused: %v", err)
	}
	if got.ActiveExternal(1) {
		t.Fatal("the plugin filter of a mixed chain is still reached over the text chain")
	}
	chain, ok := got.Library().(*policychain.Chain)
	if !ok {
		t.Fatalf("a mixed chain carries library %T, want *policychain.Chain", got.Library())
	}

	// Run the compiled chain: the plugin member must be called as filter
	// "other" of plugin "ext", after lp, so it reads lp's set (200, not the
	// route's 100). A dropped plugin member, a reversed order or a swapped
	// plugin/filter split each fail one assertion below.
	caller := &recordingCaller{decl: policychain.ExternalDeclaration{
		Reads: []string{"local-preference"}, OnError: rpc.OnErrorAccept,
	}}
	view := new(policyview.View)
	view.Reset(updateWithLocalPref(t, 100), policyview.Session{})
	defer view.Clear()
	sets := new(policychain.RunSets)
	sets.SetRoute(policychain.ExternalRoute{Caller: caller, Direction: "import"})
	outcome := chain.Run(view, sets)
	if outcome.Action != policychain.OutcomeAccept || outcome.Failed {
		t.Fatalf("mixed chain outcome = %+v, want accept", outcome)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("plugin filter calls = %v, want exactly one call of ext:other", caller.calls)
	}
	if caller.calls[0] != `ext:other {"local-preference":200}` {
		t.Fatalf("plugin filter call = %s, want ext:other sent the local-preference 200 lp set", caller.calls[0])
	}
}

// recordingCaller declares every filter of every plugin with decl, accepts
// each route, and records each call as "<plugin>:<filter> <fields>".
type recordingCaller struct {
	decl  policychain.ExternalDeclaration
	calls []string
}

func (c *recordingCaller) FilterDeclaration(string, string) (policychain.ExternalDeclaration, bool) {
	return c.decl, true
}

func (c *recordingCaller) CallFilter(plugin string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	inputValue, err := route.Input()
	if err != nil {
		return nil, err
	}
	input := &inputValue
	c.calls = append(c.calls, plugin+":"+input.Filter+" "+string(input.Fields))
	return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
}

// updateWithLocalPref builds an UPDATE for 203.0.113.0/24 carrying ORIGIN
// IGP and LOCAL_PREF lp.
func updateWithLocalPref(t *testing.T, lp uint32) *wireu.WireUpdate {
	t.Helper()
	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x05, 0x04}
	attrs = binary.BigEndian.AppendUint32(attrs, lp)
	payload := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(attrs)))
	payload = append(payload, attrs...)
	payload = append(payload, 24, 203, 0, 113)
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return wireu.NewWireUpdate(payload, id)
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
