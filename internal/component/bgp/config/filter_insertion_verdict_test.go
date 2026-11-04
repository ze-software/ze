package bgpconfig

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// verdictConfig puts peer A in group G, which meets the insertion condition
// (insertionTestYANG's mode leaf), with lists the operator configured but
// named no member in, and peer B outside G naming the same filter in both
// lists itself.
const verdictConfig = `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	group G {
		session { asn { remote 65000; } }
		insertion-test { mode on; }
		peer A {
			connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
			filter { import { final accept; } export { final accept; } }
		}
	}
	peer B {
		connection { remote { ip 192.0.2.2; } local { ip 192.0.2.254; } }
		session { asn { remote 65002; } }
		filter {
			import { filters [ fake-inserter:early ]; final accept; }
			export { filters [ fake-inserter:early ]; final accept; }
		}
	}
}
`

// verdictCaller stands for the plugin server: it declares the one filter
// reading and setting local-preference with onError, and answers every call
// with answer.
type verdictCaller struct {
	onError rpc.OnErrorPolicy
	answer  func() (*rpc.FilterUpdateOutput, error)
}

func (c *verdictCaller) FilterDeclaration(string, string) (policychain.ExternalDeclaration, bool) {
	return policychain.ExternalDeclaration{Reads: []string{"local-preference"}, Sets: []string{"local-preference"},
		OnError: c.onError}, true
}

func (c *verdictCaller) CallFilter(string, *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	return c.answer()
}

// verdictRun is what one run of a compiled list answers: the outcome and the
// sets the chain folded for the route.
type verdictRun struct {
	outcome policychain.Outcome
	folded  policychain.Sets
}

// TestInsertedMemberVerdicts checks spec-filter-wire-6-stage-filters AC-4
// for an inserted member: every verdict an auto-inserted member answers
// (accept, reject, modify with sets, withdraw) and every failure (the plugin
// unreachable, a malformed answer, under on-error reject and accept) applies
// exactly as the same filter's verdict does when the operator names it, on
// import and on export. Method: one filter, inserted through the loader in
// peer A's lists and named by the operator in peer B's; each verdict is run
// through both peers' compiled lists and the two answers must be equal and
// must be the answer the verdict owes.
func TestInsertedMemberVerdicts(t *testing.T) {
	registerInserter(t,
		rpc.FilterInsertion{Filter: "fake-inserter:early", Direction: rpc.FilterImport, Priority: 100,
			Config: rrClient, Values: []string{"on"}},
		rpc.FilterInsertion{Filter: "fake-inserter:early", Direction: rpc.FilterExport, Priority: 100,
			Config: rrClient, Values: []string{"on"}})
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(verdictConfig)
	require.NoError(t, err)
	peers, _, err := peersAndDynamicGroups(tree)
	require.NoError(t, err)
	byName := make(map[string]filterapi.Policy, 2*len(peers))
	for _, ps := range peers {
		byName[ps.Name+" import"] = ps.ImportPolicy
		byName[ps.Name+" export"] = ps.ExportPolicy
	}
	require.Equal(t, []string{"fake-inserter:early"}, refNames(byName["A import"].Filters()),
		"peer A's import list holds the member only by insertion")
	require.Equal(t, []string{"fake-inserter:early"}, refNames(byName["A export"].Filters()),
		"peer A's export list holds the member only by insertion")

	answer := func(out rpc.FilterUpdateOutput) func() (*rpc.FilterUpdateOutput, error) {
		return func() (*rpc.FilterUpdateOutput, error) { return &out, nil }
	}
	unreachable := func() (*rpc.FilterUpdateOutput, error) { return nil, errors.New("plugin unavailable") }
	lp200 := rpc.FilterSet{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("200")}
	undeclared := rpc.FilterSet{Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage("5")}
	cases := []struct {
		name    string
		onError rpc.OnErrorPolicy
		answer  func() (*rpc.FilterUpdateOutput, error)
		action  policychain.OutcomeAction
		failed  bool
		sets    bool // the folded sets carry the member's set
	}{
		{"accept", rpc.OnErrorReject, answer(rpc.FilterUpdateOutput{Action: rpc.FilterAccept}), policychain.OutcomeAccept, false, false},
		{"reject", rpc.OnErrorReject, answer(rpc.FilterUpdateOutput{Action: rpc.FilterReject}), policychain.OutcomeReject, false, false},
		{"modify", rpc.OnErrorReject, answer(rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{lp200}}),
			policychain.OutcomeAccept, false, true},
		{"withdraw", rpc.OnErrorReject, answer(rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}), policychain.OutcomeWithdraw, false, false},
		{"unreachable on-error reject", rpc.OnErrorReject, unreachable, policychain.OutcomeReject, true, false},
		{"unreachable on-error accept", rpc.OnErrorAccept, unreachable, policychain.OutcomeAccept, false, false},
		{"malformed on-error reject", rpc.OnErrorReject,
			answer(rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{undeclared}}), policychain.OutcomeReject, true, false},
		{"malformed on-error accept", rpc.OnErrorAccept,
			answer(rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{undeclared}}), policychain.OutcomeAccept, false, false},
	}
	for _, direction := range []string{"import", "export"} {
		for _, tc := range cases {
			t.Run(direction+" "+tc.name, func(t *testing.T) {
				caller := &verdictCaller{onError: tc.onError, answer: tc.answer}
				inserted := runVerdict(t, byName["A "+direction], caller, direction)
				operator := runVerdict(t, byName["B "+direction], caller, direction)
				if !reflect.DeepEqual(inserted, operator) {
					t.Fatalf("inserted member answered %+v, the operator's member %+v: an inserted verdict must apply as an operator's",
						inserted.outcome, operator.outcome)
				}
				// A run the member passes ends at the list's final accept, which
				// names no member; a decision or a failure names the member.
				member := "fake-inserter:early"
				if tc.action == policychain.OutcomeAccept {
					member = ""
				}
				got := inserted.outcome
				if got.Action != tc.action || got.Failed != tc.failed || got.Member != member {
					t.Fatalf("outcome = %+v, want action %v failed %v member %q", got, tc.action, tc.failed, member)
				}
				if inserted.folded.Empty() == tc.sets {
					t.Fatalf("folded sets empty = %v, want the member's set kept = %v", inserted.folded.Empty(), tc.sets)
				}
			})
		}
	}
}

// runVerdict runs p's compiled library chain over one route carrying
// LOCAL_PREF 100, the external member reached through caller.
func runVerdict(t *testing.T, p filterapi.Policy, caller policychain.ExternalCaller, direction string) verdictRun {
	t.Helper()
	chain, ok := p.Library().(*policychain.Chain)
	if !ok {
		t.Fatalf("the %s list carries library %T, want *policychain.Chain", direction, p.Library())
	}
	view := new(policyview.View)
	view.Reset(updateWithLocalPref(t), policyview.Session{})
	defer view.Clear()
	sets := new(policychain.RunSets)
	sets.SetRoute(policychain.ExternalRoute{Caller: caller, Direction: direction, Peer: "192.0.2.9", PeerAS: 65009})
	outcome := chain.Run(view, sets)
	return verdictRun{outcome: outcome, folded: *sets.Folded()}
}
